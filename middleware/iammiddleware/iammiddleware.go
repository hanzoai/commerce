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
	"github.com/hanzoai/commerce/pkg/org"
	pkgAuth "github.com/hanzoai/commerce/pkg/auth"
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

// Client always returns nil now: there is no in-binary JWKS client.
// Legacy callers that pass this into UI handlers receive nil and
// must use the gateway-supplied identity headers instead.
func Client() *auth.IAMClient { return nil }

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

		// Grant the $5 starter credit on first encounter (idempotent).
		uid := userID
		if uid == "" {
			uid = ownerID
		}
		go func(id string) {
			bgDb := datastore.New(context.Background())
			credit.GrantIfEligible(bgDb, id, "org-created")
		}(uid)

		// Gateway-trusted identity always counts as live.
		o.Live = true

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

// GetIAMClaims is retained for source-compat. Returns nil because we
// no longer parse JWTs in-binary. Call sites should migrate to reading
// X-Org-Id / X-User-Id / X-User-Email via pkg/auth helpers.
func GetIAMClaims(_ *gin.Context) *auth.IAMClaims { return nil }

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
