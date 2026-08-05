// Billing admin SPA HTTP handler.
//
// The embedded SPA (ui/dist, populated by the Dockerfile billing-build stage
// from github.com/hanzoai/billing) is served at /admin/billing/*. Access is
// gated on IAM roles: admin, billing_admin, owner, or superadmin — and the
// legacy IsAdmin claim. Non-admin callers (including unauthenticated) get a
// plain 404 so the route's existence does not leak.
//
// IAM auth is performed here, not by the global /v1/commerce middleware
// chain: this handler sits under /admin, outside that group. We accept the
// Bearer token, parse the IAM JWT, and check roles.
package billing

import (
	"io/fs"
	"mime"
	"net/http"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
)

// adminRoles is the set of IAM roles authorized to load the billing admin
// SPA. The IsAdmin bool claim is also sufficient.
var adminRoles = map[string]struct{}{
	"admin":         {},
	"billing_admin": {},
	"owner":         {},
	"superadmin":    {},
}

// isAuthorized returns true if the IAM claims grant access to the billing
// admin SPA. The check is purely role-based; per-tenant authorization
// happens at the API layer against /v1/commerce/billing/*.
func isAuthorized(claims *auth.IAMClaims) bool {
	if claims == nil {
		return false
	}
	if claims.IsAdmin {
		return true
	}
	for _, r := range claims.Roles {
		if _, ok := adminRoles[r]; ok {
			return true
		}
	}
	return false
}

// extractBearer returns the Bearer token from the Authorization header
// value, or an empty string if absent/malformed.
func extractBearer(h string) string {
	if h == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

// UIHandler returns a zip handler that serves the embedded billing admin
// SPA at the given mount prefix (typically "/admin/billing"). Unauthorized
// requests get a plain 404 — no existence leak.
//
// The IAMClient is optional in test harnesses; a nil client (IAM disabled)
// means every request is treated as unauthorized, which produces the same
// 404 — safer fail-closed default.
func UIHandler(prefix string, iam *auth.IAMClient) zip.Handler {
	root := UISub()

	return func(c *zip.Ctx) error {
		// Authorize first — non-admin requests never touch the FS.
		token := extractBearer(c.Header("Authorization"))
		if token == "" {
			return c.NoContent(http.StatusNotFound)
		}
		if iam == nil {
			return c.NoContent(http.StatusNotFound)
		}
		claims, err := iam.ValidateToken(c.Context(), token)
		if err != nil {
			return c.NoContent(http.StatusNotFound)
		}
		if !isAuthorized(claims) {
			return c.NoContent(http.StatusNotFound)
		}

		// Strip the mount prefix so FS lookups match embedded paths.
		path := strings.TrimPrefix(c.Path(), prefix)
		if path == "" || path == "/" {
			path = "index.html"
		}
		path = strings.TrimPrefix(path, "/")

		// Asset request (has a file extension) — serve directly with caching.
		// Read the embedded bytes and set the content type off the extension
		// (with a sniff fallback), mirroring http.FileServer without a
		// ResponseWriter.
		if i := strings.LastIndexByte(path, '.'); i >= 0 && !strings.Contains(path[i:], "/") {
			if data, err := fs.ReadFile(root, path); err == nil {
				ct := mime.TypeByExtension(path[i:])
				if ct == "" {
					ct = http.DetectContentType(data)
				}
				c.SetHeader("Cache-Control", "public, max-age=31536000, immutable")
				c.SetHeader("Content-Type", ct)
				return c.Bytes(http.StatusOK, data)
			}
		}

		// SPA fallback — serve index.html for any unmatched route under the
		// mount prefix. Client-side router resolves the rest.
		idx, err := fs.ReadFile(root, "index.html")
		if err != nil {
			return c.String(http.StatusServiceUnavailable, "billing SPA not built")
		}
		c.SetHeader("Cache-Control", "no-cache")
		c.SetHeader("Content-Type", "text/html; charset=utf-8")
		return c.Bytes(http.StatusOK, idx)
	}
}
