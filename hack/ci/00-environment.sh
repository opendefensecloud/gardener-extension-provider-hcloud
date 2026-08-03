#! /usr/bin/env bash
set -eu

source hack/tools/install.sh

export SHOOT_HASH=${SHOOT_HASH:-$(openssl rand -hex 2)}
export SHOOT_NAME=ci-seed-$SHOOT_HASH

export TEST_SHOOT_NAME=test-$SHOOT_HASH
# Must sit inside the pinned Gardener release's SupportedVersions (1.32-1.36 for
# v1.148.0) and at or above the extension's documented floor of 1.33 (README.md).
export TEST_SHOOT_VERSION=${TEST_SHOOT_VERSION:-1.36.3}

# Default to the Gardener we actually build against rather than whatever the
# newest upstream tag happens to be — floating to LATEST silently tests a
# different Gardener than go.mod pins.
export GARDENER_VERSION=${GARDENER_VERSION:-$(go list -m -f '{{.Version}}' github.com/gardener/gardener)}

cat << EOF > hack/ci/handy.sh
export AZURE_DNS_CLIENT_ID=$AZURE_DNS_CLIENT_ID
export AZURE_DNS_CLIENT_SECRET=$AZURE_DNS_CLIENT_SECRET
export AZURE_DNS_SUBSCRIPTION_ID=$AZURE_DNS_SUBSCRIPTION_ID
export AZURE_DNS_TENANT_ID=$AZURE_DNS_TENANT_ID
export HCLOUD_TOKEN=$HCLOUD_TOKEN

export SHOOT_NAME=$SHOOT_NAME
export TEST_SHOOT_NAME=$TEST_SHOOT_NAME
export TEST_SHOOT_VERSION=$TEST_SHOOT_VERSION
# Two dirs: install.sh writes kind/kubectl/yq/helm flat into bin/, while
# gardener's tools.mk writes its tools into bin/<kernel>-<arch>/.
export PATH=$(pwd)/hack/tools/bin/:$(pwd)/hack/tools/bin/$TOOLS_KERNEL-$TOOLS_ARCH/:\$PATH
EOF

if [[ ! -d gardener ]]; then
		git clone https://github.com/gardener/gardener.git
fi
cd gardener || exit
git fetch --all

if [[ $GARDENER_VERSION == "LATEST" ]]; then
  GARDENER_VERSION=$(git tag -l 'v1.*' | sort --version-sort | tail -1)
fi

git checkout "$GARDENER_VERSION"

# NOTE: four `sed -i` patches used to be applied here against
# example/provider-extensions/{registry-seed,ssh-reverse-tunnel}/... — that whole
# tree was deleted upstream in gardener PR #13994 (2026-03-04), so with `set -e`
# they now abort this script outright. Removed rather than repointed, because the
# replacement `remote` setup (dev-setup/) is a different design, not a moved path.
#
# A fifth sed rewrote charts/images.yaml to a registry.regio.digital proxy cache;
# it was already inert (it matched `repository: hetznercloud:`, but the file
# actually reads `repository: docker.io/hetznercloud/...`) and that registry
# belongs to the 23technologies landscape, so it is gone too.
