// Copyright © 2026 Hanzo AI. MIT License.

// Package iammiddleware is the gateway-trust shim for legacy call
// sites. It used to do JWKS fetch + JWT validation in-binary (293
// LOC). That trust boundary is now hanzoai/gateway: gateway validates
// the JWT, populates X-Org-Id / X-User-Id / X-User-Email, and only
// gateway-routed traffic reaches commerced.
//
// This file preserves the public API the rest of commerce depends on
// (Init, InitKV, Client, IAMTokenRequired, IsIAMAuthenticated,
// GetIAMClaims, GetIAMTier) so the 13 call sites compile, but every
// function reads identity from the gateway-supplied headers via
// pkg/auth.
//
// Deletion target: once all call sites migrate to pkg/auth + pkg/org,
// this file can be removed wholesale.
package iammiddleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/billing/credit"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/organization"
	pkgAuth "github.com/hanzoai/commerce/pkg/auth"
	"github.com/hanzoai/commerce/pkg/org"
	"github.com/hanzoai/commerce/util/bit"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// KVCache mirrors the pkg/org KVCache interface so existing wiring
// (commerce.go: iammiddleware.InitKV(kv)) keeps working unchanged.
type KVCache = org.KVCache

var (
	mu          sync.RWMutex
	initialized bool
)

// Init is a no-op kept for source-compat with the legacy bootstrap
// call (commerce.go calls it with auth.IAMConfig). The trust boundary
// is now the gateway, not this binary.
func Init(_ *auth.IAMConfig) error {
	mu.Lock()
	defer mu.Unlock()
	initialized = true
	return nil
}

// InitKV wires the KV cache used by org-id resolution.
func InitKV(kv KVCache) { org.Bind(kv) }

// Client returns the initialized IAM client, or nil if IAM is disabled or
// Init() has not been called. Consumers outside the middleware chain (e.g.
// SPA handlers with their own auth gate) use this to validate bearer tokens
// against the same JWKS the /v1 middleware uses. Fail-closed: a nil return
// means "treat every request as unauthenticated".
//
// NOTE: returns nil unconditionally on this build — the legacy IAM
// JWKS client has been retired in favor of gateway-trusted headers.
// SPA call sites that still call Client() get a nil and fall through
// to their own header-based identity path. Wire a real client back in
// here if a non-gateway entry point ever needs JWKS validation again.
func Client() *auth.IAMClient {
	return nil
}

// orgCacheKey returns the KV key for an IAM owner → org ID mapping.
func orgCacheKey(owner string) string {
	return "iam:org_by_name:" + owner
}

