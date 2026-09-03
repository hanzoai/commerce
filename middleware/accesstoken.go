package middleware

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/organization"
	orgpkg "github.com/hanzoai/commerce/pkg/org"
	"github.com/hanzoai/commerce/secret"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/session"
)

func splitAuthorization(fieldValue string) (string, string) {
	parts := strings.Split(fieldValue, " ")
	if len(parts) == 1 {
		return "", parts[0]
	}
	return parts[0], parts[1]
}

func accessTokenFromHeader(fieldValue string) string {
	method, accessToken := splitAuthorization(fieldValue)
	if method == "Basic" {
		bytes, _ := base64.StdEncoding.DecodeString(accessToken)
		return strings.Split(string(bytes), ":")[0]
	}
	return accessToken
}

// ParseToken is the route-middleware form: extract, stash, continue.
func ParseToken(c *zip.Ctx) error {
	parseAccessToken(c)
	return c.Next()
}

// parseAccessToken extracts the access token from query/header/session and
// stashes it in locals. Pure helper — no chain control, so in-gate callers
// (TokenRequired) can parse without running the rest of the chain.
func parseAccessToken(c *zip.Ctx) {
	// Check for `key` query param by default
	accessToken := c.Query("key")

	// Fallback to `token` query param
	if accessToken == "" {
		accessToken = c.Query("token")
	}

	// Try to grab key from Authorization header (basic auth)
	if accessToken == "" {
		accessToken = accessTokenFromHeader(c.Header("Authorization"))
	}

	// If it's still not set and dev server is running, grab from session
	if accessToken == "" && config.IsDevelopment {
		value, _ := session.Get(c, "access-token")
		if tokstr, ok := value.(string); ok {
			accessToken = tokstr
		}
	}

	c.Locals("access-token", accessToken)
}

// Permissions required to access route
func TokenPermits(masks ...bit.Mask) zip.Handler {
	// Any permissions acceptable by default (i.e., only valid token required)
	permissions := permission.All

	// Any arguments passed will be used as new permissions
	if len(masks) > 0 {
		permissions = permission.None
		for _, mask := range masks {
			permissions |= mask
		}
	}

	return func(c *zip.Ctx) error {
		// Verify permissions
		if !GetPermissions(c).Has(permissions) {
			return http.Fail(c, 403, "Token doesn't support this scope", errors.New("Token doesn't support this scope"))
		}
		return c.Next()
	}
}

// ensureIAMOrg resolves the request's org from the VALIDATED IAM identity (the
// gateway-minted X-Org-Id) and sets the "organization" context key — so commerce
// derives the org from IAM, the ONE org/user/auth authority, never its own
// Organization table and never requiring commerce-side pre-provisioning. It is
// idempotent (a no-op when iammiddleware already resolved the org upstream) and
// uses the cached GetOrCreate resolver (middleware/svcorg), so an IAM
// principal's first request auto-projects a thin billing record keyed by the IAM
// org. No org header, or a bearer-shaped selector, leaves the key unset (the
// handler's GetOrganizationOK degrades cleanly rather than seeding a bogus org).
func ensureIAMOrg(c *zip.Ctx) {
	if c.Locals("organization") != nil {
		return
	}
	orgName := strings.TrimSpace(c.Header("X-Org-Id"))
	if orgName == "" || secret.Like(orgName) {
		return
	}
	org, err := orgpkg.Resolve(c.Context(), orgName)
	if err != nil {
		log.Warn("TokenRequired: IAM org resolve for '%s' failed: %v", orgName, err)
		return
	}
	// Resolve hands back a request-owned org, so the per-request Live view is
	// set directly — no copy can leak into the shared cache.
	org.Live = !strings.EqualFold(strings.TrimSpace(c.Header("X-Hanzo-Test")), "true")
	c.Locals("organization", org)
	c.Locals("active-organization", org.Id())
}

