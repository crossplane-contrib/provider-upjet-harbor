//go:build generate
// +build generate

// NOTE: See the below link for details on what is happening here.
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

// Remove existing CRDs
//go:generate rm -rf ../package/crds

// Remove generated files
//go:generate bash -c "find . -iname 'zz_*' ! -iname 'zz_generated.managed*.go' ! -iname 'zz_generated.pc*.go' ! -iname 'zz_generated.pcu*.go' ! -iname 'zz_generated.pculist*.go' -delete"
//go:generate bash -c "find . -type d -empty -delete"
//go:generate bash -c "find ../internal/controller -iname 'zz_*' -delete"
//go:generate bash -c "find ../internal/controller -type d -empty -delete"
//go:generate rm -rf ../examples-generated

// Generate documentation from Terraform docs.
//go:generate go run github.com/crossplane/upjet/v2/cmd/scraper -n ${TERRAFORM_PROVIDER_SOURCE} -r ../.work/${TERRAFORM_PROVIDER_SOURCE}/${TERRAFORM_DOCS_PATH} -o ../config/provider-metadata.yaml

// Run Upjet generator
//go:generate go run ../cmd/generator/main.go ..

// Generate deepcopy methodsets and CRD manifests
//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile=../hack/boilerplate.go.txt paths=./... crd:allowDangerousTypes=true,crdVersions=v1 output:artifacts:config=../package/crds

// Generate crossplane-runtime methodsets (resource.Claim, etc)
//go:generate go run -tags generate github.com/crossplane/crossplane-tools/cmd/angryjet generate-methodsets --header-file=../hack/boilerplate.go.txt ./...

// Patch generated resolvers for *int64 reference fields. angryjet only emits
// reference.{From,To}PtrValue (string) and {From,To}FloatPtrValue (float64);
// it has no IsIntPointer branch, so resolvers for *int64 fields don't compile.
// Project.registryId and Replication.registryId are *int64 because their Terraform
// schemas are TypeInt and the upstream provider expects integer registry IDs.
// Flipping the SDK schema to TypeString causes upjet's HCL pre-processor to
// panic ("interface {} is int64, not string") because TerraformConversions
// produces an int64 value but the schema-driven type assertion expects a string.
// So we keep the *int64 field and rewrite the resolver to use the int helpers
// from crossplane-runtime. The dot is left as a regex wildcard because the
// surrounding text uniquely identifies these four expressions. This must run after
// angryjet regenerates the file, and for both the cluster- and namespaced-scoped
// copies of the harbor group's resolvers.
//go:generate bash -c "sed -i.bak -e 's|reference.FromPtrValue(mg.Spec.ForProvider.RegistryID)|reference.FromIntPtrValue(mg.Spec.ForProvider.RegistryID)|g' -e 's|reference.FromPtrValue(mg.Spec.InitProvider.RegistryID)|reference.FromIntPtrValue(mg.Spec.InitProvider.RegistryID)|g' -e 's|mg.Spec.ForProvider.RegistryID = reference.ToPtrValue(rsp.ResolvedValue)|mg.Spec.ForProvider.RegistryID = reference.ToIntPtrValue(rsp.ResolvedValue)|g' -e 's|mg.Spec.InitProvider.RegistryID = reference.ToPtrValue(rsp.ResolvedValue)|mg.Spec.InitProvider.RegistryID = reference.ToIntPtrValue(rsp.ResolvedValue)|g' cluster/harbor/v1alpha1/zz_generated.resolvers.go namespaced/harbor/v1alpha1/zz_generated.resolvers.go && rm -f cluster/harbor/v1alpha1/zz_generated.resolvers.go.bak namespaced/harbor/v1alpha1/zz_generated.resolvers.go.bak"

package apis

import (
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen" //nolint:typecheck

	_ "github.com/crossplane/crossplane-tools/cmd/angryjet" //nolint:typecheck
)
