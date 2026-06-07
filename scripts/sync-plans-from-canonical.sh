#!/bin/sh
# sync-plans-from-canonical.sh
#
# Pulls subscription.json from the canonical hanzoai/plans repo into the
# commerce binary's embedded catalog (api/billing/plans/subscription.json).
# Re-run after every change to ~/work/hanzo/plans/subscription.json so the
# binary's hanzoPlans var matches the SOT.
#
# Why a script and not a Git submodule: commerce's go:embed sees the file
# as a regular file in the same module, no submodule init / vendor dance.
# The pull is intentional — a build-time fetch from GitHub would silently
# pick up new plans the operator never reviewed.
#
# Usage:
#   scripts/sync-plans-from-canonical.sh                  # pull from ~/work/hanzo/plans
#   PLANS_DIR=/path/to/plans scripts/sync-plans-from-canonical.sh

set -eu

PLANS_DIR=${PLANS_DIR:-$HOME/work/hanzo/plans}
DEST_DIR=$(cd "$(dirname "$0")/.." && pwd)/api/billing/plans

if [ ! -f "$PLANS_DIR/subscription.json" ]; then
  echo "fatal: $PLANS_DIR/subscription.json not found" >&2
  echo "       clone github.com/hanzoai/plans into $PLANS_DIR or set PLANS_DIR" >&2
  exit 1
fi

cp "$PLANS_DIR/subscription.json" "$DEST_DIR/subscription.json"

if command -v shasum >/dev/null 2>&1; then
  CANON=$(shasum "$PLANS_DIR/subscription.json" | awk '{print $1}')
  DEST=$(shasum "$DEST_DIR/subscription.json" | awk '{print $1}')
  printf 'canonical: %s  (%s)\n' "$CANON" "$PLANS_DIR/subscription.json"
  printf 'embedded:  %s  (%s)\n' "$DEST" "$DEST_DIR/subscription.json"
  if [ "$CANON" != "$DEST" ]; then
    echo "fatal: sync produced a divergent file" >&2
    exit 1
  fi
fi

echo "synced subscription.json from $PLANS_DIR -> $DEST_DIR"
