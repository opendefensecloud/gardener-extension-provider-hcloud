# syntax=docker/dockerfile:1

############# builder
FROM golang:1.26.5@sha256:ae5a2316d12f3e78fd99177dad452e6ad4f240af2d71d57b480c3477f250fec6 AS builder

ENV BINARY_PATH=/go/bin \
    CGO_ENABLED=0
WORKDIR /go/src/github.com/opendefensecloud/gardener-extension-provider-hcloud

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG EFFECTIVE_VERSION

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    make install EFFECTIVE_VERSION=$EFFECTIVE_VERSION

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
