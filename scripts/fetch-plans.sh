#!/bin/sh
# fetch-plans.sh — re-vendor one @hanzo/plans version from npm into
# ./api/billing/plans/.
#
# hanzoai/plans is published as @hanzo/plans on the public npm registry.
# Commerce embeds api/billing/plans/subscription.json and dns.json with
# go:embed, tracked in git and pinned by PinnedPlansVersion and its digest
# (api/billing/plans_drift_test.go). Builds and CI compile the vendored
# bytes and never run this: it is how a person re-vendors, together with
# bumping the pin and recording the new digest.
#
# Env:
#   PLANS_VERSION  the exact semver to vendor (required)
#
# Run: PLANS_VERSION=1.10.1 sh scripts/fetch-plans.sh — public npm needs no auth.

set -eu

DEST=$(cd "$(dirname "$0")/.." && pwd)/api/billing/plans
TMP=$(mktemp -d)

# The registry metadata names the exact tarball URL for the version, so a URL is
# never hand-built and cannot 404 on a missing version.
curl -fsSL "https://registry.npmjs.org/@hanzo/plans" -o "$TMP/meta.json"

PLANS_VERSION=${PLANS_VERSION:-}
if [ -z "$PLANS_VERSION" ]; then
  echo "fetch-plans: set PLANS_VERSION to the exact @hanzo/plans version to vendor" >&2
  exit 1
fi

# Extract the tarball URL for the resolved version from the metadata.
TARBALL=$(
  tr ',' '\n' < "$TMP/meta.json" \
    | grep -F "/plans-${PLANS_VERSION}.tgz" \
    | sed -n 's/.*"\(https:[^"]*plans-'"${PLANS_VERSION}"'\.tgz\)".*/\1/p' \
    | head -n1
)
if [ -z "$TARBALL" ]; then
  echo "fetch-plans: version ${PLANS_VERSION} not found in registry" >&2
  exit 1
fi

mkdir -p "$DEST"
rm -f "$DEST"/*.json

curl -fsSL "$TARBALL" -o "$TMP/plans.tgz"
tar -xzf "$TMP/plans.tgz" -C "$TMP"
cp "$TMP"/package/*.json "$DEST/"
rm -rf "$TMP"

echo "fetched @hanzo/plans@${PLANS_VERSION} -> $DEST ($(ls "$DEST" | wc -l | tr -d ' ') files)"
