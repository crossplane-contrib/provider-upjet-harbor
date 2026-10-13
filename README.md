# Upjet-based Crossplane provider for Harbor

<div style="text-align: center;">

![CI](https://github.com/crossplane-contrib/provider-upjet-harbor/workflows/CI/badge.svg)
[![GitHub release](https://img.shields.io/github/release/crossplane-contrib/provider-upjet-harbor/all.svg)](https://github.com/crossplane-contrib/provider-upjet-harbor/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/crossplane-contrib/provider-upjet-harbor)](https://goreportcard.com/report/github.com/crossplane-contrib/provider-upjet-harbor)
[![Contributors](https://img.shields.io/github/contributors/crossplane-contrib/provider-upjet-harbor)](https://github.com/crossplane-contrib/provider-upjet-harbor/graphs/contributors)
[![Slack](https://img.shields.io/badge/Slack-4A154B?logo=slack)](https://crossplane.slack.com)

</div>

> ⚠️ **DEPRECATION NOTICE**: xpkgs in the `jonasz-lasut` namespace are deprecated and will be removed in v1.4.0. Please migrate to the official `crossplane-contrib` namespace.

Provider Upjet-Harbor is a [Crossplane](https://crossplane.io/) provider that
is built using [Upjet](https://github.com/crossplane/upjet) code
generation tools and exposes XRM-conformant managed resources for
[Harbor](https://goharbor.io/), wrapping the
[goharbor/terraform-provider-harbor](https://github.com/goharbor/terraform-provider-harbor)
Terraform provider.

## Getting Started

Install the provider into a Crossplane control plane. The provider package is
published from this repository's CI on tag pushes; see
[Releases](https://github.com/crossplane-contrib/provider-upjet-harbor/releases) for
the published image references.

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-upjet-harbor
spec:
  package: <image-from-releases>
```

Then configure credentials for your Harbor instance via a
`ProviderConfig` / `ClusterProviderConfig`. Examples live under
[`examples/namespaced/providerconfig/`](examples/namespaced/providerconfig/);
each managed resource has an example under
[`examples/namespaced/<group>/<version>/`](examples/namespaced/).

For an end-to-end walkthrough of building and consuming Upjet-based providers,
see the upstream [Upjet generating-a-provider guide](https://github.com/crossplane/upjet/blob/main/docs/generating-a-provider.md).

For information about monitoring the Upjet runtime, see the
[Upjet Monitoring Guide](https://github.com/crossplane/upjet/blob/main/docs/monitoring.md).

## Supply Chain Security

Released images are signed and attested (SBOM, SLSA provenance) via keyless
cosign signing. See [docs/SUPPLYCHAIN.md](docs/SUPPLYCHAIN.md) for
verification instructions.

## Developing

Code-generation pipeline (re-runs Upjet + controller-gen + angryjet, then
applies the in-tree resolver patches from [`apis/generate.go`](apis/generate.go)):

```console
make generate
```

Run the provider out-of-cluster against the currently-targeted kubeconfig
(useful while iterating):

```console
make run
```

End-to-end test a single example against a local kind cluster
(spins up Crossplane, builds and loads the provider package, applies the
example, asserts `Test` condition):

```console
UPTEST_EXAMPLE_LIST="examples/namespaced/harbor/v1alpha1/registry.yaml" \
  UPTEST_CLOUD_CREDENTIALS="" \
  make e2e
```

Build the local provider package without running it:

```console
make build
```

### Add a New Resource

Follow the upstream Upjet guide for
[adding new resources](https://github.com/crossplane/upjet/blob/main/docs/adding-new-resource.md).
Per-group customizers live under [`config/namespaced/<group>/config.go`](config/namespaced/);
the directory layout mirrors [`apis/namespaced/`](apis/namespaced/), so the
Harbor Terraform resource `harbor_<group>_<name>` is configured in the matching
`config/namespaced/<group>/config.go`.

## Getting help

For filing bugs, suggesting improvements, or requesting new resources or features,
please open an
[issue](https://github.com/crossplane-contrib/provider-upjet-harbor/issues/new/choose).

For general help with Crossplane and Upjet, the
[Crossplane Slack](https://slack.crossplane.io) is the best place to ask.

## License

The provider is released under the [Apache 2.0 license](LICENSE).
