#!/bin/sh
# fetch-plans.sh — clone hanzoai/plans into ./api/billing/plans/.
#
# Commerce does NOT vendor the plans data; hanzoai/plans is the SOT
# for every plan catalog (subscription, dns, cloud, gpu, regions,
# storage, blockchain, tools, pricing-policy). CI invokes this via
# pre-build-command on the canonical docker-build.yml before
# `docker build` runs, so the Dockerfile's COPY picks up the fresh
# tree and go:embed bakes it into the binary at compile time.
#
# Local dev (e.g. `go run ./cmd/commerce`) — run this once to fetch a
# working copy. Rerun whenever you need a fresh snapshot.
#
# Env:
#   PLANS_REPO     git host/path (default: github.com/hanzoai/plans.git)
#   PLANS_VERSION  branch / tag / sha (default: main)
#   GITHUB_TOKEN   PAT or workflow token with read on hanzoai/plans
#                  (required — repo is private)

set -eu

PLANS_REPO=${PLANS_REPO:-github.com/hanzoai/plans.git}
PLANS_VERSION=${PLANS_VERSION:-main}
DEST=$(cd "$(dirname "$0")/.." && pwd)/api/billing/plans

mkdir -p "$(dirname "$DEST")"
rm -rf "$DEST"

if [ -n "${GITHUB_TOKEN:-}" ]; then
  URL="https://x-access-token:${GITHUB_TOKEN}@${PLANS_REPO}"
else
  URL="https://${PLANS_REPO}"
fi

git clone --depth=1 --branch="${PLANS_VERSION}" "$URL" "$DEST"
rm -rf "$DEST/.git" "$DEST/CLAUDE.md" "$DEST/AGENTS.md" "$DEST/LLM.md"

echo "fetched hanzoai/plans@${PLANS_VERSION} -> $DEST"
