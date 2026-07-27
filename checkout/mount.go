// Package checkout mounts the hosted multi-tenant checkout into the
// commerce router. Public paths live under /v1/commerce/*; admin paths
// live under /_/commerce/*; the Vite SPA is the least-specific catch-all.
//
// Path convention (canonical, per platform rules):
//
//	GET  /v1/commerce/tenant                 public tenant config (branding)
//	POST /v1/commerce/deposits               create intent → proxy to tenant BD
//	POST /v1/commerce/deposits/:id/confirm   submit provider token
//	GET  /v1/commerce/deposits/:id/status    poll settlement
//	POST /v1/commerce/webhooks/:provider     provider-hosted webhook intake
//
//	GET    /_/commerce/providers                       list (redacted)
//	POST   /_/commerce/providers/:name/enable          toggle enabled=true
//	POST   /_/commerce/providers/:name/disable         toggle enabled=false
//	POST   /_/commerce/providers/:name/credentials     stream creds → KMS
//	DELETE /_/commerce/providers/:name/credentials     clear KMS version
//	POST   /_/commerce/providers/:name/test            sandbox $0.01 charge
//	GET    /_/commerce/methods                         derived live methods
//	POST   /_/commerce/methods/:method/configure       per-method config
//	GET    /_/commerce/idv                             IDV provider + config
//	PUT    /_/commerce/idv                             set IDV provider
//	GET    /_/commerce/iam                             IAM app config
//	PUT    /_/commerce/iam                             set IAM app config
//	GET    /_/commerce/audit                           admin action audit log
package checkout

import (
	"net/http"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/store"
)

// MountTenantAdmin registers the store-backed admin surface onto
// a router group the caller has already wrapped with IAM auth +
// admin-role checks. Only the create-tenant + list-providers handlers
// live here today; future per-tenant config endpoints (idv, iam, etc.)
// hang off the same TenantAdminAPI struct so they share the
// store-backed instance and the audit-mutation log.
//
// The legacy provider-credentials / methods / audit endpoints stay on
// the older MountAdmin path (StaticResolver-driven) until they migrate
// over to the store seam. Both groups can coexist on the same
// /_/commerce prefix because their handler paths don't overlap.
func MountTenantAdmin(group zip.Router, s *store.Store) {
	if s == nil {
		return
	}
	a := NewTenantAdminAPI(s)
	group.Post("/tenants", a.CreateTenant)
	group.Get("/providers", a.ListProviders)
}

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
	// commerce.go setupRoutes and with MountPublic/MountAdmin above.
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
// checkout SPA. /_/commerce/ is tenant admin (new). /v1/commerce/ is
// the canonical public API surface.
var apiPrefixes = []string{
	"/v1/",
	"/_/",
	"/admin/",
}
