// Copyright © 2026 Hanzo AI. MIT License.

package middleware

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/permission"
)

// RequireAdmin is the ONE admin gate the money-moving handlers use — IAM-aware
// AND legacy/service-token-aware. It fails closed (403) unless the caller is an
// admin, and is enforced INSIDE each money handler because the route-level
// TokenRequired(permission.Admin) middleware is a NO-OP on the IAM path: it
// short-circuits (c.Next) for any IAM-authenticated request WITHOUT checking the
// Admin bit (Red HIGH-4). A handler must never trust that gate on its own.
//
// Precedence (fail-closed):
//  1. Permissions bit — the legacy access token AND the service token both set
//     c["permissions"] with permission.Admin when the caller is admin
//     (middleware/accesstoken.go). Honored FIRST so the trusted M2M service-token
//     money path (cloud-api → commerce, which carries X-Org-Id) is authorized by
//     its verified token, not mistaken for a spoofable IAM-edge header identity.
//  2. IAM identity — the gateway/EdgeAuth-minted, JWT-verified claims must carry
//     org-level IsAdmin OR platform SuperAdmin.
//
// These are per-ORG money actions (the caller acts within its own resolved
// namespace), so org-level admin suffices and a global admin is also allowed
// (superset). Cross-tenant/platform actions gate on the STRICTER SuperAdmin
// predicate instead (api/catalog.requireSuperAdmin,
// checkout.isSuperadmin), never this one.
//
// Returns true when admin; writes a 403 and returns false otherwise. Reads
// c["permissions"] without MustGet so a handler mounted without the token gate
// fails closed (403) rather than panicking (500).
func RequireAdmin(c *zip.Ctx) bool {
	if IsAdmin(c) {
		return true
	}
	_ = http.Fail(c, 403, "admin privileges required", errors.New("caller is not an admin"))
	return false
}

// IsAdmin is the PREDICATE behind [RequireAdmin] — who administers this org —
// with no opinion about what to do when the answer is no.
//
// It is split out because the same question is asked in two shapes. A raw
// handler holds the request and wants the refusal written for it; a TYPED zip
// op holds only a context and must return its own error, so [Bind] records this
// answer on the context and the op reads it with [AdminFrom]. One predicate,
// two renderings — asking it two ways is how two surfaces come to disagree
// about who an administrator is.
//
// Three ways to be one, all fail-closed:
//
//  1. the Admin permission BIT — the internal service token and the legacy
//     per-org access token both set it (middleware/accesstoken.go). Honored
//     first so a verified M2M money path is authorized by its own token rather
//     than by an identity header it does not carry;
//  2. a platform SuperAdmin — the reserved "admin" org, the cross-tenant
//     exception;
//  3. the org's OWN admin, for the org this request acts in.
//
// Clause 3 accepts BOTH spellings of the org-admin flag, because the two edges
// mint different ones for the same authority: the gateway sets X-User-IsAdmin,
// while cloud's in-process identity boundary reserves that for a SuperAdmin and
// mints X-User-IsOrgAdmin for a merchant admin. Reading only the first left a
// real merchant admin 403'd behind cloud on gates whose stated bar is "org
// admin" — the same authority, refused because of where the request entered.
// iammiddleware.IsOrgAdmin carries the home==effective binding with it, so an
// org-switched principal never inherits authority over a foreign tenant.
func IsAdmin(c *zip.Ctx) bool {
	if v := c.Locals("permissions"); v != nil {
		if f, ok := v.(bit.Field); ok && f.Has(permission.Admin) {
			return true
		}
	}
	if !iammiddleware.IsIAMAuthenticated(c) {
		return false
	}
	if iammiddleware.IsOrgAdmin(c) {
		return true
	}
	claims := iammiddleware.GetIAMClaims(c) // non-nil by contract
	return claims.IsAdmin || claims.IsSuperAdmin()
}