// Parses token, default permissions check
func TokenRequired(masks ...bit.Mask) zip.Handler {
	// Any permissions acceptable by default (i.e., only valid token required)
	permissions := permission.All

	// Any arguments passed will be used as new permissions
	if len(masks) > 0 {
		permissions = permission.None
		for _, mask := range masks {
			permissions |= mask
		}
	}

	return func(c *zip.Ctx) error {
		// IAM/gateway identity path. IAMTokenRequired has already set
		// c["permissions"] from the gateway-minted X-User-Permissions (0 when
		// absent — fail closed). Enforce the SAME masks the legacy
		// path enforces: a bare c.Next() here made every masked gate
		// (e.g. TokenRequired(Admin, Published) on the checkout money routes) a
		// NO-OP for ANY IAM-authenticated principal, so a low-privilege or forged
		// (perms=0) IAM caller reached the money handlers. No masks
		// (TokenRequired()) still means "any authenticated principal" — the billing
		// read path is unchanged; with masks the caller must actually hold them.
		if iammiddleware.IsIAMAuthenticated(c) {
			if len(masks) == 0 || hasScope(c, permissions) {
				// Commerce derives the org from IAM, never its own table: resolve the
				// validated X-Org-Id via the SAME cached GetOrCreate resolver the
				// steady state uses, auto-projecting a thin billing record so an
				// IAM principal's org "just works" with no commerce-side provisioning.
				// Idempotent — a no-op when iammiddleware already resolved it upstream.
				ensureIAMOrg(c)
				return c.Next()
			}
			return http.Fail(c, 403, "Token doesn't support this scope",
				errors.New("IAM principal lacks required permission scope"))
		}

		// Parse token
		parseAccessToken(c)

		// Try to fetch access token
		accessToken := GetAccessToken(c)

		// Bail if we still don't have an access token
		if accessToken == "" {
			return http.Fail(c, 401, "No access token provided.", errors.New("No access token provided"))
		}

		ctx := c.Context()
		db := datastore.New(ctx)
		org := organization.New(db)

		// Try to find organization using access token
		tok, err := org.GetWithAccessToken(accessToken)
		if err != nil {
			return http.Fail(c, 401, "Unable to retrieve organization associated with access token: "+err.Error(), err)
		}

		// Verify token signature
		if ok, err := tok.Verify(org.SecretKey); !ok {
			log.Error("Org '%s' == '%s', Token '%s' == '%s', Verify error '%s'", org.Id(), tok.Subject, tok.String, accessToken, err, ctx)
			return http.Fail(c, 403, "Unable to verify token.", err)
		}

		// Verify permissions
		if !tok.HasPermission(permissions) {
			return http.Fail(c, 403, "Token doesn't support this scope", errors.New("Token doesn't support this scope"))
		}

		// Whether or not we can make live calls
		org.Live = tok.HasPermission(permission.Live)

		// Save organization in context
		c.Locals("permissions", tok.Permissions)
		c.Locals("organization", org)
		c.Locals("token", tok)
		return c.Next()
	}
}

func GetAccessToken(c *zip.Ctx) string {
	tok := c.Locals("access-token")
	if tok == nil {
		return ""
	}

	return tok.(string)
}

func GetPermissions(c *zip.Ctx) bit.Field {
	return c.Locals("permissions").(bit.Field)
}

// hasScope reports whether the request's resolved permissions (set by
// IAMTokenRequired for the IAM path) include the
// required mask. It reads c["permissions"] defensively (no MustGet) so a gate
// mounted without a permission-setting middleware fails CLOSED (false → 403)
// instead of panicking. bit.Field.Has is intersection semantics, so a combined
// mask like Admin|Published is satisfied by holding EITHER bit.
func hasScope(c *zip.Ctx, need bit.Mask) bool {
	v := c.Locals("permissions")
	if v == nil {
		return false
	}
	f, ok := v.(bit.Field)
	return ok && f.Has(need)
}
