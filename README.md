# [Gardener Extension for Hetzner Cloud provider](https://gardener.cloud)

Project [Gardener](https://gardener.cloud) implements the automated management and operation of [Kubernetes](https://kubernetes.io/) clusters as a service.
Its main principle is to leverage Kubernetes concepts for all of its tasks.

With [GEP-1](https://github.com/gardener/gardener/blob/master/docs/proposals/01-extensibility.md), Gardener adopted an architecture in which vendor-specific logic lives in external controllers that implement Gardener's extension contract. This keeps Gardener core clean and provider-independent.

This controller implements Gardener's extension contract for the **Hetzner Cloud** provider. It is maintained by [OpenDefenseCloud](https://github.com/opendefensecloud) as a fork of the 23technologies extension, kept in sync with current Gardener releases.

The latest release ships a [`ControllerRegistration` resource](https://github.com/opendefensecloud/gardener-extension-provider-hcloud/releases/latest/download/controller-registration.yaml) used to register this controller with Gardener.

For more on the extensibility concepts and a detailed proposal, see [GEP-1](https://github.com/gardener/gardener/blob/master/docs/proposals/01-extensibility.md).

> **Status:** This extension is under active development. Interfaces and behaviour may change between releases. Use at your own risk.

## Compatibility

| Component | Version |
| --- | --- |
| Gardener | v1.153.1 |
| Go | 1.26 (≥ 1.26.10) |
| Hetzner Cloud CCM | v1.39.1 |
| Hetzner Cloud CSI driver | v2.24.0 |
| machine-controller-manager | v0.63.0 |
| machine-controller-manager-provider-hcloud | v0.6.0 |

The full set of shipped component images is defined in [charts/images.yaml](charts/images.yaml).

Go 1.26.10 is a hard floor (`go` directive in [go.mod](go.mod)): it adds the `crypto/x509` denylist for the compromised eMudhra intermediates (CVE-2026-111943). The runtime image is `distroless/static-debian13`, whose CA bundle no longer contains the emSign ECC Root CA - G3 and AAA Certificate Services roots behind them.

### Supported Kubernetes versions

The extension supports the Kubernetes versions supported by the pinned Gardener release (v1.153, `SupportedVersions` 1.32–1.37). The effective lower bound is **Kubernetes 1.34**, because the Hetzner Cloud CCM (v1.38+) and CSI driver (v2.24+) dropped support for 1.33.

| Version | Support |
| --- | --- |
| Kubernetes 1.37 | ✅ |
| Kubernetes 1.36 | ✅ |
| Kubernetes 1.35 | ✅ |
| Kubernetes 1.34 | ✅ (lower bound) |

See the [Gardener supported Kubernetes versions](https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/supported_k8s_versions.md) for the versions Gardener supports in general.

## Controllers and features

This extension implements the following controllers:

- `controlplane`
- `infrastructure`
- `worker`
- `healthcheck`

### Infrastructure

- Creation of private networks in Hetzner Cloud
- Registration of the Gardener SSH public key for use on the nodes

### Not (yet) supported

- Root volume customization (restricted to Hetzner Cloud image sizes and types)
- Additional data volumes
- Mapping of Gardener machine profiles to Hetzner Cloud image names

Contributions in these areas are highly appreciated.

## Documentation

- [docs/README.md](docs/README.md) — controller and feature overview
- [docs/deployment.md](docs/deployment.md) — deployment specifics for the `gardener-extension-admission-hcloud` component

## Developing locally

You can run the controller against a local or remote Gardener installation:

```sh
make start            # run the provider extension controller
make start-admission  # run the admission webhook controller
```

Common development targets:

```sh
make generate   # regenerate code, CRDs and API reference docs
make tidy       # tidy Go module dependencies (use this, not `go mod tidy` directly)
make test       # run unit tests
make verify     # check + format + test
make docker-images  # build the container images
```

Dependency management uses Go modules. Tests are written with [Ginkgo](https://github.com/onsi/ginkgo)/[Gomega](https://github.com/onsi/gomega).

## Feedback and support

Feedback and contributions are always welcome. Please report bugs or suggestions as [GitHub issues](https://github.com/opendefensecloud/gardener-extension-provider-hcloud/issues).

For general Gardener topics you can also join the [Kubernetes Slack](http://slack.k8s.io) `#gardener` channel.

## Learn more

- [Gardener landing page](https://gardener.cloud/)
- ["Gardener, the Kubernetes Botanist" blog](https://kubernetes.io/blog/2018/05/17/gardener/)
- [GEP-1 — extensibility](https://github.com/gardener/gardener/blob/master/docs/proposals/01-extensibility.md)
- [GEP-4 — new `core.gardener.cloud/v1beta1` API](https://github.com/gardener/gardener/blob/master/docs/proposals/04-new-core-gardener-cloud-apis.md)
- [Gardener extensibility documentation](https://github.com/gardener/gardener/tree/master/docs/extensions)
- [Gardener Extensions Go library](https://pkg.go.dev/github.com/gardener/gardener/extensions/pkg)
- [Gardener API reference](https://gardener.cloud/api-reference/)
