// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package retention

import (
	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
)

// Configure adds Harbor retention_policy resource configuration.
func Configure(p *ujconfig.Provider) {
	p.AddResourceConfigurator("harbor_retention_policy", func(r *ujconfig.Resource) {
		r.References["scope"] = ujconfig.Reference{
			TerraformName: "harbor_project",
			Extractor:     `github.com/crossplane/upjet/v2/pkg/resource.ExtractParamPath("id",true)`,
		}
	})
}
