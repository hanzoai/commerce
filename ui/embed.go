// Copyright © 2026 Hanzo AI. MIT License.

// Package ui exposes THE Commerce admin export as an embedded filesystem.
//
// There is one Commerce admin: app/admin (@hanzo/commerce-dashboard, Next.js
// `output: export`, built on @hanzo/ui + @hanzo/gui — the same component set
// the cloud console renders). scripts/sync-admin-ui.sh copies app/admin/out
// into ui/dist, which is baked into the commerced binary at compile time — no
// external static-assets directory, no sidecar, no separate deploy.
//
// Mounted at the ROOT /admin/* only (commerce.go). The export emits
// root-relative /_next/* chunk URLs, so it can only be served from a mount
// whose asset paths are root-relative; the old /_/commerce/ui/* mount could
// not serve it and is gone rather than serving a second, frozen bundle.
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
