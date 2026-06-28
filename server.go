// Copyright © 2026 Hanzo AI. MIT License.

package commerce

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/pkg/auth"
	"github.com/hanzoai/commerce/ui"
)

// installIdentityBoundary registers the directly-exposed-edge identity
// trust boundary on the router. It is the ONE place commerce's identity
// headers are cleaned and re-minted, and it MUST be mounted BEFORE any
// route group is registered: gin applies engine.Use() middleware only to
// routes registered AFTER the Use() call (verified — see
// edgeauth_standalone_test.go), so mounting it after setupRoutes (the
// historical embed.go behaviour) silently left /_/commerce/* and
// /v1/commerce/* with NO boundary while only the later-registered /v1
// group inherited it. Bootstrap is the universal chokepoint every entry
// path (cmd/commerce, cmd/commerced, the cloud mount) flows through, so
// the boundary lives there exactly once.
//
//	EdgeAuth (1) strips client-supplied identity headers (X-Org-Id,
//	X-User-*, X-Roles, …) and (2) re-mints them ONLY from a
//	cryptographically-verified IAM Bearer JWT. No-op unless
//	COMMERCE_EDGE_AUTH=true (gateway-fronted deploys keep a single
//	upstream trust boundary). It NEVER 401s — opaque service tokens and
//	hk-/sk- API keys flow straight through (they are not JWTs), so the
//	cloud-api -> commerce per-org billing path (Bearer service token +
//	X-Hanzo-Org, which EdgeAuth does NOT strip) is untouched.
//
//	auth.Gin then binds the cleaned/minted headers into the request
//	context. EdgeAuth MUST precede it: auth.Gin reads X-Org-Id into ctx,
//	so running EdgeAuth after it would leave a spoofed value in ctx even
//	after the header was stripped. require=false here because the
//	binary-edge "reject header-less request" gate is incompatible with
//	the service-token money path (no X-Org-Id); per-route authorization
//	(IAMTokenRequired, the access-token middleware, handler GlobalAdmin
//	checks) is the gate, EdgeAuth is the anti-spoofing boundary.
func installIdentityBoundary(app *App, require bool) {
	if app == nil || app.Router == nil {
		return
	}
	app.Router.Use(middleware.EdgeAuth())
	app.Router.Use(auth.Gin(require))
}

// mountAdminSPA mounts the embedded admin SPA at /_/commerce/ui/*. It is
// called AFTER setupRoutes (preserving the historical registration order
// relative to the /_/commerce JSON admin routes) so the SPA catch-all and
// the concrete /_/commerce/tenants|providers routes never contend. The
// legacy /admin/* handler stays in place for the in-progress cutover so
// commerce.hanzo.ai keeps working while it migrates to /_/commerce/.
//
// The SPA inherits the identity boundary installed by
// installIdentityBoundary (mounted earlier in Bootstrap), so these routes
// are header-spoofing safe like every other route.
func mountAdminSPA(app *App) {
	if app == nil || app.Router == nil {
		return
	}
	// SPA mounts at /_/commerce/ui/*. The neighbouring JSON admin routes
	// (/_/commerce/tenants, /_/commerce/providers) keep their existing
	// paths — Gin can't share a wildcard with sibling concrete segments
	// at the same prefix, so the SPA gets its own subpath. The browser
	// hits commerce.hanzo.ai/_/commerce/ui/ and the React Router uses
	// basename="/_/commerce/ui" so deep links survive a refresh.
	uiHandler := http.StripPrefix("/_/commerce/ui", ui.Handler())
	app.Router.GET("/_/commerce", func(c *gin.Context) {
		http.Redirect(c.Writer, c.Request, "/_/commerce/ui/", http.StatusFound)
	})
	app.Router.GET("/_/commerce/ui", func(c *gin.Context) {
		http.Redirect(c.Writer, c.Request, "/_/commerce/ui/", http.StatusFound)
	})
	app.Router.GET("/_/commerce/ui/*filepath", gin.WrapH(uiHandler))
}
