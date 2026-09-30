// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package purge

import (
	ujconfig "github.com/crossplane/upjet/v2/pkg/config"

	"github.com/crossplane-contrib/provider-upjet-harbor/config/converters"
)

// Configure adds Harbor purge_audit_log resource configuration.
func Configure(p *ujconfig.Provider) {
	p.AddResourceConfigurator("harbor_purge_audit_log", func(r *ujconfig.Resource) {
		// Harbor reads "daily" back as "Daily"; without this the resource
		// is rewritten on every reconcile.
		converters.SuppressCaseInsensitiveDiff(r, "schedule")
	})
}
