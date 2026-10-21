// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"context"
	"net/http"
	"sync"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/upjet/v2/pkg/diffserver"
	"github.com/crossplane/upjet/v2/pkg/terraform"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	namespacedv1beta1 "github.com/crossplane-contrib/provider-upjet-harbor/apis/namespaced/v1beta1"
)

const (
	// Placeholder credentials the offline setup configures the Harbor client
	// with. They are never sent anywhere, because offlineTransport refuses
	// every request before it leaves the process. They exist so that the
	// client is fully built, the way the Terraform provider's resource
	// functions expect to find it in their meta argument, without reading the
	// ProviderConfig's credentials, which are assumed to be unavailable.
	offlineUsername = "offline-diff-server"
	offlinePassword = "offline-diff-server-placeholder" //nolint:gosec // a placeholder, never sent

	// fmtErrOfflineRequestBlocked reports a Harbor API call that the offline
	// diff server refused to send. The query string is deliberately left out
	// of the message: Harbor's list endpoints carry resource names in it.
	fmtErrOfflineRequestBlocked = "the offline diff server blocked an outbound Harbor API request (%s %s): computing this diff requires calling Harbor, which is not possible offline"
)

// EnableOfflineHTTPTransport replaces net/http's default transport with one
// that refuses every request, so that nothing in the process can reach the
// network through it.
//
// This is for the offline diff server, which computes the difference between
// a desired and an actual managed resource state and never reconciles the
// external resource. It must not be called in a provider that reconciles, as
// it disables outbound HTTP for the whole process.
//
// The swap is what guards the Harbor client, and the only thing that can: the
// client sends every request through a fresh http.Client it builds on the
// spot, with no transport of its own, and such a client uses
// http.DefaultTransport. The client's own *http.Client field is unexported
// and unused, so there is no narrower hook. The one exception is a client
// built with insecure set, which builds its own transport to skip TLS
// verification; buildOfflineHarborSetup never does that.
//
// It is safe to call more than once; only the first call takes effect.
func EnableOfflineHTTPTransport() {
	offlineTransportOnce.Do(func() {
		http.DefaultTransport = offlineTransport{}
	})
}

var offlineTransportOnce sync.Once

// OfflineTerraformSetupBuilder returns a Terraform setup that configures the
// Harbor Terraform provider without credentials and without contacting Harbor.
// The returned terraform.Setup carries a fully built *client.Client as its
// Meta, which the Terraform provider's resource functions expect, but one
// that cannot send a request: see EnableOfflineHTTPTransport.
//
// Use it in place of TerraformSetupBuilder in the diff server only. The
// ProviderConfig is still resolved, so that the Harbor URL, API version and
// robot account prefix it carries are honoured the same way they are for a
// live reconcile; only its credentials are ignored, and the Secret they would
// come from is never read.
func OfflineTerraformSetupBuilder(_ *schema.Provider) terraform.SetupFn {
	EnableOfflineHTTPTransport()
	return func(ctx context.Context, c client.Client, mg resource.Managed) (terraform.Setup, error) {
		pcSpec, err := resolveProviderConfig(ctx, c, mg)
		if err != nil {
			return terraform.Setup{}, errors.Wrap(err, errResolveProviderConfig)
		}
		return buildOfflineHarborSetup(*pcSpec), nil
	}
}

// buildOfflineHarborSetup translates a ProviderConfig into a terraform.Setup
// for offline diffs. It is the credential-free counterpart of
// buildHarborSetup: the same configuration, with placeholders where the
// credentials would go.
func buildOfflineHarborSetup(cfg namespacedv1beta1.ProviderConfigSpec) terraform.Setup {
	// insecure only relaxes TLS verification on requests that are never sent.
	// It is pinned to false because true makes the Harbor client build its
	// own transport, bypassing the one EnableOfflineHTTPTransport installed.
	const insecure = false
	ps := terraform.Setup{
		Configuration: harborConfiguration(cfg, insecure),
	}
	ps.Configuration["username"] = offlineUsername
	ps.Configuration["password"] = offlinePassword
	ps.Meta = newHarborClient(cfg, offlineUsername, offlinePassword, "", insecure)
	return ps
}

// offlineTransport refuses every HTTP request it sees, failing the diff at
// once with an error the diff server reports as DIFF_COMPUTATION_NOT_SUPPORTED,
// instead of letting the request reach Harbor or fail slowly against an
// unreachable endpoint. No Harbor resource calls the API from its
// CustomizeDiff today, so this is a safeguard: for the resource functions the
// diff server never calls, and for an upstream change that adds such a call.
type offlineTransport struct{}

func (offlineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, diffserver.NewDiffComputationNotSupportedError(errors.Errorf(fmtErrOfflineRequestBlocked, req.Method, req.URL.Host+req.URL.Path))
}
