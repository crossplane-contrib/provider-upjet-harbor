// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"context"
	"strings"
	"testing"

	xpresource "github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	rtfake "github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/crossplane/upjet/v2/pkg/terraform"
	harborclient "github.com/goharbor/terraform-provider-harbor/client"
	harborprovider "github.com/goharbor/terraform-provider-harbor/provider"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	tfsdk "github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1beta1 "github.com/crossplane-contrib/provider-upjet-harbor/apis/cluster/v1beta1"
	namespacedv1beta1 "github.com/crossplane-contrib/provider-upjet-harbor/apis/namespaced/v1beta1"
)

const (
	offlineTestURL         = "https://harbor.example.com"
	offlineTestRobotPrefix = "robot@"
	offlineTestNamespace   = "team-a"
	offlineTestPCName      = "default"

	// valFalse is how Terraform's flatmap state spells a false boolean.
	valFalse = "false"
)

// offlineTestSpec returns a ProviderConfig spec whose every field is set, so
// that a test can tell which ones the offline setup honours. Its credentials
// reference a Secret that no test creates: a setup that resolves this spec
// proves it never reads them, as the online setup fails on the missing Secret.
func offlineTestSpec(url string) namespacedv1beta1.ProviderConfigSpec {
	return namespacedv1beta1.ProviderConfigSpec{
		URL:         &url,
		Insecure:    new(true),
		APIVersion:  new(2),
		RobotPrefix: new(offlineTestRobotPrefix),
		Credentials: namespacedv1beta1.ProviderCredentials{
			Source: xpv2.CredentialsSourceSecret,
			CommonCredentialSelectors: xpv2.CommonCredentialSelectors{
				SecretRef: &xpv2.SecretKeySelector{
					SecretReference: xpv2.SecretReference{Name: "harbor-creds", Namespace: "crossplane-system"},
					Key:             "credentials",
				},
			},
		},
	}
}

// TestOfflineTerraformSetupBuilder asserts that the offline setup resolves the
// ProviderConfig a managed resource references, honours everything in it
// except the credentials, and never reads those.
func TestOfflineTerraformSetupBuilder(t *testing.T) {
	const (
		clusterURL    = "https://cluster.harbor.example.com"
		namespacedURL = "https://team-a.harbor.example.com"
		legacyURL     = "https://legacy.harbor.example.com"
	)
	wantConfiguration := func(url string) terraform.ProviderConfiguration {
		return terraform.ProviderConfiguration{
			"url":          url,
			"insecure":     false,
			"api_version":  2,
			"robot_prefix": offlineTestRobotPrefix,
			"username":     offlineUsername,
			"password":     offlinePassword,
		}
	}
	modern := func(kind, name string) xpresource.Managed {
		mg := &rtfake.ModernManaged{
			ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: offlineTestNamespace, UID: "test-uid"},
		}
		mg.SetProviderConfigReference(&xpv2.ProviderConfigReference{Kind: kind, Name: name})
		return mg
	}
	legacy := func(name string) xpresource.Managed {
		mg := &rtfake.LegacyManaged{
			ObjectMeta: metav1.ObjectMeta{Name: "example", UID: "test-uid"},
		}
		mg.SetProviderConfigReference(&xpv2.Reference{Name: name})
		return mg
	}

	scheme := runtime.NewScheme()
	if err := clusterv1beta1.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatalf("cannot add the cluster-scoped ProviderConfig APIs to the test scheme: %v", err)
	}
	if err := namespacedv1beta1.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatalf("cannot add the namespaced ProviderConfig APIs to the test scheme: %v", err)
	}
	kc := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&clusterv1beta1.ProviderConfig{
			ObjectMeta: metav1.ObjectMeta{Name: offlineTestPCName},
			Spec: clusterv1beta1.ProviderConfigSpec{
				URL:         legacyURL,
				Insecure:    true,
				APIVersion:  new(2),
				RobotPrefix: offlineTestRobotPrefix,
				Credentials: clusterv1beta1.ProviderCredentials{Source: xpv2.CredentialsSourceSecret},
			},
		},
		&namespacedv1beta1.ClusterProviderConfig{
			ObjectMeta: metav1.ObjectMeta{Name: offlineTestPCName},
			Spec:       offlineTestSpec(clusterURL),
		},
		&namespacedv1beta1.ProviderConfig{
			ObjectMeta: metav1.ObjectMeta{Name: offlineTestPCName, Namespace: offlineTestNamespace},
			Spec:       offlineTestSpec(namespacedURL),
		},
	).Build()

	// The positive control for "never reads the credentials": the online setup
	// resolves the same ProviderConfig and fails on its missing Secret.
	if _, err := TerraformSetupBuilder(nil)(context.Background(), kc, modern(namespacedv1beta1.ClusterProviderConfigKind, offlineTestPCName)); err == nil {
		t.Fatal("expected the online setup to fail on the missing credentials Secret")
	}

	type want struct {
		configuration terraform.ProviderConfiguration
		err           bool
	}
	cases := map[string]struct {
		mg   xpresource.Managed
		want want
	}{
		"ModernWithClusterProviderConfig": {
			mg:   modern(namespacedv1beta1.ClusterProviderConfigKind, offlineTestPCName),
			want: want{configuration: wantConfiguration(clusterURL)},
		},
		"ModernWithNamespacedProviderConfig": {
			mg:   modern(namespacedv1beta1.ProviderConfigKind, offlineTestPCName),
			want: want{configuration: wantConfiguration(namespacedURL)},
		},
		"Legacy": {
			mg:   legacy(offlineTestPCName),
			want: want{configuration: wantConfiguration(legacyURL)},
		},
		"MissingProviderConfig": {
			mg:   modern(namespacedv1beta1.ProviderConfigKind, "missing"),
			want: want{err: true},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ps, err := OfflineTerraformSetupBuilder(nil)(context.Background(), kc, tc.mg)
			if tc.want.err != (err != nil) {
				t.Fatalf("OfflineTerraformSetupBuilder(...)(...) error = %v, want error: %t", err, tc.want.err)
			}
			if err != nil {
				return
			}
			if diff := cmp.Diff(tc.want.configuration, ps.Configuration); diff != "" {
				t.Errorf("unexpected Terraform provider configuration, -want +got:\n%s", diff)
			}
			c, ok := ps.Meta.(*harborclient.Client)
			if !ok {
				t.Fatalf("terraform.Setup.Meta is %T, the Harbor provider's resource functions need a *client.Client", ps.Meta)
			}
			if diff := cmp.Diff(offlineTestRobotPrefix, c.GetRobotPrefix()); diff != "" {
				t.Errorf("the Harbor client does not carry the ProviderConfig's robot prefix, -want +got:\n%s", diff)
			}
		})
	}
}