// IAMTokenRequired returns a Gin middleware that:
//  1. Reads the gateway-supplied X-Org-Id / X-User-Id / X-User-Email
//     headers (already JWT-validated upstream).
//  2. Resolves the Organization via pkg/org.Resolve (KV-cached).
//  3. Sets the legacy gin context keys downstream handlers expect:
//     iam_authenticated, iam_org, iam_user_id, iam_email,
//     organization, active-organization, permissions.
//
// Missing headers: falls through (handler chain may use a legacy
// org-token instead). The gateway is the trust boundary; commerced is
// only reachable via the gateway in production, where COMMERCED_REQUIRE_IDENTITY
// rejects header-less requests at the edge of the binary.
func IAMTokenRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// pkg/auth.Gin has already attached headers to ctx; read from there.
		// If pkg/auth.Gin wasn't installed, fall back to direct header reads
		// so this middleware still works in legacy mounts.
		ctx := c.Request.Context()
		ownerID := pkgAuth.OrgID(ctx)
		userID := pkgAuth.UserID(ctx)
		email := pkgAuth.UserEmail(ctx)
		if ownerID == "" {
			ownerID = c.GetHeader(pkgAuth.HeaderOrgID)
		}
		if userID == "" {
			userID = c.GetHeader(pkgAuth.HeaderUserID)
		}
		if email == "" {
			email = c.GetHeader(pkgAuth.HeaderUserEmail)
		}

		if ownerID == "" {
			// No identity headers — fall through to legacy auth.
			c.Next()
			return
		}

		// Bound DB context — request ctx may be canceled by a navigation.
		dbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		o, err := org.Resolve(dbCtx, ownerID)
		if err != nil {
			log.Warn("iammiddleware: org resolve failed for %q: %v", ownerID, err)
			jsonhttp.Fail(c, http.StatusServiceUnavailable,
				"Unable to resolve organization: "+err.Error(), err)
			return
		}

		// Best-effort signup credit grant on first encounter. Fires in
		// the background so a slow credit check never blocks the request
		// path. Grants are idempotent (deduped by the "org-created" tag
		// inside GrantIfEligible).
		go func(id string) {
			bgDb := datastore.New(context.Background())
			credit.GrantIfEligible(bgDb, id, "org-created")
		}(userID)

		// Gateway-trusted identity counts as live by default. An explicit
		// gateway-propagated X-Hanzo-Test: true opts the request into TEST
		// mode (org.Live=false) so charges hit sandbox processors and write
		// the test ledger — the same Live=false semantics a test access
		// token carries (see middleware/accesstoken.go). The gateway is the
		// trust boundary: it only forwards this header for test orgs. The
		// flag is additive and never widens scope (test is strictly less
		// privileged than live).
		o.Live = liveFromHeaders(c)

		// Permissions are derived strictly from gateway-supplied headers,
		// never granted by mere presence of identity. The gateway MUST
		// mint X-User-Permissions from the validated JWT (see
		// hanzoai/gateway/auth_middleware.go and HEADERS.md). If the
		// header is absent we fail closed: zero permissions, no Admin,
		// no Live. The gateway is the trust boundary; this binary
		// trusts the bits it provides and nothing else.
		perms := parsePermissionsHeader(c.GetHeader(HeaderUserPermissions))

		// Mirror onto Gin keys for legacy handlers.
		c.Set("iam_authenticated", true)
		c.Set("iam_user_id", userID)
		c.Set("iam_email", email)
		c.Set("iam_org", ownerID)
		c.Set("organization", o)
		c.Set("active-organization", o.Id())
		c.Set("permissions", perms)

		c.Next()
	}
}

// HeaderTest is the gateway-propagated test-mode signal. When the
// gateway forwards "true" (only for test orgs), the request runs in
// TEST mode: org.Live=false, charges hit sandbox processors, ledger
// rows are flagged Test. Absent/any-other value ⇒ live. Mirrors the
// X-Hanzo-Test semantics in middleware/accesstoken.go. The gateway is
// the trust boundary; commerced trusts the bit it forwards.
const HeaderTest = "X-Hanzo-Test"

// liveFromHeaders reports whether a gateway-trusted request is live.
// It is live unless the gateway forwarded HeaderTest == "true". This
// is the single place that turns the gateway test signal into the
// org.Live authority that payment.ProcessorsForOrg keys sandbox-vs-
// production off of (see payment/orgsetup.go SquareConfig(!org.Live)).
// Forwards-only: never widen test → live based on identity presence.
func liveFromHeaders(c *gin.Context) bool {
	return !strings.EqualFold(strings.TrimSpace(c.GetHeader(HeaderTest)), "true")
}

// HeaderUserPermissions is the canonical gateway-minted permission
// header. It carries the bit.Field value as a base-10 int64 string
// (e.g. "3" for Live|Test). The gateway MUST set it from the
// validated JWT roles/claims; commerced reads it as-is. Missing or
// malformed values fail closed (zero permissions). Documented in
// HEADERS.md.
const HeaderUserPermissions = "X-User-Permissions"

