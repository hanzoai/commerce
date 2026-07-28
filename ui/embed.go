// Copyright © 2026 Hanzo AI. MIT License.

// Package ui exposes THE Commerce admin export as an embedded filesystem.
//
// There is one Commerce admin: app/admin (@hanzo/commerce-dashboard, Next.js
// `output: export`, built on @hanzo/ui + @hanzo/gui — the same component set
// the cloud console renders). scripts/sync-admin-ui.sh copies app/admin/out
// into ui/dist, which is baked into the commerced binary at compile time — no
// external static-assets directory, no sidecar, no separate deploy.
//
// dist/ is BUILD OUTPUT, not tracked content: gitignored but for .gitkeep, and
// produced by Dockerfile's admin-build stage before `go build`. It
// was committed until now, and a committed bundle is a bundle nobody rebuilds —
// the binary shipped a retired Vite SPA for as long as that was true.
//
// Mounted at /admin/* (commerce.go), and the export is BUILT for that path:
// next.config.ts sets `basePath: '/admin'`, so chunk URLs are /admin/_next/*
// and the client router resolves routes under the same prefix. An export built
// for "/" cannot be served from a sub-path at all — its root-absolute /_next/*
// escapes the mount and its router 404s on its own pathname.
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
