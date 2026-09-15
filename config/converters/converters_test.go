// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package converters

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// TestWrapReadDeriveStringID_SharedResourceDualScope reproduces the Registry
// panic seen after the v3.12.0 update. cmd/provider/main.go builds both the
// cluster and namespaced providers from a single sdkProvider, so the
// harbor_registry resource configurator runs twice against the same shared
// *schema.Resource. The wrapper must therefore be installed idempotently;
// otherwise the second pass captures an already-nil legacy Read and the
// installed ReadContext dereferences a nil func on Observe.
func TestWrapReadDeriveStringID_SharedResourceDualScope(t *testing.T) {
	var reads int
	res := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"registry_id": {Type: schema.TypeString, Optional: true},
		},
		Read: func(_ *schema.ResourceData, _ any) error { //nolint: staticcheck
			reads++
			return nil
		},
	}

	// Two provider scopes sharing one *schema.Resource.
	WrapReadDeriveStringID(res, "registry_id")
	WrapReadDeriveStringID(res, "registry_id")

	d := res.TestResourceData()
	d.SetId("/registries/42")

	diags := res.ReadContext(context.Background(), d, nil)
	if diags.HasError() {
		t.Fatalf("ReadContext returned errors: %v", diags)
	}
	if reads != 1 {
		t.Fatalf("original Read called %d times, want 1", reads)
	}
	if got, want := d.Get("registry_id"), "42"; got != want {
		t.Fatalf("registry_id = %q, want %q", got, want)
	}
}