// parsePermissionsHeader converts the gateway-minted X-User-Permissions
// value into a bit.Field. Empty or invalid input fails closed (zero
// permissions). This is the only path that turns gateway intent into
// commerced permissions; do not introduce defaults that grant rights
// based on identity presence alone.
func parsePermissionsHeader(v string) bit.Field {
	v = strings.TrimSpace(v)
	if v == "" {
		return bit.Field(0)
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return bit.Field(0)
	}
	return bit.Field(n)
}

// IsIAMAuthenticated reports whether the request was identity-attached
// by either pkg/auth.Gin (preferred) or legacy IAMTokenRequired.
func IsIAMAuthenticated(c *gin.Context) bool {
	if v, ok := c.Get("iam_authenticated"); ok {
		if b, ok := v.(bool); ok && b {
			return true
		}
	}
	return c.GetHeader(pkgAuth.HeaderOrgID) != ""
}

// GetIAMClaims returns a non-nil *auth.IAMClaims populated from the
// gateway-minted identity headers. The gateway validated the JWT and
// stamped X-Org-Id, X-User-Id, X-User-Email, X-User-IsAdmin, X-Roles
// (see hanzoai/gateway/auth_middleware.go). commerced trusts those
// bits and reflects them into a claims struct so call sites can read
// IsAdmin / Owner / Subject / Roles uniformly.
//
// Fail-closed contract: missing headers map to zero-valued fields. In
// particular, missing X-User-IsAdmin yields IsAdmin=false (not "unknown").
// Call sites MUST NOT nil-guard the return — it is always non-nil.
//
// The legacy in-test path stores a *auth.IAMClaims under the
// "iam_claims" gin key; that wins when present so tests can inject
// arbitrary claim shapes without going through HTTP.
func GetIAMClaims(c *gin.Context) *auth.IAMClaims {
	if c == nil {
		return &auth.IAMClaims{}
	}
	if v, ok := c.Get("iam_claims"); ok {
		if claims, ok := v.(*auth.IAMClaims); ok && claims != nil {
			return claims
		}
	}
	owner := strings.TrimSpace(c.GetHeader(pkgAuth.HeaderOrgID))
	user := strings.TrimSpace(c.GetHeader(pkgAuth.HeaderUserID))
	email := strings.TrimSpace(c.GetHeader(pkgAuth.HeaderUserEmail))
	isAdmin := strings.EqualFold(strings.TrimSpace(c.GetHeader(HeaderUserIsAdmin)), "true")
	roles := parseRolesHeader(c.GetHeader(HeaderRoles))

	claims := &auth.IAMClaims{
		Owner:   owner,
		Name:    user,
		Email:   email,
		IsAdmin: isAdmin,
		Roles:   roles,
	}
	// Subject is the canonical user id field IAMClaims callers read;
	// the gateway puts the JWT sub into X-User-Id.
	claims.Subject = user
	return claims
}

// HeaderUserIsAdmin is the gateway-minted "true"/"" superadmin flag.
// Only "true" (case-insensitive) is treated as admin; any other value
// (including absent) fails closed to false.
const HeaderUserIsAdmin = "X-User-IsAdmin"

// HeaderRoles is the canonical comma-joined role-name header set by
// the gateway from the JWT roles claim. Empty value -> no roles.
const HeaderRoles = "X-Roles"

// parseRolesHeader splits the X-Roles comma list into a FlexRoles
// slice, trimming whitespace and dropping empties.
func parseRolesHeader(v string) auth.FlexRoles {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make(auth.FlexRoles, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// GetIAMTier returns "" — tier is no longer derived in-binary. The
// gateway can attach an X-Tier header in a future iteration if needed.
func GetIAMTier(_ *gin.Context) string { return "" }

// orgFromContext is exported for tests that want to assert the legacy
// gin "organization" key was populated correctly.
func orgFromContext(c *gin.Context) *organization.Organization {
	if v, ok := c.Get("organization"); ok {
		if o, ok := v.(*organization.Organization); ok {
			return o
		}
	}
	return nil
}

var _ = orgFromContext // referenced in tests
