// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package config

import (
	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
)

// Configure adds Harbor system-config resource configurations
// (config_auth, config_security, config_system).
func Configure(p *ujconfig.Provider) {
	p.AddResourceConfigurator("harbor_config_auth", func(r *ujconfig.Resource) {
		// Write-only secret fields have no Terraform-managed state to read
		// back, so they cannot be represented as a Crossplane resource
		// field; drop them from generation entirely.
		delete(r.TerraformResource.Schema, "oidc_client_secret_wo")
		delete(r.TerraformResource.Schema, "oidc_client_secret_wo_version")
	})
}
