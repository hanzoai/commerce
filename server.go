// Copyright © 2026 Hanzo AI. MIT License.

package commerce

import (

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/pkg/auth"
)

// installEdgeAuth registers the directly-exposed-edge anti-spoofing boundary.
// It is the ONE place commerce's identity headers are cleaned and re-minted,
// and it MUST be mounted BEFORE any route group is registered: zip applies
// Use() middleware only to routes registered AFTER the Use() call. Mounting it
// after setupRoutes silently left /_/commerce/* and /v1/commerce/* with NO
// boundary while only the later-registered /v1 group inherited it — the
// in-cluster header-forge hole. Bootstrap is the universal chokepoint every
// entry path (cmd/commerce, cmd/commerced, the cloud mount) flows through, so
// the boundary lives there exactly once, ahead of every route.
//
// EdgeAuth (1) strips client-supplied identity headers (X-Org-Id, X-User-*,
// X-Roles, …) and (2) re-mints them ONLY from a cryptographically-verified IAM
// Bearer JWT. No-op unless COMMERCE_EDGE_AUTH=true (gateway-fronted deploys keep
// a single upstream trust boundary). It NEVER 401s — opaque service tokens and
// sk- API keys flow straight through (they are not JWTs) and X-Org-Id is
// NOT in the strip set, so the cloud-api -> commerce per-org billing money path
// is untouched. Authorization is each route's own job (IAMTokenRequired, the
// access-token middleware, handler SuperAdmin checks); EdgeAuth only guarantees
// no caller can SPOOF identity at the directly-exposed edge.
func installEdgeAuth(app *App) {
	if app == nil || app.Router == nil {
		return
	}
	app.Router.Use(middleware.EdgeAuth())
}

// installEventsLocal binds the analytics events client into every request's
// locals so handlers on ANY route group can fire best-effort analytics without
// each mount site re-wiring it. It MUST be a root-level Use during Bootstrap:
// the /v1 api.Route bundle (billing, checkout, usage) is registered AFTER
// Bootstrap returns, and zip/fiber applies earlier-registered middleware to
// later-registered routes — so this is what actually delivers the events client
// to /v1/billing/* and /v1/checkout/* (the per-group injection in setupRoutes
// only covers /v1/commerce/*). No-op when no collector is configured
// (app.Events nil), so it never blocks or fails the money path. This is the ONE
// place the events local is bound root-wide; the setupRoutes per-group injection
// predates it and stays a harmless superset for /v1/commerce.
func installEventsLocal(app *App) {
	if app == nil || app.Router == nil {
		return
	}
	app.Router.Use(zip.H(func(c *zip.Ctx) error {
		if app.Events != nil {
			c.Locals("events", app.Events)
		}
		return c.Next()
	}))
}

// installRequireGate registers the binary-edge require-identity gate that
// binds the EdgeAuth-cleaned headers into request context and, when
// require=true, 401s a request carrying NO identity. It is mounted AFTER
// setupRoutes — DELIBERATELY: that preserves the long-standing exemption of the
// probe + public routes registered there (notably /healthz, which k8s
// liveness/readiness probes hit unauthenticated, and /v1/commerce, /_/commerce
// which carry their own IAMTokenRequired/handler auth). It covers the admin SPA
// and the post-Bootstrap /v1 api.Route() bundle, exactly as before. require is
// off by default and MUST stay off wherever the service-token money path runs
// (it has no X-Org-Id) — see Config.RequireIdentity. EdgeAuth (installed before
// setupRoutes) already ran by the time this binds ctx, so ctx never holds a
// spoofed value.
func installRequireGate(app *App, require bool) {
	if app == nil || app.Router == nil {
		return
	}
	app.Router.Use(auth.Identity(require))
}
