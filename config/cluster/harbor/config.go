// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package harbor

import (
	ujconfig "github.com/crossplane/upjet/v2/pkg/config"

	"github.com/crossplane-contrib/provider-upjet-harbor/config/converters"
)

// Configure adds Harbor core resource configurations
// (group, label, project, registry, replication, tasks, user).
func Configure(p *ujconfig.Provider) {
	// harbor_registry: override registry_id from int to string so the CRD
	// field is *string and ExtractParamPath returns a usable value.
	p.AddResourceConfigurator("harbor_registry", func(r *ujconfig.Resource) {
		converters.OverrideIntFieldAsString(r, "registry_id")
		r.TerraformConversions = append(r.TerraformConversions, converters.IntFieldAsString("registry_id"))

		// Upstream's Read calls d.Set("registry_id", int). After our schema
		// override the field is TypeString, and terraform-plugin-sdk's
		// setPrimitive uses strict mapstructure.Decode for TypeString — so
		// int→string fails and the Set is silently dropped (upstream swallows
		// the error). Re-derive the value from d.Id() ("/registries/<id>")
		// so it lands in state and propagates to status.atProvider.registryId.
		converters.WrapReadDeriveStringID(r.TerraformResource, "registry_id")
	})

	// harbor_replication: leave the registry_id schema untouched (TypeInt). The
	// upstream Create/Update code asserts d.Get("registry_id").(int), and upjet's
	// HCL pre-processor would panic if the schema were flipped to TypeString
	// while our ToTerraform conversion produced an int64. The reference still
	// uses ExtractParamPath because the referenced Registry's status carries
	// registryId as a *string (its schema *is* overridden). The generated
	// resolver is hand-patched to use FromIntPtrValue/ToIntPtrValue since
	// angryjet only emits string/float pointer helpers for *int64 fields.
	p.AddResourceConfigurator("harbor_replication", func(r *ujconfig.Resource) {
		r.References["registry_id"] = ujconfig.Reference{
			TerraformName: "harbor_registry",
			Extractor:     `github.com/crossplane/upjet/v2/pkg/resource.ExtractParamPath("registry_id",true)`,
		}
	})
}
