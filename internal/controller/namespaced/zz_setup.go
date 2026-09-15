// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/upjet/v2/pkg/controller"

	auth "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/config/auth"
	security "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/config/security"
	system "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/config/system"
	collection "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/garbage/collection"
	group "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/harbor/group"
	label "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/harbor/label"
	project "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/harbor/project"
	registry "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/harbor/registry"
	replication "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/harbor/replication"
	tasks "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/harbor/tasks"
	user "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/harbor/user"
	tagrule "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/immutable/tagrule"
	services "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/interrogation/services"
	instance "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/preheat/instance"
	membergroup "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/project/membergroup"
	memberuser "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/project/memberuser"
	webhook "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/project/webhook"
	providerconfig "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/providerconfig"
	auditlog "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/purge/auditlog"
	policy "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/retention/policy"
	account "github.com/crossplane-contrib/provider-upjet-harbor/internal/controller/namespaced/robot/account"
)

// Setup creates all controllers with the supplied logger and adds them to
// the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		auth.Setup,
		security.Setup,
		system.Setup,
		collection.Setup,
		group.Setup,
		label.Setup,
		project.Setup,
		registry.Setup,
		replication.Setup,
		tasks.Setup,
		user.Setup,
		tagrule.Setup,
		services.Setup,
		instance.Setup,
		membergroup.Setup,
		memberuser.Setup,
		webhook.Setup,
		providerconfig.Setup,
		auditlog.Setup,
		policy.Setup,
		account.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}

// SetupGated creates all controllers with the supplied logger and adds them to
// the supplied manager gated.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		auth.SetupGated,
		security.SetupGated,
		system.SetupGated,
		collection.SetupGated,
		group.SetupGated,
		label.SetupGated,
		project.SetupGated,
		registry.SetupGated,
		replication.SetupGated,
		tasks.SetupGated,
		user.SetupGated,
		tagrule.SetupGated,
		services.SetupGated,
		instance.SetupGated,
		membergroup.SetupGated,
		memberuser.SetupGated,
		webhook.SetupGated,
		providerconfig.SetupGated,
		auditlog.SetupGated,
		policy.SetupGated,
		account.SetupGated,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}

// SetupWebhookWithManager registers conversion webhooks for all resource kinds in the group.
func SetupWebhookWithManager(mgr ctrl.Manager) error {
	for _, setup := range []func(ctrl.Manager) error{
		auth.SetupWebhookWithManager,
		security.SetupWebhookWithManager,
		system.SetupWebhookWithManager,
		collection.SetupWebhookWithManager,
		group.SetupWebhookWithManager,
		label.SetupWebhookWithManager,
		project.SetupWebhookWithManager,
		registry.SetupWebhookWithManager,
		replication.SetupWebhookWithManager,
		tasks.SetupWebhookWithManager,
		user.SetupWebhookWithManager,
		tagrule.SetupWebhookWithManager,
		services.SetupWebhookWithManager,
		instance.SetupWebhookWithManager,
		membergroup.SetupWebhookWithManager,
		memberuser.SetupWebhookWithManager,
		webhook.SetupWebhookWithManager,
		providerconfig.SetupWebhookWithManager,
		auditlog.SetupWebhookWithManager,
		policy.SetupWebhookWithManager,
		account.SetupWebhookWithManager,
	} {
		if err := setup(mgr); err != nil {
			return err
		}
	}
	return nil
}
