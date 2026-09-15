// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package robot

import (
	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
)

// Configure adds Harbor robot_account resource configuration.
func Configure(p *ujconfig.Provider) {
	p.AddResourceConfigurator("harbor_robot_account", func(r *ujconfig.Resource) {
		r.References["permissions.namespace"] = ujconfig.Reference{
			TerraformName: "harbor_project",
			Extractor:     `github.com/crossplane/upjet/v2/pkg/resource.ExtractParamPath("name",true)`,
		}
	})
}
