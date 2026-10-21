// SPDX-FileCopyrightText: 2026 crossplane-contrib
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	diffv1alpha1 "github.com/crossplane/upjet/v2/proto/diff/v1alpha1"
	harborprovider "github.com/goharbor/terraform-provider-harbor/provider"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane-contrib/provider-upjet-harbor/config"
)

// The manifests below are what a client puts in a plan request. Every
// ProviderConfig references a Secret that does not exist anywhere, which the
// offline diff server must never need.
const (
	clusterProviderConfig = `
apiVersion: harbor.m.crossplane.io/v1beta1
kind: ClusterProviderConfig
metadata:
  name: default
spec:
  url: https://harbor.example.com
  credentials:
    source: Secret
    secretRef:
      name: harbor-creds
      namespace: crossplane-system
      key: credentials
`
	legacyProviderConfig = `
apiVersion: harbor.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  url: https://harbor.example.com
  credentials:
    source: Secret
    secretRef:
      name: harbor-creds
      namespace: crossplane-system
      key: credentials
`
	// A namespaced Project in a namespace of its own, referencing the
	// cluster-scoped ProviderConfig above. Project is a root group kind: its
	// API group is the provider's root group itself, with no short group
	// prefixed to it.
	namespacedProject = `
apiVersion: harbor.m.crossplane.io/v1alpha1
kind: Project
metadata:
  name: main
  namespace: team-a
spec:
  forProvider:
    name: main
    public: true
  providerConfigRef:
    kind: ClusterProviderConfig
    name: default
`
	clusterProjectPublic = `
apiVersion: harbor.crossplane.io/v1alpha1
kind: Project
metadata:
  name: main
spec:
  forProvider:
    name: main
    public: true
  providerConfigRef:
    name: default
`
	clusterProjectRenamed = `
apiVersion: harbor.crossplane.io/v1alpha1
kind: Project
metadata:
  name: main
spec:
  forProvider:
    name: renamed
    public: false
  providerConfigRef:
    name: default
`
	// The actual state of a Project as the provider records it after a
	// reconcile: the external name annotation and the observation the plan is
	// computed against.
	clusterProjectActual = `
apiVersion: harbor.crossplane.io/v1alpha1
kind: Project
metadata:
  name: main
  annotations:
    crossplane.io/external-name: /projects/1
spec:
  forProvider:
    name: main
    public: false
  providerConfigRef:
    name: default
status:
  atProvider:
    id: /projects/1
    name: main
    projectId: 1
    public: false
    vulnerabilityScanning: true
    storageQuota: -1
    enableContentTrust: false
    enableContentTrustCosign: false
    forceDestroy: false
    autoSbomGeneration: false
    proxySpeedKb: -1
    proxyCacheLocalOnNotFound: false
`
	// A robot Account lives in a short group under the root group, the other
	// shape a Harbor kind's API group takes.
	namespacedRobotAccount = `
apiVersion: robot.harbor.m.crossplane.io/v1alpha1
kind: Account
metadata:
  name: ci
  namespace: team-a
spec:
  forProvider:
    name: ci
    level: system
    permissions:
      - kind: system
        namespace: "/"
        access:
          - action: create
            resource: label
  providerConfigRef:
    kind: ClusterProviderConfig
    name: default
`
	missingProviderConfigProject = `
apiVersion: harbor.m.crossplane.io/v1alpha1
kind: Project
metadata:
  name: main
  namespace: team-a
spec:
  forProvider:
    name: main
  providerConfigRef:
    kind: ProviderConfig
    name: missing
`
)

