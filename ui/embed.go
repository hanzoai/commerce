// Copyright © 2026 Hanzo AI. MIT License.

// Package ui exposes the built admin-commerce (Hanzo GUI v7) bundle as
// an embedded filesystem.
//
// The bundle is produced externally in the @hanzo/gui workspace at
// ~/work/hanzo/gui/apps/admin-commerce (Vite + Hanzo GUI v7) and synced
// into ui/dist by scripts/sync-admin-ui.sh. The synced dist/ is baked
// into the commerced binary at compile time — no external static-assets
// directory, no sidecar, no separate deploy.
//
// Mount the returned handler at /_/commerce/ in the commerced HTTP
// router so it serves the SPA shell for every non-API request:
//
//	import commerceui "github.com/hanzoai/commerce/ui"
//	mux.Handle("/_/commerce/", http.StripPrefix("/_/commerce", commerceui.Handler()))
//
// API routes under /v1/commerce/* take precedence via earlier route
// registration; anything else falls through to the SPA, which uses
// client-side routing so deep links survive reload.
package ui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// FS returns the embedded built-UI filesystem rooted at dist/.
// Empty when scripts/sync-admin-ui.sh has not been run.
func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return distFS
	}
	return sub
}
