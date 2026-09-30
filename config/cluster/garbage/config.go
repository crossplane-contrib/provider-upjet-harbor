// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package garbage

import (
	ujconfig "github.com/crossplane/upjet/v2/pkg/config"

	"github.com/crossplane-contrib/provider-upjet-harbor/config/converters"
)

// Configure adds Harbor garbage_collection resource configuration.
func Configure(p *ujconfig.Provider) {
	p.AddResourceConfigurator("harbor_garbage_collection", func(r *ujconfig.Resource) {
		// Harbor reads "weekly" back as "Weekly"; without this the resource
		// is rewritten on every reconcile.
		converters.SuppressCaseInsensitiveDiff(r, "schedule")
	})
}
