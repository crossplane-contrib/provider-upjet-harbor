// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package interrogation

import (
	ujconfig "github.com/crossplane/upjet/v2/pkg/config"

	"github.com/crossplane-contrib/provider-upjet-harbor/config/converters"
)

// Configure adds Harbor interrogation_services resource configuration.
func Configure(p *ujconfig.Provider) {
	p.AddResourceConfigurator("harbor_interrogation_services", func(r *ujconfig.Resource) {
		// Harbor reads "daily" back as "Daily"; without this the resource
		// is rewritten on every reconcile.
		converters.SuppressCaseInsensitiveDiff(r, "vulnerability_scan_policy")
	})
}
