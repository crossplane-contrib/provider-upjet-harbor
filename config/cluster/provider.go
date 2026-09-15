// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"github.com/crossplane-contrib/provider-upjet-harbor/config/cluster/config"
	"github.com/crossplane-contrib/provider-upjet-harbor/config/cluster/garbage"
	"github.com/crossplane-contrib/provider-upjet-harbor/config/cluster/harbor"
	"github.com/crossplane-contrib/provider-upjet-harbor/config/cluster/immutable"
	"github.com/crossplane-contrib/provider-upjet-harbor/config/cluster/interrogation"
	"github.com/crossplane-contrib/provider-upjet-harbor/config/cluster/preheat"
	"github.com/crossplane-contrib/provider-upjet-harbor/config/cluster/project"
	"github.com/crossplane-contrib/provider-upjet-harbor/config/cluster/purge"
	"github.com/crossplane-contrib/provider-upjet-harbor/config/cluster/retention"
	"github.com/crossplane-contrib/provider-upjet-harbor/config/cluster/robot"
)

func init() {
	ProviderConfiguration.AddConfig(config.Configure)
	ProviderConfiguration.AddConfig(garbage.Configure)
	ProviderConfiguration.AddConfig(harbor.Configure)
	ProviderConfiguration.AddConfig(immutable.Configure)
	ProviderConfiguration.AddConfig(interrogation.Configure)
	ProviderConfiguration.AddConfig(preheat.Configure)
	ProviderConfiguration.AddConfig(project.Configure)
	ProviderConfiguration.AddConfig(purge.Configure)
	ProviderConfiguration.AddConfig(retention.Configure)
	ProviderConfiguration.AddConfig(robot.Configure)
}
