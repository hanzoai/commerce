#!/bin/sh
# fetch-plans.sh — pull the latest @hanzo/plans tarball from npm and
# extract its JSON files into ./api/billing/plans/.
#
# hanzoai/plans is published as @hanzo/plans on the public npm
# registry (the catalog is customer-facing marketing data, no reason
# to gate it). Commerce uses Go's go:embed at compile time, so we
# can't `npm install` at runtime — but we CAN curl the tarball at
# build time and unpack it. No git clone, no vendored copy.
#
# Env:
#   PLANS_VERSION  semver (default: tracks the highest published)
#
# Local dev: just run this script — public npm needs no auth.

set -eu

PLANS_VERSION=${PLANS_VERSION:-1.1.1}
DEST=$(cd "$(dirname "$0")/.." && pwd)/api/billing/plans
TMP=$(mktemp -d)

mkdir -p "$DEST"
rm -f "$DEST"/*.json

curl -fsSL "https://registry.npmjs.org/@hanzo/plans/-/plans-${PLANS_VERSION}.tgz" \
  -o "$TMP/plans.tgz"
tar -xzf "$TMP/plans.tgz" -C "$TMP"
cp "$TMP"/package/*.json "$DEST/"
rm -rf "$TMP"

echo "fetched @hanzo/plans@${PLANS_VERSION} -> $DEST ($(ls "$DEST" | wc -l | tr -d ' ') files)"
