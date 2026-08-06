// Package billing embeds the Next.js-built billing admin SPA into the
// commerce binary. The billing repo builds it and publishes it as
// ghcr.io/hanzoai/billing; the Dockerfile copies /public out of that image
// into ui/dist, which is then go:embed'd here. Local Go test runs only need
// .gitkeep in ui/dist/ so the directive resolves.
//
// Mirrors the admin/embed.go + checkout/embed.go pattern. All three SPAs
// (commerce admin, pay, billing) ship in a single commerce binary.
package billing

import (
	"embed"
	"io/fs"
)

// UIFS is the embedded billing admin SPA bundle.
//
//go:embed all:ui/dist
var UIFS embed.FS

// UISub returns the ui/dist subtree as an fs.FS — ready for http.FS.
func UISub() fs.FS {
	sub, err := fs.Sub(UIFS, "ui/dist")
	if err != nil {
		// Embedding failed — return empty FS so the binary still starts.
		return embed.FS{}
	}
	return sub
}
