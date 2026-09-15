package clients

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	harborclient "github.com/goharbor/terraform-provider-harbor/client"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/upjet/v2/pkg/terraform"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	clusterv1beta1 "github.com/crossplane-contrib/provider-upjet-harbor/apis/cluster/v1beta1"
	namespacedv1beta1 "github.com/crossplane-contrib/provider-upjet-harbor/apis/namespaced/v1beta1"
)

const (
	// error messages
	errNoProviderConfig     = "no providerConfigRef provided"
	errGetProviderConfig    = "cannot get referenced ProviderConfig"
	errTrackUsage           = "cannot track ProviderConfig usage"
	errExtractCredentials   = "cannot extract credentials"
	errUnmarshalCredentials = "cannot unmarshal harbor credentials as JSON"
	errNoCredentials        = "no harbor credentials provided (need username+password or bearer_token)"
)

// buildHarborSetup translates a parsed ProviderConfig + raw credentials
// secret payload into a terraform.Setup. Pure function — no Kubernetes,
// no SDK provider — so tests can exercise the credential-selection logic
// directly.
func buildHarborSetup(cfg namespacedv1beta1.ProviderConfigSpec, credsData []byte) (terraform.Setup, error) {
	ps := terraform.Setup{}
	creds := map[string]string{}
	if err := json.Unmarshal(credsData, &creds); err != nil {
		return ps, errors.Wrap(err, errUnmarshalCredentials)
	}

	ps.Configuration = map[string]any{
		"url":      ptr.Deref(cfg.URL, ""),
		"insecure": ptr.Deref(cfg.Insecure, false),
	}
	if cfg.APIVersion != nil {
		ps.Configuration["api_version"] = *cfg.APIVersion
	}
	if robotPrefix := ptr.Deref(cfg.RobotPrefix, ""); robotPrefix != "" {
		ps.Configuration["robot_prefix"] = robotPrefix
	}

	switch {
	case creds["bearer_token"] != "":
		ps.Configuration["bearer_token"] = creds["bearer_token"]
	case creds["username"] != "" && creds["password"] != "":
		ps.Configuration["username"] = creds["username"]
		ps.Configuration["password"] = creds["password"]
	default:
		return ps, errors.New(errNoCredentials)
	}

	// Mirrors providerConfigure in terraform-provider-harbor: the configured
	// *client.Client is what Terraform resource CRUD functions receive as meta.
	baseURL := strings.TrimRight(ptr.Deref(cfg.URL, ""), "/")
	ps.Meta = harborclient.NewClient(
		baseURL+harborAPIPath(ptr.Deref(cfg.APIVersion, 2)),
		creds["username"],
		creds["password"],
		"",
		creds["bearer_token"],
		ptr.Deref(cfg.Insecure, false),
		ptr.Deref(cfg.RobotPrefix, ""),
		nil,
	)
	return ps, nil
}

// harborAPIPath mirrors the path logic in terraform-provider-harbor's
// providerConfigure. Default (api_version=2) maps to /api/v2.0.
func harborAPIPath(apiVersion int) string {
	switch apiVersion {
	case 1:
		return "/api"
	case 2:
		return "/api/v2.0"
	default:
		return ""
	}
}

// TerraformSetupBuilder returns a terraform.SetupFn for the Harbor provider,
// using SDK-mode. The tfProvider argument is the in-process SDK provider
// instance; upjet's controller uses it via WithTerraformProvider (configured
// in config/provider.go), so this function only needs to translate the
// ProviderConfig + secret into the dynamic Configuration map.
func TerraformSetupBuilder(_ *schema.Provider) terraform.SetupFn {
	return func(ctx context.Context, c client.Client, mg resource.Managed) (terraform.Setup, error) {
		ps := terraform.Setup{}

		pcSpec, err := resolveProviderConfig(ctx, c, mg)
		if err != nil {
			return ps, errors.Wrap(err, "cannot resolve provider config")
		}

		data, err := resource.CommonCredentialExtractor(ctx, pcSpec.Credentials.Source, c, pcSpec.Credentials.CommonCredentialSelectors)
		if err != nil {
			return ps, errors.Wrap(err, errExtractCredentials)
		}

		return buildHarborSetup(*pcSpec, data)
	}
}

func toSharedPCSpec(pc *clusterv1beta1.ProviderConfig) (*namespacedv1beta1.ProviderConfigSpec, error) {
	if pc == nil {
		return nil, nil
	}
	data, err := json.Marshal(pc.Spec)
	if err != nil {
		return nil, err
	}

	var mSpec namespacedv1beta1.ProviderConfigSpec
	err = json.Unmarshal(data, &mSpec)
	return &mSpec, err
}

func resolveProviderConfig(ctx context.Context, crClient client.Client, mg resource.Managed) (*namespacedv1beta1.ProviderConfigSpec, error) {
	switch managed := mg.(type) {
	case resource.LegacyManaged: //nolint:staticcheck // still handling cluster-scoped behavior
		return resolveLegacy(ctx, crClient, managed)
	case resource.ModernManaged:
		return resolveModern(ctx, crClient, managed)
	default:
		return nil, errors.New("resource is not a managed resource")
	}
}

func resolveLegacy(ctx context.Context, client client.Client, mg resource.LegacyManaged) (*namespacedv1beta1.ProviderConfigSpec, error) { //nolint:staticcheck // still handling cluster-scoped behavior
	configRef := mg.GetProviderConfigReference()
	if configRef == nil {
		return nil, errors.New(errNoProviderConfig)
	}
	pc := &clusterv1beta1.ProviderConfig{}
	if err := client.Get(ctx, types.NamespacedName{Name: configRef.Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetProviderConfig)
	}

	t := resource.NewLegacyProviderConfigUsageTracker(client, &clusterv1beta1.ProviderConfigUsage{})
	if err := t.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackUsage)
	}

	return toSharedPCSpec(pc)
}

func resolveModern(ctx context.Context, crClient client.Client, mg resource.ModernManaged) (*namespacedv1beta1.ProviderConfigSpec, error) {
	configRef := mg.GetProviderConfigReference()
	if configRef == nil {
		return nil, errors.New(errNoProviderConfig)
	}

	pcRuntimeObj, err := crClient.Scheme().New(namespacedv1beta1.SchemeGroupVersion.WithKind(configRef.Kind))
	if err != nil {
		return nil, errors.Wrap(err, "unknown GVK for ProviderConfig")
	}
	pcObj, ok := pcRuntimeObj.(resource.ProviderConfig)
	if !ok {
		// This indicates a programming error, types are not properly generated
		return nil, errors.New("referenced kind is not a client.Object")
	}

	// Namespace will be ignored if the PC is a cluster-scoped type
	if err := crClient.Get(ctx, types.NamespacedName{Name: configRef.Name, Namespace: mg.GetNamespace()}, pcObj); err != nil {
		return nil, errors.Wrap(err, errGetProviderConfig)
	}

	var pcSpec namespacedv1beta1.ProviderConfigSpec
	pcu := &namespacedv1beta1.ProviderConfigUsage{}
	switch pc := pcObj.(type) {
	case *namespacedv1beta1.ProviderConfig:
		pcSpec = pc.Spec
		if pcSpec.Credentials.SecretRef != nil {
			pcSpec.Credentials.SecretRef.Namespace = mg.GetNamespace()
		}
	case *namespacedv1beta1.ClusterProviderConfig:
		pcSpec = pc.Spec
	default:
		return nil, errors.New("unknown provider config type")
	}
	t := resource.NewProviderConfigUsageTracker(crClient, pcu)
	if err := t.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackUsage)
	}
	return &pcSpec, nil
}
