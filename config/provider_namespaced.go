// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"

	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/crossplane-contrib/provider-upjet-harbor/config/namespaced"
)

// GetProviderNamespaced returns the namespaced provider configuration with
// all upstream Harbor resources included.
func GetProviderNamespaced(_ context.Context, sdkProvider *schema.Provider) (*ujconfig.Provider, error) {
	pc := ujconfig.NewProvider(
		[]byte(providerSchema), resourcePrefix, modulePath, []byte(providerMetadata),
		ujconfig.WithRootGroup("harbor.m.crossplane.io"),
		ujconfig.WithIncludeList([]string{}),
		ujconfig.WithTerraformPluginSDKIncludeList([]string{".*$"}),
		ujconfig.WithFeaturesPackage("internal/features"),
		ujconfig.WithTerraformProvider(sdkProvider),
		ujconfig.WithDefaultResourceOptions(ExternalNameConfigurations()),
		ujconfig.WithBasePackages(harborBasePackages),
		ujconfig.WithExampleManifestConfiguration(ujconfig.ExampleManifestConfiguration{
			ManagedResourceNamespace: "crossplane-system",
		}),
	)

	for _, configure := range namespaced.ProviderConfiguration {
		configure(pc)
	}

	flattenHarborShortGroup(pc)

	pc.ConfigureResources()
	return pc, nil
}
