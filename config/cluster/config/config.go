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
		// _wo fields stay in the runtime schema: upstream Create and
		// Update fails with the following error if they're not present
		// "async create failed: failed to create the resource: [{0 error retrieving
		// write-only argument \"oidc_client_secret_wo\": [{0 Invalid config path The Terraform
		// Provider unexpectedly provided a path that does not match the current schema.
		// This can happen if the path does not correctly follow the schema in structure
		// or types. Please report this to the provider developers. \n\nCannot find config
		// value for given path. [{{} oidc_client_secret_wo}]}]  []}]"
		r.TerraformResource.Schema["oidc_client_secret_wo"].Sensitive = true
		r.TerraformResource.Schema["oidc_client_secret_wo"].RequiredWith = nil
		r.TerraformResource.Schema["oidc_client_secret_wo_version"].RequiredWith = nil
		ujconfig.MoveToStatus(r.TerraformResource, "oidc_client_secret_wo", "oidc_client_secret_wo_version")
	})
}
