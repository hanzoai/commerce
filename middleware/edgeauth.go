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
	"github.com/hanzoai/commerce/middleware/iammiddleware"
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
	"X-User-IsGlobalAdmin", "X-User-Permissions", "X-Roles", "X-Phone-Number",
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
// callers carry X-Org-Id, never X-Org-Id).
//
// ORDER: EdgeAuth MUST run BEFORE pkg/auth.Gin (both installed by Bootstrap
// via server.go installIdentityBoundary, ahead of every route group).
// auth.Gin binds the X-Org-Id header into the request CONTEXT; if EdgeAuth
// ran after it, stripping the header would leave the spoofed value in the
// context (which IAMTokenRequired reads first). Mounting EdgeAuth first
// means auth.Gin only ever sees the stripped/minted headers.
//
// The IAM client is resolved lazily (iammiddleware.Client()) so mount
// order is independent of when iammiddleware.Init() runs at boot.
func EdgeAuth() gin.HandlerFunc {
	enabled := os.Getenv("COMMERCE_EDGE_AUTH") == "true"
	return func(c *gin.Context) {
		if !enabled {
			c.Next()
			return
		}

		// (1) Never trust client-supplied identity at a directly-exposed edge.
		// Capture the caller-supplied org selector before stripping; the
		// service-token path (below) restores it once we know the bearer is an
		// opaque token, not a spoofable JWT-edge identity.
		clientOrg := c.Request.Header.Get("X-Org-Id")

		for _, h := range identityHeaders {
			c.Request.Header.Del(h)
		}

		// (2) Mint identity from a verified IAM JWT, if one is present.
		tok := bearerToken(c.Request.Header.Get("Authorization"))
		iam := iammiddleware.Client()
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
				// Mint the PLATFORM superadmin signal ONLY for a global admin —
				// distinct from org-level isAdmin, mirroring the gateway. This
				// is the spoof-proof header cross-org gates read.
				if isGlobalAdmin(claims) {
					c.Request.Header.Set("X-User-IsGlobalAdmin", "true")
				}
				c.Request.Header.Set("X-User-Permissions", permsHeader(claims))

				// (3) Per-org isolation: the browser never chooses whose
				// billing it reads — the subject is locked to the caller's
				// own org slug. A GLOBAL ADMIN (and only a global admin) may
				// redirect the view to another org via ?org=<slug>; for
				// everyone else the override is consumed-and-ignored so
				// isolation can never be weakened (fail-closed).
				if strings.Contains(c.Request.URL.Path, "/billing/") {
					subject, override := resolveBillingSubject(c.Request, claims)
					if override {
						c.Request.Header.Set("X-Org-Id", subject)
					}
					lockBillingSubject(c.Request, subject)
				}
			}
		}

		// Service-token path: an opaque (non-JWT) bearer names its own org via
		// X-Org-Id. Restore it here structurally only — the token itself is
		// validated downstream (accesstoken.go), which rejects forgeries before
		// any billing, so a spoofed X-Org-Id can never reach the money path.
		if tok != "" && !looksLikeJWT(tok) && clientOrg != "" {
			c.Request.Header.Set("X-Org-Id", clientOrg)
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

// resolveBillingSubject decides whose billing a verified request may read,
// and whether the org namespace header must follow that choice.
//
// Default (every caller): the subject is the caller's OWN org slug
// (claims.Owner) — strict per-org isolation. A global admin, and ONLY a
// global admin, may redirect the view to another org with ?org=<slug>.
//
// The ?org override is consumed (stripped from the query) unconditionally so
// it can never reach a handler as anything but the admin-gated signal decided
// here. It is HONORED only when isGlobalAdmin(claims) holds; a non-admin's
// ?org is read, discarded, and the subject stays pinned to their own org.
// Returns (subject, override) where override means the namespace header
// (X-Org-Id) must be re-pointed at subject.
func resolveBillingSubject(r *http.Request, claims *auth.IAMClaims) (string, bool) {
	own := strings.ToLower(strings.TrimSpace(claims.Owner))
	reqOrg := consumeOrgOverride(r)
	if reqOrg != "" && isGlobalAdmin(claims) {
		return reqOrg, true
	}
	return own, false
}

// isGlobalAdmin reports whether the verified claims belong to a real
// platform-wide administrator. Two independent signals, either suffices:
//   - the explicit isGlobalAdmin JWT claim; or
//   - membership in the global admin org (Owner == "admin"), the slug Hanzo
//     IAM seeds global admins into.
//
// Plain IsAdmin is deliberately NOT trusted: it is an ORG-level role (an org
// owner carries IsAdmin=true within their own org), so gating cross-org reads
// on it would let any org owner view another org via ?org= — the exact
// isolation break this boundary exists to stop.
func isGlobalAdmin(claims *auth.IAMClaims) bool {
	// One canonical predicate, defined on the claims type and shared by every
	// global-admin gate (edge billing ?org override, checkout tenant admin).
	return claims.GlobalAdmin()
}

// consumeOrgOverride removes and returns a normalized ?org=<slug> billing-view
// override. It ALWAYS deletes the param (so it never leaks downstream) and
// returns "" when the param is absent or not a syntactically valid org slug.
func consumeOrgOverride(r *http.Request) string {
	q := r.URL.Query()
	if !q.Has("org") {
		return ""
	}
	raw := strings.ToLower(strings.TrimSpace(q.Get("org")))
	q.Del("org")
	r.URL.RawQuery = q.Encode()
	if !validOrgSlug(raw) {
		return ""
	}
	return raw
}

// validOrgSlug accepts the lowercase [a-z0-9-] org-slug shape (1–63 chars,
// must start alphanumeric). Rejects anything else so a crafted ?org can't
// smuggle separators or path characters into the locked subject.
func validOrgSlug(s string) bool {
	if len(s) == 0 || len(s) > 63 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case c == '-' && i > 0:
		default:
			return false
		}
	}
	return true
}
