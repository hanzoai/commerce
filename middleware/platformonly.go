// Copyright © 2026 Hanzo AI. MIT License.

package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/util/json/http"
)

// PlatformOnly restricts a route to the ONLY two principals allowed to MINT
// money / spendable balance:
//
//  1. the internal service (cloud-api → commerce), authenticated by a bearer
//     equal to COMMERCE_SERVICE_TOKEN — recorded by TokenRequired's service-token
//     branch as IsServiceToken(c); and
//  2. a Hanzo PLATFORM (global) administrator — auth.IAMClaims.GlobalAdmin(): the
//     spoof-proof isGlobalAdmin claim (gateway/EdgeAuth X-User-IsGlobalAdmin) OR
//     membership in the "admin" org.
//
// It deliberately does NOT admit the org-level Admin bit (permission.Admin). An
// org OWNER carries org-level IsAdmin=true within their own org (IAM), which the
// gateway/EdgeAuth mints into X-User-Permissions = Admin|Live; a legacy per-org
// access token can hold Admin too. Gating the money-mint billing routes on that
// bit — via TokenRequired(permission.Admin) alone — let ANY org owner self-credit
// unlimited balance (POST /v1/billing/deposit &c.) → unlimited free inference.
// That is the real-money-GA blocker this gate closes. It is the same
// org-admin-vs-global-admin anti-conflation the codebase enforces for cross-org
// actions (checkout tenant admin, the edge billing ?org override).
//
// MOUNT IT AFTER TokenRequired(permission.Admin): TokenRequired resolves the org
// (service-token + legacy paths), sets c["permissions"], and stamps the
// service-token marker; PlatformOnly then NARROWS who may proceed to the handler.
// It never widens access — a caller already rejected by TokenRequired (401) never
// reaches here.
//
// Fail-closed: neither signal present → 403, handler not reached.
func PlatformOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Verified internal service token — cloud-api's legitimate money path.
		if IsServiceToken(c) {
			c.Next()
			return
		}
		// Platform (global) admin — NOT an org owner's org-level IsAdmin.
		if iammiddleware.GetIAMClaims(c).GlobalAdmin() {
			c.Next()
			return
		}
		http.Fail(c, 403,
			"This operation requires platform-administrator or internal-service credentials.",
			errors.New("money-mint route: caller is neither the internal service token nor a platform global admin"))
	}
}