// TestDiffServerPlan drives the diff gRPC server the way a client does, over
// a unix socket, with the provider configurations and the offline Terraform
// setup main wires it with. It covers both shapes a Harbor kind's API group
// takes, a namespaced resource resolving a cluster-scoped ProviderConfig from
// its own namespace, and every plan action.
func TestDiffServerPlan(t *testing.T) {
	ctx := context.Background()
	sdkProvider := harborprovider.Provider()
	clusterProvider, err := config.GetProvider(ctx, sdkProvider)
	if err != nil {
		t.Fatalf("cannot initialize the cluster-scoped provider configuration: %v", err)
	}
	namespacedProvider, err := config.GetProviderNamespaced(ctx, sdkProvider)
	if err != nil {
		t.Fatalf("cannot initialize the namespaced provider configuration: %v", err)
	}
	s, scheme, err := newDiffServer(logging.NewNopLogger(), sdkProvider, clusterProvider, namespacedProvider)
	if err != nil {
		t.Fatalf("cannot initialize the diff gRPC server: %v", err)
	}

	// A unix socket, because Serve does not report the port it bound when
	// given port zero. Its path is kept short: macOS limits socket paths to
	// 104 bytes, which the path t.TempDir returns can exceed.
	dir, err := os.MkdirTemp("", "diff")
	if err != nil {
		t.Fatalf("cannot create the socket directory: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) }) //nolint:errcheck // best effort cleanup of a temporary directory
	sock := filepath.Join(dir, "diff.sock")

	serveCtx, cancel := context.WithCancel(ctx)
	served := make(chan error, 1)
	go func() { served <- s.Serve(serveCtx, "unix", sock, scheme) }()
	t.Cleanup(func() {
		cancel()
		if err := <-served; err != nil {
			t.Errorf("the diff gRPC server failed: %v", err)
		}
	})

	conn, err := grpc.NewClient("unix://"+sock, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("cannot connect to the diff gRPC server: %v", err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // the process is exiting
	client := diffv1alpha1.NewPlanServiceClient(conn)

	type want struct {
		action  diffv1alpha1.Action
		changes map[string]string
		code    codes.Code
		err     string
	}
	cases := map[string]struct {
		desired string
		actual  string
		store   []string
		want    want
	}{
		"NamespacedProjectCreateWithClusterProviderConfig": {
			desired: namespacedProject,
			store:   []string{clusterProviderConfig},
			want: want{
				action: diffv1alpha1.Action_ACTION_CREATE,
				changes: map[string]string{
					"spec.forProvider.name":   "<unset> -> main",
					"spec.forProvider.public": "<unset> -> true",
				},
			},
		},
		"ClusterProjectUpdate": {
			desired: clusterProjectPublic,
			actual:  clusterProjectActual,
			store:   []string{legacyProviderConfig},
			want: want{
				action:  diffv1alpha1.Action_ACTION_UPDATE,
				changes: map[string]string{"spec.forProvider.public": "false -> true"},
			},
		},
		"ClusterProjectReplace": {
			desired: clusterProjectRenamed,
			actual:  clusterProjectActual,
			store:   []string{legacyProviderConfig},
			want: want{
				action:  diffv1alpha1.Action_ACTION_REPLACE,
				changes: map[string]string{"spec.forProvider.name": "main -> renamed (requires replace)"},
			},
		},
		"ClusterProjectNoOp": {
			desired: clusterProjectActual,
			actual:  clusterProjectActual,
			store:   []string{legacyProviderConfig},
			want:    want{action: diffv1alpha1.Action_ACTION_NO_OP},
		},
		"NamespacedRobotAccountCreate": {
			desired: namespacedRobotAccount,
			store:   []string{clusterProviderConfig},
			want: want{
				action: diffv1alpha1.Action_ACTION_CREATE,
				changes: map[string]string{
					"spec.forProvider.level":                             "<unset> -> system",
					"spec.forProvider.name":                              "<unset> -> ci",
					"spec.forProvider.permissions[*].access[*].action":   "<unset> -> create",
					"spec.forProvider.permissions[*].access[*].resource": "<unset> -> label",
					"spec.forProvider.permissions[*].kind":               "<unset> -> system",
					"spec.forProvider.permissions[*].namespace":          "<unset> -> /",
				},
			},
		},
		"MissingProviderConfig": {
			desired: missingProviderConfigProject,
			store:   []string{clusterProviderConfig},
			want:    want{code: codes.Internal, err: "cannot get referenced ProviderConfig"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			req := &diffv1alpha1.PlanRequest{
				DesiredResource: asStruct(t, tc.desired),
				ActualResource:  asStruct(t, tc.actual),
			}
			for _, m := range tc.store {
				req.KubernetesObjectStore = append(req.KubernetesObjectStore, asStruct(t, m))
			}

			callCtx, cancel := context.WithTimeout(ctx, time.Minute)
			defer cancel()
			rsp, err := client.Plan(callCtx, req, grpc.WaitForReady(true))
			if tc.want.err != "" {
				if err == nil {
					t.Fatalf("expected the plan to fail with %q, got response: %s", tc.want.err, protojson.Format(rsp))
				}
				if st := status.Convert(err); st.Code() != tc.want.code || !strings.Contains(st.Message(), tc.want.err) {
					t.Fatalf("expected a %s error containing %q, got %s: %s", tc.want.code, tc.want.err, st.Code(), st.Message())
				}
				return
			}
			if err != nil {
				t.Fatalf("cannot plan: %v", err)
			}
			if diff := cmp.Diff(tc.want.action.String(), rsp.GetAction().String()); diff != "" {
				t.Errorf("unexpected plan action, -want +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.changes, declaredChanges(rsp)); diff != "" {
				t.Errorf("unexpected changes to the declared fields, -want +got:\n%s", diff)
			}
		})
	}
}

// collectionIndex matches the index of a collection element in a field path.
// A set element is indexed by the hash of its content, which no expectation
// should spell out, so indices are rendered as [*] instead.
var collectionIndex = regexp.MustCompile(`\[\d+\]`)

// declaredChanges renders the changes a plan reports for the fields the
// desired resource declares, as field to "actual -> planned". The changes the
// Terraform provider contributes on its own - the defaults it fills in on a
// create - are left out, so that a provider bump changing a default does not
// turn into a failing plan here.
func declaredChanges(rsp *diffv1alpha1.PlanResponse) map[string]string {
	changes := map[string]string{}
	for _, c := range rsp.GetChanges() {
		if c.GetOrigin() != diffv1alpha1.Origin_ORIGIN_DESIRED_STATE {
			continue
		}
		change := renderValue(c.GetActual()) + " -> " + renderValue(c.GetPlanned())
		if c.GetRequiresReplace() {
			change += " (requires replace)"
		}
		changes[collectionIndex.ReplaceAllString(c.GetField(), "[*]")] = change
	}
	if len(changes) == 0 {
		return nil
	}
	return changes
}

func renderValue(v *diffv1alpha1.FieldValue) string {
	switch {
	case v == nil:
		return "<unset>"
	case v.GetValue() != nil:
		return fmt.Sprintf("%v", v.GetValue().AsInterface())
	default:
		return v.GetAbsence().String()
	}
}

// asStruct converts a YAML manifest into the protobuf Struct a plan request
// carries it as. An empty manifest converts to nil, which is how a request
// says that the resource does not exist yet.
func asStruct(t *testing.T, manifest string) *structpb.Struct {
	t.Helper()
	if manifest == "" {
		return nil
	}
	j, err := yaml.ToJSON([]byte(manifest))
	if err != nil {
		t.Fatalf("cannot convert the manifest to JSON: %v", err)
	}
	s := &structpb.Struct{}
	if err := s.UnmarshalJSON(j); err != nil {
		t.Fatalf("cannot convert the manifest to a protobuf Struct: %v", err)
	}
	return s
}
