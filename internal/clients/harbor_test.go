// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"testing"

	"k8s.io/utils/ptr"

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
