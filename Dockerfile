# syntax=docker/dockerfile:1

############# builder
# Run the builder on the native build platform and cross-compile to the target
# arch (GOOS/GOARCH below). This avoids QEMU emulation of the whole Go toolchain.
FROM --platform=$BUILDPLATFORM golang:1.26.10@sha256:672c944cfd07599b5f1bb179869270cdc94c6254e41db4d431259693cb286b82 AS builder

ARG TARGETOS
ARG TARGETARCH
# `local` pins the build to the toolchain baked into the base image above. Without
# it, a `toolchain` directive in go.mod newer than that image silently downloads a
# second toolchain mid-build; failing loudly is better than an unpinned compiler.
ENV BINARY_PATH=/go/bin \
    CGO_ENABLED=0 \
    GOTOOLCHAIN=local \
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
# debian13: its ca-certificates (20250419+deb13u1) drops the emSign ECC Root CA - G3 and
# AAA Certificate Services roots behind the compromised eMudhra intermediates
# (CVE-2026-111943); static-debian12 still ships both.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:d62067879de890a1def9da0e7766b7d20810a5e5555cef8506b752437d7a3e88 AS base
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
