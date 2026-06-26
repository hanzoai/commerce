// Copyright © 2026 Hanzo AI. MIT License.

package middleware

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/util/permission"
)

// identityHeaders are the gateway-minted identity headers commerce trusts
// downstream (see middleware/iammiddleware). At a directly-exposed edge
// (commerce-api.hanzo.ai is NOT behind hanzoai/gateway) a client can set
// these by hand and impersonate any org — IsIAMAuthenticated only checks
// that X-Org-Id is present. EdgeAuth strips them unconditionally and only
// re-mints them from a cryptographically-verified IAM JWT.
var identityHeaders = []string{
	"X-Org-Id", "X-User-Id", "X-User-Email", "X-User-IsAdmin",
	"X-User-Permissions", "X-Roles", "X-Phone-Number",
}

// EdgeAuth is the standalone-edge trust boundary for a directly-exposed
// commerce-api. It is a NO-OP unless COMMERCE_EDGE_AUTH=true, so
// gateway-fronted deployments (where hanzoai/gateway already strips and
// mints identity) are untouched — one trust boundary at a time.
//
// When enabled, on every request it:
//
//  1. Strips client-supplied identity headers (anti-spoofing). Without
//     this, `curl -H "X-Org-Id: <org>"` reads any org's billing because
//     downstream trusts the header (gateway is supposed to have stripped
//     it — but commerce-api is not behind the gateway).
//
//  2. If a Bearer IAM JWT is present, verifies it against the IAM JWKS
//     (fail-closed, RSA-pinned) and mints X-Org-Id / X-User-Id /
//     X-User-Email / X-User-IsAdmin / X-User-Permissions from the
//     validated claims, so the existing IAMTokenRequired + handlers
//     resolve the caller's org exactly as in the gateway path.
//
//  3. For /billing/ requests, locks the billing-subject query params
//     (user / userId / customerId) to the caller's own org slug, so a
//     browser can only ever read its OWN org's billing (per-org
//     isolation) regardless of what it puts on the URL.
//
// Service tokens (COMMERCE_SERVICE_TOKEN) and hk-/sk- API keys are left
// untouched: they are not JWTs, so step 2 skips them and the existing
// TokenRequired service-token branch authorizes them as before (those
// callers carry X-Hanzo-Org, never X-Org-Id).
func EdgeAuth(iam *auth.IAMClient) gin.HandlerFunc {
	enabled := os.Getenv("COMMERCE_EDGE_AUTH") == "true"
	return func(c *gin.Context) {
		if !enabled {
			c.Next()
			return
		}

		// (1) Never trust client-supplied identity at a directly-exposed edge.
		for _, h := range identityHeaders {
			c.Request.Header.Del(h)
		}

		// (2) Mint identity from a verified IAM JWT, if one is present.
		tok := bearerToken(c.Request.Header.Get("Authorization"))
		if iam != nil && looksLikeJWT(tok) {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
			claims, err := iam.ValidateToken(ctx, tok)
			cancel()
			switch {
			case err != nil:
				// Invalid JWT → leave identity unset; downstream
				// TokenRequired returns a clean 401. Do NOT 401 here so
				// service-token / legacy-token requests still flow through.
				log.Debug("EdgeAuth: JWT rejected: %v", err)
			case claims == nil || claims.Owner == "":
				log.Debug("EdgeAuth: validated JWT has no owner claim")
			default:
				c.Request.Header.Set("X-Org-Id", claims.Owner)
				if uid := claims.Subject; uid != "" {
					c.Request.Header.Set("X-User-Id", uid)
				} else if claims.Name != "" {
					c.Request.Header.Set("X-User-Id", claims.Name)
				}
				if claims.Email != "" {
					c.Request.Header.Set("X-User-Email", claims.Email)
				}
				if claims.IsAdmin {
					c.Request.Header.Set("X-User-IsAdmin", "true")
				}
				c.Request.Header.Set("X-User-Permissions", permsHeader(claims))

				// (3) Per-org isolation: the browser never chooses whose
				// billing it reads — lock the subject to its own org slug.
				if strings.Contains(c.Request.URL.Path, "/billing/") {
					lockBillingSubject(c.Request, strings.ToLower(strings.TrimSpace(claims.Owner)))
				}
			}
		}

		c.Next()
	}
}

// bearerToken returns the token from an "Authorization: Bearer <tok>"
// header, or "" if absent/malformed.
func bearerToken(h string) string {
	const p = "Bearer "
	if len(h) > len(p) && strings.EqualFold(h[:len(p)], p) {
		return strings.TrimSpace(h[len(p):])
	}
	return ""
}

// looksLikeJWT reports whether tok has the three-segment shape of a JWS.
// Opaque service tokens and hk-/sk- API keys have no dots and are skipped.
func looksLikeJWT(tok string) bool {
	return tok != "" && strings.Count(tok, ".") == 2
}

// permsHeader mirrors the gateway: isAdmin implies Admin|Live; the
// header carries the bit.Field as a base-10 int64 (commerce parses it in
// middleware/iammiddleware.parsePermissionsHeader). IAM currently sends
// no explicit permission names, so isAdmin is the operative signal.
func permsHeader(claims *auth.IAMClaims) string {
	var f int64
	if claims.IsAdmin {
		f = int64(permission.Admin | permission.Live)
	}
	return strconv.FormatInt(f, 10)
}

// lockBillingSubject rewrites any user/userId/customerId query param to
// the caller's own org slug so a browser-issued read can never target
// another subject. Per-org billing keys on the org slug (== namespace),
// so this is the canonical key; cross-org isolation also holds because
// the namespace itself comes from the validated X-Org-Id.
func lockBillingSubject(r *http.Request, orgSlug string) {
	if orgSlug == "" {
		return
	}
	q := r.URL.Query()
	changed := false
	for _, k := range []string{"user", "userId", "customerId"} {
		if q.Has(k) {
			q.Set(k, orgSlug)
			changed = true
		}
	}
	if changed {
		r.URL.RawQuery = q.Encode()
	}
}
