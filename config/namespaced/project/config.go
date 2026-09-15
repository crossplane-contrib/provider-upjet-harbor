// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package project

import (
	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
)

// Configure adds Harbor project-scoped resource configurations
// (project_member_group, project_member_user, project_webhook).
func Configure(p *ujconfig.Provider) {
	p.AddResourceConfigurator("harbor_project_member_group", func(r *ujconfig.Resource) {
		r.References["project_id"] = ujconfig.Reference{
			TerraformName: "harbor_project",
			Extractor:     `github.com/crossplane/upjet/v2/pkg/resource.ExtractParamPath("id",true)`,
		}
		r.References["group_name"] = ujconfig.Reference{
			TerraformName: "harbor_group",
			Extractor:     `github.com/crossplane/upjet/v2/pkg/resource.ExtractParamPath("group_name",true)`,
		}
	})

	p.AddResourceConfigurator("harbor_project_member_user", func(r *ujconfig.Resource) {
		r.References["project_id"] = ujconfig.Reference{
			TerraformName: "harbor_project",
			Extractor:     `github.com/crossplane/upjet/v2/pkg/resource.ExtractParamPath("id",true)`,
		}
		r.References["user_name"] = ujconfig.Reference{
			TerraformName: "harbor_user",
			Extractor:     `github.com/crossplane/upjet/v2/pkg/resource.ExtractParamPath("username",true)`,
		}
	})

	p.AddResourceConfigurator("harbor_project_webhook", func(r *ujconfig.Resource) {
		r.References["project_id"] = ujconfig.Reference{
			TerraformName: "harbor_project",
			Extractor:     `github.com/crossplane/upjet/v2/pkg/resource.ExtractParamPath("id",true)`,
		}
	})
}
