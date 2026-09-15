// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	// Note: imported to embed the provider schema document.
	_ "embed"

	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/crossplane-contrib/provider-upjet-harbor/config/cluster"
)

const (
	resourcePrefix = "harbor"
	modulePath     = "github.com/crossplane-contrib/provider-upjet-harbor"
)

//go:embed schema.json
var providerSchema string

//go:embed provider-metadata.yaml
var providerMetadata string

// harborBasePackages overrides upjet's DefaultBasePackages to drop the
// top-level "v1alpha1" entry. We don't have an apis/{cluster,namespaced}/v1alpha1
// placeholder package (it was removed in the L5 cleanup); leaving it in the
// list causes upjet's pipeline to re-emit a broken import on every
// `make generate` run.
var harborBasePackages = ujconfig.BasePackages{
	APIVersion: []string{
		"v1beta1",
	},
	ControllerMap: map[string]string{
		"providerconfig": ujconfig.PackageNameConfig,
	},
}

// GetProvider returns the cluster-scoped provider configuration. It mirrors
// GetProviderNamespaced: the same upstream Harbor resources are included, but
// the CRDs land under the cluster-scoped root group (harbor.crossplane.io).
func GetProvider(_ context.Context, sdkProvider *schema.Provider) (*ujconfig.Provider, error) {
	pc := ujconfig.NewProvider(
		[]byte(providerSchema), resourcePrefix, modulePath, []byte(providerMetadata),
		ujconfig.WithRootGroup("harbor.crossplane.io"),
		ujconfig.WithIncludeList([]string{}),
		ujconfig.WithTerraformPluginSDKIncludeList([]string{".*$"}),
		ujconfig.WithFeaturesPackage("internal/features"),
		ujconfig.WithTerraformProvider(sdkProvider),
		ujconfig.WithDefaultResourceOptions(ExternalNameConfigurations()),
		ujconfig.WithBasePackages(harborBasePackages),
	)

	for _, configure := range cluster.ProviderConfiguration {
		configure(pc)
	}

	flattenHarborShortGroup(pc)

	pc.ConfigureResources()
	return pc, nil
}

// flattenHarborShortGroup flattens resources whose auto-derived ShortGroup
// duplicates the RootGroup's leading segment (harbor_project, harbor_user,
// harbor_group, etc.) so their CRDs land at the root group rather than the
// duplicate-prefixed harbor.harbor.* group.
func flattenHarborShortGroup(pc *ujconfig.Provider) {
	for _, r := range pc.Resources {
		if r.ShortGroup == "harbor" {
			r.ShortGroup = ""
		}
	}
}
