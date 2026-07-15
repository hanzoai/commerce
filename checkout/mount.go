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

// MountPublic registers the /v1/commerce/* public endpoints onto an
// already-authed zip.Router group. The caller (commerce.go setupRoutes)
// owns middleware — IAM auth, org resolution, request context, cache
// headers — so this package stays focused on tenant routing and
// upstream forwarding.
//
// The API group is passed in so we never register a duplicate /v1
// prefix, and so admin/IAM middleware composed on the group applies to
// every handler here.
func MountPublic(group zip.Router, r Resolver, fwd Forwarder) {
	// Public: tenant branding + enabled payment methods. No auth required
	// — the SPA calls this before the user signs in. The JSON is a tight
	// projection (publicView) that never includes secrets.
	group.Get("/tenant", TenantJSON(r))

	// Authenticated: deposit intent creation and follow-up. The group's
	// IAM middleware validates the bearer token before this handler runs;
	// we also sanity-check presence inside the handler to fail fast if the
	// middleware is ever reordered.
	group.Post("/deposits", Deposits(r, fwd))
	group.Post("/deposits/:id/confirm", DepositConfirm(r, fwd))
	group.Get("/deposits/:id/status", DepositStatus(r, fwd))

	// Provider-hosted webhook intake. No IAM auth — signature verification
	// happens per-provider inside WebhookIntake using the tenant's
	// configured signing key. The Resolver scopes the tenant by Host so
	// one webhook URL serves all tenants.
	group.Post("/webhooks/:provider", WebhookIntake(r))
}

// MountPublicFromStore mirrors MountPublic but reads tenant config from
// the hanzo/base-backed commerce/store rather than the legacy
// StaticResolver. New callers should prefer this; the legacy variant
// remains for tests and deployments that have not yet constructed a
// *store.Store.
func MountPublicFromStore(group zip.Router, s *store.Store, fwd Forwarder) {
	if s == nil {
		return
	}
	group.Get("/tenant", TenantJSONFromStore(s))
	// Deposits + webhook intake continue to use the Resolver adapter while
	// those flows migrate in follow-on slices. The explicit adapter goes in
	// commerce.go setupRoutes when the full flow is wired — this slice
	// covers only the tenant-facing GET and the two admin endpoints below.
}

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

// MountAdmin registers the /_/commerce/* admin endpoints onto a router
// group the caller has already wrapped with IAM + admin-role guard.
// These endpoints are tenant-scoped: every mutation derives the tenant
// from the session, never from the request body.
func MountAdmin(group zip.Router, r *StaticResolver, adminStore AdminStore) {
	a := &AdminAPI{Resolver: r, Store: adminStore}

	group.Get("/providers", a.ListProviders)
	group.Post("/providers/:name/enable", a.EnableProvider)
	group.Post("/providers/:name/disable", a.DisableProvider)
	group.Post("/providers/:name/credentials", a.UploadCredentials)
	group.Delete("/providers/:name/credentials", a.RotateCredentials)
	group.Post("/providers/:name/test", a.TestProvider)

	group.Get("/methods", a.ListMethods)
	group.Post("/methods/:method/configure", a.ConfigureMethod)

	group.Get("/idv", a.GetIDV)
	group.Put("/idv", a.SetIDV)

	group.Get("/iam", a.GetIAM)
	group.Put("/iam", a.SetIAM)

	group.Get("/audit", a.AuditLog)
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
