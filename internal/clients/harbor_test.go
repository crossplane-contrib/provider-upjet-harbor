// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"context"
	"testing"

	rtfake "github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	namespacedv1beta1 "github.com/crossplane-contrib/provider-upjet-harbor/apis/namespaced/v1beta1"
)

var (
	testHarborURL = "https://harbor.example.com"
)

func TestBuildHarborSetup(t *testing.T) {
	tests := []struct {
		name           string
		pcSpec         namespacedv1beta1.ProviderConfigSpec
		credsJSON      string
		wantErr        bool
		wantContains   map[string]any
		mustNotContain []string
	}{
		{
			name:      "basic auth",
			pcSpec:    namespacedv1beta1.ProviderConfigSpec{URL: ptr.To(testHarborURL), Insecure: ptr.To(false)},
			credsJSON: `{"username":"admin","password":"Harbor12345"}`,
			wantContains: map[string]any{
				"url":      testHarborURL,
				"username": "admin",
				"password": "Harbor12345",
				"insecure": false,
			},
			mustNotContain: []string{"bearer_token"},
		},
		{
			name:      "bearer token",
			pcSpec:    namespacedv1beta1.ProviderConfigSpec{URL: ptr.To(testHarborURL)},
			credsJSON: `{"bearer_token":"eyJhbGciOi..."}`,
			wantContains: map[string]any{
				"url":          testHarborURL,
				"bearer_token": "eyJhbGciOi...",
			},
			mustNotContain: []string{"username", "password"},
		},
		{
			name:      "both set, bearer wins",
			pcSpec:    namespacedv1beta1.ProviderConfigSpec{URL: ptr.To(testHarborURL)},
			credsJSON: `{"username":"admin","password":"x","bearer_token":"tok"}`,
			wantContains: map[string]any{
				"url":          testHarborURL,
				"bearer_token": "tok",
			},
			mustNotContain: []string{"username", "password"},
		},
		{
			name:      "no credentials -> error",
			pcSpec:    namespacedv1beta1.ProviderConfigSpec{URL: ptr.To(testHarborURL)},
			credsJSON: `{}`,
			wantErr:   true,
		},
		{
			name:      "malformed JSON -> error",
			pcSpec:    namespacedv1beta1.ProviderConfigSpec{URL: ptr.To(testHarborURL)},
			credsJSON: `{not json`,
			wantErr:   true,
		},
		{
			name: "api_version and robot_prefix propagated",
			pcSpec: namespacedv1beta1.ProviderConfigSpec{
				URL:         ptr.To(testHarborURL),
				APIVersion:  ptr.To(2),
				RobotPrefix: ptr.To("robot$"),
			},
			credsJSON: `{"username":"u","password":"p"}`,
			wantContains: map[string]any{
				"api_version":  2,
				"robot_prefix": "robot$",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildHarborSetup(tc.pcSpec, []byte(tc.credsJSON))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for k, want := range tc.wantContains {
				if got.Configuration[k] != want {
					t.Errorf("Configuration[%q] = %v, want %v", k, got.Configuration[k], want)
				}
			}
			for _, k := range tc.mustNotContain {
				if _, present := got.Configuration[k]; present {
					t.Errorf("Configuration[%q] should not be present", k)
				}
			}
		})
	}
}

// TestResolveModern covers which namespace a modern managed resource's
// ProviderConfig is looked up in.
//
// The ClusterProviderConfig case is the one that matters. It is the default
// when a managed resource sets no providerConfigRef, and it is cluster scoped,
// so it must resolve from a namespaced managed resource in any namespace. A
// real API server's RESTMapper-aware client ignores the namespace given for a
// cluster-scoped kind, which hides a lookup under the managed resource's own
// namespace. The diff server's in-memory client does not: it keys its store on
// an exact (GVK, namespace, name) match and holds a cluster-scoped object under
// the empty namespace, so such a lookup misses. The fake client used here keys
// its tracker the same way, which is what makes this test fail without the fix.
//
// rtfake.ModernManaged stands in for a generated managed resource on purpose:
// this package is compiled into the provider binary, so importing a resource
// API package here would point the dependency the wrong way.
func TestResolveModern(t *testing.T) {
	const (
		mrNamespace   = "team-a"
		clusterURL    = "https://cluster.harbor.example.com"
		namespacedURL = "https://team-a.harbor.example.com"
	)

	scheme := runtime.NewScheme()
	if err := namespacedv1beta1.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatalf("cannot build the test scheme: %v", err)
	}

	type want struct {
		url string
		err bool
	}
	cases := map[string]struct {
		kind string
		want want
	}{
		"ClusterProviderConfigFromNamespacedResource": {
			kind: namespacedv1beta1.ClusterProviderConfigKind,
			want: want{url: clusterURL},
		},
		"ProviderConfigFromOwnNamespace": {
			kind: namespacedv1beta1.ProviderConfigKind,
			want: want{url: namespacedURL},
		},
		"UnknownKind": {
			kind: "NoSuchProviderConfig",
			want: want{err: true},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			kc := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				&namespacedv1beta1.ClusterProviderConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "default"},
					Spec:       namespacedv1beta1.ProviderConfigSpec{URL: new(clusterURL)},
				},
				&namespacedv1beta1.ProviderConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: mrNamespace},
					Spec:       namespacedv1beta1.ProviderConfigSpec{URL: new(namespacedURL)},
				},
			).Build()

			mg := &rtfake.ModernManaged{
				ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: mrNamespace, UID: "test-uid"},
			}
			mg.SetProviderConfigReference(&xpv2.ProviderConfigReference{Kind: tc.kind, Name: "default"})

			got, err := resolveModern(context.Background(), kc, mg)
			if tc.want.err != (err != nil) {
				t.Fatalf("resolveModern(...) error = %v, want error: %t", err, tc.want.err)
			}
			if err != nil {
				return
			}
			if diff := cmp.Diff(tc.want.url, ptr.Deref(got.URL, "")); diff != "" {
				t.Errorf("resolveModern(...) resolved the wrong ProviderConfig, -want +got:\n%s", diff)
			}
		})
	}
}
