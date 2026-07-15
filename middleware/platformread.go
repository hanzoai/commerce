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

// MayReadPlatform is THE single predicate for "may this caller READ cross-org
// PLATFORM (god-view) data" — the business metrics, vendor costs, and any other
// aggregate that spans every tenant. It admits exactly the principals a
// platform god-view trusts, and fails closed for everyone else:
//
//  1. a Hanzo PLATFORM SuperAdmin — auth.IAMClaims.IsSuperAdmin():
//     owner=="admin" (the reserved admin org), from the gateway/EdgeAuth X-User-Owner header
//     OR membership in the built-in "admin" org; and
//  2. the trusted internal service token — IsServiceToken(c), the console's OWN
//     global-admin-gated proxy forwarding (console → commerce) with the
//     COMMERCE_SERVICE_TOKEN and X-Org-Id but NO user identity; and
//  3. the same M2M forward observed structurally: the Admin permission bit is
//     present AND there is NO IAM user identity (empty Subject). An IAM user
//     ALWAYS carries a Subject (the JWT sub the gateway mints alongside any
//     permission), so "Admin bit AND no IAM Subject" uniquely identifies the
//     verified service token and can never be an org admin. This branch is the
//     one requireCostsAdmin has always used; keeping it here means costs and
//     the SaaS metrics god-view share ONE gate.
//
// It deliberately does NOT admit the org-level Admin bit (permission.Admin) held
// by an org OWNER: the gateway mints permission.Admin from a tenant's own
// IsAdmin (edgeauth.permsHeader), so gating cross-org reads on that bit would let
// ANY org owner read the whole fleet's revenue. Only a SuperAdmin (or the trusted
// service token) may. This is the same org-admin-vs-global-admin anti-conflation
// PlatformOnly/MayMintMoney enforce for the money-MINT side.
//
// Reads c["permissions"] without MustGet so a handler mounted without the token
// gate fails closed (false) rather than panicking. Fail-closed: none present → false.
func MayReadPlatform(c *zip.Ctx) bool {
	claims := iammiddleware.GetIAMClaims(c) // non-nil by contract
	if claims.IsSuperAdmin() {
		return true
	}
	if IsServiceToken(c) {
		return true
	}
	// Trusted M2M service token observed structurally: Admin bit AND no IAM user.
	if claims.Subject == "" {
		if v := c.Locals("permissions"); v != nil {
			if f, ok := v.(bit.Field); ok && f.Has(permission.Admin) {
				return true
			}
		}
	}
	return false
}

// RequirePlatformAdmin gates a handler on MayReadPlatform, writing a 403 and
// returning false when the caller is not a trusted platform principal. It is the
// in-handler boundary for every cross-org god-view: the route-level
// TokenRequired(permission.Admin) is a NO-OP on the IAM path (it short-circuits
// for any IAM-authenticated request without checking the Admin bit), so a handler
// must call this as its first line — never trust the route gate alone.
//
//	func GetRevenue(c *zip.Ctx) error {
//		if !middleware.RequirePlatformAdmin(c) { return nil }
//		...
//	}
func RequirePlatformAdmin(c *zip.Ctx) bool {
	if MayReadPlatform(c) {
		return true
	}
	_ = http.Fail(c, 403,
		"This operation requires platform-administrator or internal-service credentials.",
		errors.New("cross-org god-view: caller is neither a platform global admin nor the internal service token"))
	return false
}
