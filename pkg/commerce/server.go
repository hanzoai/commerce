// Copyright © 2026 Hanzo AI. MIT License.

package commerce

import (
	"net/http"

	"github.com/gin-gonic/gin"

	commerceApp "github.com/hanzoai/commerce"
	"github.com/hanzoai/commerce/pkg/auth"
	"github.com/hanzoai/commerce/ui"
)

// mountIdentity installs the gateway-trust identity middleware on the
// existing router. It runs before any handler so the X-Org-Id /
// X-User-Id / X-User-Email headers are bound to the request context
// once and only once. The middleware is registered globally; legacy
// handlers continue to work because identity is additive, not exclusive.
//
// Also mounts the embedded admin SPA at /_/commerce/. The legacy
// /admin/* handler stays in place for the in-progress cutover so
// commerce.hanzo.ai keeps working while admin.commerce.hanzo.ai is
// migrated to /_/commerce/.
func mountIdentity(app *commerceApp.App, require bool) {
	if app == nil || app.Router == nil {
		return
	}
	app.Router.Use(auth.Gin(require))

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
