# syntax=docker/dockerfile:1

############# builder
# Run the builder on the native build platform and cross-compile to the target
# arch (GOOS/GOARCH below). This avoids QEMU emulation of the whole Go toolchain.
FROM --platform=$BUILDPLATFORM golang:1.26.5@sha256:3aff6657219a4d9c14e27fb1d8976c49c29fddb70ba835014f477e1c70636647 AS builder

ARG TARGETOS
ARG TARGETARCH
ENV BINARY_PATH=/go/bin \
    CGO_ENABLED=0 \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH
WORKDIR /go/src/github.com/opendefensecloud/gardener-extension-provider-hcloud

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG EFFECTIVE_VERSION

# `go install` places cross-compiled binaries under $GOPATH/bin/$GOOS_$GOARCH;
# flatten that subdir so the COPY paths below work for both native and cross builds.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    make install EFFECTIVE_VERSION=$EFFECTIVE_VERSION && \
    if [ -d "/go/bin/${GOOS}_${GOARCH}" ]; then mv "/go/bin/${GOOS}_${GOARCH}/"* /go/bin/; fi

############# base
FROM gcr.io/distroless/static-debian12:nonroot@sha256:aef9602f8710ec12bde19d593fed1f76c708531bb7aba205110f1029786ead7b AS base
LABEL org.opencontainers.image.source="https://github.com/opendefensecloud/gardener-extension-provider-hcloud"

WORKDIR /

############# gardener-extension-provider-hcloud
FROM base AS gardener-extension-provider-hcloud

COPY charts /charts
COPY --from=builder /go/bin/gardener-extension-provider-hcloud /gardener-extension-provider-hcloud
ENTRYPOINT ["/gardener-extension-provider-hcloud"]

############# gardener-extension-admission-hcloud
FROM base AS gardener-extension-admission-hcloud

COPY --from=builder /go/bin/gardener-extension-admission-hcloud /gardener-extension-admission-hcloud
ENTRYPOINT ["/gardener-extension-admission-hcloud"]
