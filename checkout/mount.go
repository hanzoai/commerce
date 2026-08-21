// Package checkout mounts the hosted multi-org checkout into the commerce
// router. Every route it contributes lives under /v1/commerce — the public org
// config a storefront reads before anyone signs in (GET /v1/commerce/org) — and
// the Vite SPA is the least-specific catch-all behind them.
package checkout

import (
	"net/http"
	"strings"

	"github.com/zap-proto/zip"
)

// MountSPA registers the least-specific catch-all that serves the embedded
// Vite SPA at /. zip routes by specificity, so every concrete API route wins
// over this wildcard regardless of registration order.
func MountSPA(app *zip.App) {
	spa := SPAHandler("")
	app.All("/*", func(c *zip.Ctx) error {
		// Any API path that fell through is a 404, not the SPA. Serving
		// index.html for a missing API endpoint would mask routing bugs
		// and let attackers probe namespaces by watching 200 vs 404.
		if isAPIPath(c.Path()) {
			return c.JSON(http.StatusNotFound, map[string]any{"error": "not found"})
		}
		return spa(c)
	})
}

// isAPIPath returns true for request paths that MUST NOT fall through
// to the SPA handler. The SPA answers with 200 OK + index.html, so any
// real API path leaking into it would (a) hide routing regressions and
// (b) give attackers a free oracle for path existence.
func isAPIPath(path string) bool {
	// Exact matches first — cheap and common.
	switch path {
	case "/healthz", "/readyz", "/metrics":
		return true
	}
	// Prefix matches. Keep in sync with the route groups registered in
	// commerce.go setupRoutes.
	for _, p := range apiPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// apiPrefixes enumerates path prefixes owned by the Go API surface.
// /admin/ is the embedded Next.js admin SPA served by the commerce
// binary itself and its deep-links must never fall through to the
// checkout SPA. /v1/ is every API route commerce answers, the tenant's
// own admin reads included — there is no second root.
var apiPrefixes = []string{
	"/v1/",
	"/admin/",
}
