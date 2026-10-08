// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	cluster "github.com/crossplane-contrib/provider-upjet-harbor/apis/cluster/harbor/v1alpha1"
	namespaced "github.com/crossplane-contrib/provider-upjet-harbor/apis/namespaced/harbor/v1alpha1"
)

type projectWithReferences interface {
	client.Object
	ResolveReferences(context.Context, client.Reader) error
}

func TestProjectRegistryReferences(t *testing.T) {
	const (
		registryName   = "docker-hub"
		registryLabel  = "dockerhub"
		nameKey        = "name"
		registryRefKey = "registryIdRef"
	)

	scopes := map[string]struct {
		namespace   string
		addToScheme func(*runtime.Scheme) error
		project     func() projectWithReferences
		registry    func() client.Object
	}{
		"Cluster": {
			addToScheme: cluster.SchemeBuilder.AddToScheme,
			project:     func() projectWithReferences { return &cluster.Project{} },
			registry:    func() client.Object { return &cluster.Registry{} },
		},
		"Namespaced": {
			namespace:   "cache-system",
			addToScheme: namespaced.SchemeBuilder.AddToScheme,
			project:     func() projectWithReferences { return &namespaced.Project{} },
			registry:    func() client.Object { return &namespaced.Registry{} },
		},
	}

	cases := map[string]struct {
		parameters map[string]any
		registryID string
		missing    bool
		wantID     any
		wantRef    string
		wantError  bool
	}{
		"Reference": {
			parameters: map[string]any{registryRefKey: map[string]any{nameKey: registryName}},
			registryID: "42", wantID: int64(42), wantRef: registryName,
		},
		"Selector": {
			parameters: map[string]any{"registryIdSelector": map[string]any{
				"matchLabels": map[string]any{"registry": registryLabel},
			}},
			registryID: "42", wantID: int64(42), wantRef: registryName,
		},
		"ResolveAlways": {
			parameters: map[string]any{
				"registryId": int64(17),
				registryRefKey: map[string]any{
					nameKey: registryName, "policy": map[string]any{"resolve": "Always"},
				},
			},
			registryID: "42", wantID: int64(42), wantRef: registryName,
		},
		"ExplicitID": {
			parameters: map[string]any{"registryId": int64(17)},
			missing:    true, wantID: int64(17),
		},
		"NoRegistry": {
			parameters: map[string]any{}, missing: true,
		},
		"MissingReference": {
			parameters: map[string]any{registryRefKey: map[string]any{nameKey: registryName}},
			missing:    true, wantError: true,
		},
		"UnmatchedSelector": {
			parameters: map[string]any{"registryIdSelector": map[string]any{
				"matchLabels": map[string]any{"registry": registryLabel},
			}},
			missing: true, wantError: true,
		},
		"RegistryNotObserved": {
			parameters: map[string]any{registryRefKey: map[string]any{nameKey: registryName}},
			wantError:  true,
		},
	}

	for scopeName, scope := range scopes {
		for _, field := range []string{"forProvider", "initProvider"} {
			for name, tc := range cases {
				t.Run(scopeName+"/"+field+"/"+name, func(t *testing.T) {
					scheme := runtime.NewScheme()
					if err := scope.addToScheme(scheme); err != nil {
						t.Fatal(err)
					}
					project := scope.project()
					setReferenceTestObject(t, project, map[string]any{
						"metadata": map[string]any{nameKey: registryLabel, "namespace": scope.namespace},
						"spec":     map[string]any{field: tc.parameters},
					})
					objects := make([]client.Object, 0, 3)
					newRegistry := func(name, namespace, label, id string) client.Object {
						r := scope.registry()
						setReferenceTestObject(t, r, map[string]any{
							"metadata": map[string]any{
								nameKey: name, "namespace": namespace,
								"labels":      map[string]any{"registry": label},
								"annotations": map[string]any{"crossplane.io/external-name": "/registries/" + id},
							},
							"status": map[string]any{"atProvider": map[string]any{"registryId": id}},
						})
						return r
					}
					if !tc.missing {
						objects = append(objects, newRegistry(registryName, scope.namespace, registryLabel, tc.registryID))
					}
					// Neither a different registry nor another namespace may satisfy the reference.
					objects = append(objects, newRegistry("aaa-unrelated", scope.namespace, "quay", "99"))
					if scope.namespace != "" {
						objects = append(objects, newRegistry(registryName, "another-namespace", registryLabel, "99"))
					}
					reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
					err := project.ResolveReferences(t.Context(), reader)
					if tc.wantError {
						if err == nil {
							t.Fatal("ResolveReferences() succeeded without a usable registry ID")
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					result, err := runtime.DefaultUnstructuredConverter.ToUnstructured(project)
					if err != nil {
						t.Fatal(err)
					}
					parameters := result["spec"].(map[string]any)[field].(map[string]any)
					if diff := cmp.Diff(tc.wantID, parameters["registryId"]); diff != "" {
						t.Errorf("registryId (-want, +got):\n%s", diff)
					}
					if tc.wantRef != "" {
						ref := parameters[registryRefKey].(map[string]any)
						if ref[nameKey] != tc.wantRef {
							t.Errorf("registryIdRef.name = %v, want %q", ref[nameKey], tc.wantRef)
						}
					}
				})
			}
		}
	}
}

func setReferenceTestObject(t *testing.T, object client.Object, fields map[string]any) {
	t.Helper()
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(fields, object); err != nil {
		t.Fatal(err)
	}
}