// TestOfflineDiff asserts that an offline setup's Meta is what the Terraform
// plugin SDK needs to compute a diff, by making the call the upjet Terraform
// plugin SDK external client makes on the diff server's behalf. The purge
// audit log resource is included because its CustomizeDiff function is one of
// the two in the Harbor provider, and both must run offline.
func TestOfflineDiff(t *testing.T) {
	p := harborprovider.Provider()
	ps := buildOfflineHarborSetup(offlineTestSpec(offlineTestURL))

	type want struct {
		changes     map[string][2]string
		errContains string
	}
	cases := map[string]struct {
		resource string
		state    *tfsdk.InstanceState
		config   map[string]any
		want     want
	}{
		"ProjectUpdate": {
			resource: "harbor_project",
			state: &tfsdk.InstanceState{
				ID: "/projects/1",
				Attributes: map[string]string{
					"id":                             "/projects/1",
					"name":                           "main",
					"project_id":                     "1",
					"public":                         valFalse,
					"vulnerability_scanning":         "true",
					"storage_quota":                  "-1",
					"enable_content_trust":           valFalse,
					"enable_content_trust_cosign":    valFalse,
					"force_destroy":                  valFalse,
					"auto_sbom_generation":           valFalse,
					"proxy_speed_kb":                 "-1",
					"proxy_cache_local_on_not_found": valFalse,
				},
			},
			config: map[string]any{"name": "main", "public": true},
			want:   want{changes: map[string][2]string{"public": {valFalse, "true"}}},
		},
		"PurgeAuditLogCustomizeDiffRunsOffline": {
			resource: "harbor_purge_audit_log",
			state:    &tfsdk.InstanceState{},
			config:   map[string]any{"schedule": "Daily", "audit_retention_hour": 24},
			want:     want{errContains: "at least one of 'include_operations'"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := p.ResourcesMap[tc.resource]
			diff, err := schema.InternalMap(r.Schema).Diff(context.Background(), tc.state, &tfsdk.ResourceConfig{Config: tc.config}, r.CustomizeDiff, ps.Meta, false)
			if tc.want.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want.errContains) {
					t.Fatalf("expected the diff to fail with %q, got: %v", tc.want.errContains, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("cannot diff %s offline: %v", tc.resource, err)
			}
			got := map[string][2]string{}
			for k, a := range diff.Attributes {
				if a.Old != a.New {
					got[k] = [2]string{a.Old, a.New}
				}
			}
			if d := cmp.Diff(tc.want.changes, got); d != "" {
				t.Errorf("unexpected diff for %s, -want +got:\n%s", tc.resource, d)
			}
		})
	}
}
