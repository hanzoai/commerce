// Copyright © 2026 Hanzo AI. MIT License.

package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/zap-proto/zip"

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
	// X-User-Owner is the HOME-org (platform-sudo) anchor — stripped on ingress so a
	// client can NEVER forge `X-User-Owner: admin` to become a SuperAdmin; re-minted
	// below only from a verified JWT. X-User-IsGlobalAdmin is the RETIRED boolean —
	// no longer minted, but still stripped so a stale client copy never survives.
	"X-User-Owner", "X-User-IsGlobalAdmin", "X-User-Permissions", "X-Roles", "X-Phone-Number",
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
//  3. For /billing/ requests, locks the billing-subject to the caller's
//     own org slug — in the query params (user / userId / customerId) for
//     reads, AND in the JSON body of writes (POST/PUT/PATCH) — so a browser
//     can only ever read or WRITE its OWN org's billing (per-org isolation)
//     regardless of what it puts on the URL or in the body. Without the body
//     half, a write whose body customerId differs from the locked query
//     subject lands under an arbitrary key that every read (forced to the
//     slug) can never see — silently orphaning the record.
//
// Service tokens (COMMERCE_SERVICE_TOKEN) and sk- API keys are not JWTs,
// so step 2 skips them. Their client-supplied X-Org-Id is NOT restored to the
// trusted header (that would let IAMTokenRequired treat an unvalidated token as
// a verified identity — the bypass this boundary now closes); it is stashed in a
// PRIVATE context key that ONLY TokenRequired's service-token branch reads, after
// it has verified the bearer equals COMMERCE_SERVICE_TOKEN.
//
// ORDER: EdgeAuth MUST run BEFORE pkg/auth.Zip (both installed by Bootstrap
// via server.go installIdentityBoundary, ahead of every route group).
// pkg/auth binds the X-Org-Id header into the request CONTEXT; if EdgeAuth
// ran after it, stripping the header would leave the spoofed value in the
// context (which IAMTokenRequired reads first). Mounting EdgeAuth first
// means the identity binding only ever sees the stripped/minted headers.
//
// The IAM client is resolved lazily (iammiddleware.Client()) so mount
// order is independent of when iammiddleware.Init() runs at boot.
func EdgeAuth() zip.Handler {
	enabled := os.Getenv("COMMERCE_EDGE_AUTH") == "true"
	return func(c *zip.Ctx) error {
		if !enabled {
			return c.Next()
		}

		req := c.Fiber().Request()

		// (1) Never trust client-supplied identity at a directly-exposed edge.
		// Capture the caller-supplied org selector before stripping so the
		// validated-service-token branch (below) can honor it via a private ctx
		// key — WITHOUT re-exposing it on the trusted X-Org-Id header, which
		// IAMTokenRequired would otherwise mistake for a verified identity.
		clientOrg := string(req.Header.Peek("X-Org-Id"))

		for _, h := range identityHeaders {
			req.Header.Del(h)
		}

		// (2) Mint identity from a verified IAM JWT, if one is present.
		tok := bearerToken(string(req.Header.Peek("Authorization")))
		iam := iammiddleware.Client()
		if iam != nil && looksLikeJWT(tok) {
			ctx, cancel := context.WithTimeout(c.Context(), 8*time.Second)
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
				// X-User-Owner is the HOME org — the validated JWT `owner`, minted
				// BEFORE the /billing/ ?org override below so it always carries the
				// caller's un-switchable home org even when X-Org-Id is re-pointed at
				// another tenant for a SuperAdmin view. IsSuperAdmin (owner=="admin")
				// reads it, so an org-switch never strips sudo — and no spoofable
				// boolean is minted (the org IS the signal).
				req.Header.Set("X-User-Owner", claims.Owner)
				req.Header.Set("X-Org-Id", claims.Owner)
				if uid := claims.Subject; uid != "" {
					req.Header.Set("X-User-Id", uid)
				} else if claims.Name != "" {
					req.Header.Set("X-User-Id", claims.Name)
				}
				if claims.Email != "" {
					req.Header.Set("X-User-Email", claims.Email)
				}
				if claims.IsAdmin {
					req.Header.Set("X-User-IsAdmin", "true")
				}
				req.Header.Set("X-User-Permissions", permsHeader(claims))

				// (3) Per-org isolation: the browser never chooses whose
				// billing it reads — the subject is locked to the caller's
				// own org slug. A GLOBAL ADMIN (and only a global admin) may
				// redirect the view to another org via ?org=<slug>; for
				// everyone else the override is consumed-and-ignored so
				// isolation can never be weakened (fail-closed).
				if strings.Contains(string(req.URI().Path()), "/billing/") {
					subject, override := resolveBillingSubject(req, claims)
					if override {
						req.Header.Set("X-Org-Id", subject)
					}
					lockBillingSubject(req, subject)
					lockBillingSubjectBody(req, subject)
				}
			}
		}

		// Opaque (non-JWT) bearer: a service token or a legacy org access token.
		// Its client-supplied org selector MUST NOT go back on the trusted
		// X-Org-Id header. IAMTokenRequired runs BEFORE the token is validated and
		// treats a present X-Org-Id as a verified IAM identity (iam_authenticated
		// =true, no token check) — restoring the header was a complete auth bypass
		// (opaque bearer + X-Org-Id ⇒ forged principal for any org). Stash it in a
		// PRIVATE ctx key instead; TokenRequired's service-token branch reads it
		// ONLY after verifying the bearer == COMMERCE_SERVICE_TOKEN. X-Org-Id stays
		// stripped, so an unvalidated token can never resolve an org.
		if tok != "" && !looksLikeJWT(tok) && clientOrg != "" {
			c.Locals(ctxKeyClientOrg, clientOrg)
		}

		return c.Next()
	}
}

// ctxKeyClientOrg is the PRIVATE request-local key under which EdgeAuth stashes an
// opaque bearer's client-supplied org selector (the pre-strip X-Org-Id). It is
// read ONLY by TokenRequired's service-token branch, and only after that branch
// has verified the bearer equals COMMERCE_SERVICE_TOKEN — so an unvalidated
// token's org selector never reaches org resolution. Deliberately NOT the
// X-Org-Id header: leaving that header stripped stops IAMTokenRequired (which
// runs first) from mistaking an unvalidated value for a verified IAM identity.
const ctxKeyClientOrg = "edge_client_org"

// clientOrgFromContext returns the org selector EdgeAuth stashed for an opaque
// bearer, or "" when absent. Consumed by TokenRequired's service-token branch
// (middleware/accesstoken.go) only after the service token is verified.
func clientOrgFromContext(c *zip.Ctx) string {
	if v := c.Locals(ctxKeyClientOrg); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
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
// Opaque service tokens and sk- API keys have no dots and are skipped.
func looksLikeJWT(tok string) bool {
	return tok != "" && strings.Count(tok, ".") == 2
}

// permsHeader mints the X-User-Permissions bit.Field a verified JWT carries
// downstream (commerce parses it in middleware/iammiddleware.parsePermissionsHeader,
// gated by TokenRequired via hasScope). IAM sends no explicit permission names,
// so the admin/live posture is derived from the claim flags here — the ONE place
// the caller's authority is turned into commerce permission bits.
//
// The Admin bit is the MONEY/admin authority: TokenRequired(permission.Admin)
// gates every credit-creating and card-charging billing endpoint
// (api/billing/handlers.go — deposit, credit-grants, customer-balance/adjustments,
// auto-recharge/run-all, cycle/run-all, payouts, refund, test-mode). Because
// bit.Field.Has is intersection semantics, granting Admin to a caller lets that
// caller satisfy those gates. It is therefore GLOBAL-admin-only: an org-level
// admin (claims.IsAdmin — an org OWNER like maxpower carries it within their own
// org) must NOT mint free balance or charge cards platform-wide. Only
// claims.IsSuperAdmin() — the same spoof-proof predicate this file uses for the
// cross-org ?org billing-view override (resolveBillingSubject) — grants Admin.
//
// Live is the "real money, not sandbox" mode bit, orthogonal to authority: an org
// owner running live payment flows is exactly right. It stays on claims.IsAdmin so
// a normal org owner's own real Square top-up / welcome credit / plans / balance
// reads keep working unchanged (those user-facing routes admit any authenticated
// principal via TokenRequired() with no mask — the fix removes only the money-CREATE
// Admin bit, never the self-service surface).
func permsHeader(claims *auth.IAMClaims) string {
	var f int64
	if claims.IsAdmin {
		// Org-level admin ⇒ live (non-sandbox) mode. Deliberately NOT the Admin
		// (money/admin) bit — that is global-admin-only below.
		f |= int64(permission.Live)
	}
	if claims.IsSuperAdmin() {
		// PLATFORM admin ⇒ the money/admin authority the admin billing gates check.
		f |= int64(permission.Admin)
	}
	return strconv.FormatInt(f, 10)
}

// billingSubjectKeys name whose billing a request targets. The SAME set is
// pinned in the URL query (lockBillingSubject, every method) and in the JSON
// write body (lockBillingSubjectBody, POST/PUT/PATCH) so a read and its
// matching write can never disagree about the subject — the asymmetry that
// silently orphaned payment methods (write under the body value, reads forced
// to the slug). Per-org billing keys on the org slug (== namespace), so the
// caller's own slug is the canonical, only writable key.
var billingSubjectKeys = []string{"user", "userId", "customerId"}

// lockBillingSubject rewrites any billing-subject query param to the caller's
// own org slug so a browser-issued read can never target another subject.
// Cross-org isolation also holds because the namespace itself comes from the
// validated X-Org-Id. This is the QUERY half; lockBillingSubjectBody is the
// body half for writes.
func lockBillingSubject(r *fasthttp.Request, orgSlug string) {
	if orgSlug == "" {
		return
	}
	q, _ := url.ParseQuery(string(r.URI().QueryString()))
	changed := false
	for _, k := range billingSubjectKeys {
		if q.Has(k) {
			q.Set(k, orgSlug)
			changed = true
		}
	}
	if changed {
		r.URI().SetQueryString(q.Encode())
	}
}

// maxBillingBodyBytes caps how large a write body EdgeAuth will pin the billing
// subject in. Billing writes are small JSON objects; a body larger than this is
// passed through untouched (subject not pinned) rather than rewritten — the
// handler still receives the full, unmodified body. fasthttp has already read the
// whole body into memory (bounded by the App BodyLimit), so there is no streaming
// buffer to manage here.
const maxBillingBodyBytes = 1 << 20 // 1 MiB

// lockBillingSubjectBody is the write-side companion to lockBillingSubject: for
// a /billing/ POST/PUT/PATCH with a JSON object body it pins the same
// billing-subject fields (user/userId/customerId) to subject before the
// handler binds them, so a client can never persist a billing record under a
// subject other than its own (global-admin ?org= override still honored,
// because subject already reflects it). It is body-shape-agnostic: only those
// keys are rewritten, and only when present; arrays, primitives, other shapes,
// non-JSON content, and unparseable bodies all pass through untouched. Reads
// (GET/DELETE/...) carry the subject in the query, locked by lockBillingSubject.
func lockBillingSubjectBody(r *fasthttp.Request, subject string) {
	if subject == "" {
		return
	}
	switch string(r.Header.Method()) {
	case "POST", "PUT", "PATCH":
	default:
		return
	}
	if ct := string(r.Header.ContentType()); !strings.Contains(strings.ToLower(ct), "json") {
		return
	}

	body := r.Body()
	if len(body) > maxBillingBodyBytes {
		// Oversized: leave the complete body untouched (subject not pinned).
		return
	}

	pinned, ok := pinJSONSubjectFields(body, subject)
	if !ok {
		// Not a JSON object, no subject keys present, or unparseable: hand the
		// original bytes back untouched (never reject an unexpected shape).
		return
	}
	r.SetBody(pinned)
	r.Header.SetContentLength(len(pinned))
}

// pinJSONSubjectFields rewrites any billingSubjectKeys present at the TOP LEVEL
// of a JSON object body to subject, returning the re-encoded body and true when
// at least one key was rewritten. It returns (nil, false) — telling the caller
// to keep the original bytes — when the body is not a JSON object, carries none
// of the keys, or cannot be parsed.
//
// Decoding uses json.Number so numeric fields (e.g. billing amounts) keep their
// exact representation across the re-encode instead of being coerced through
// float64; HTML escaping is disabled so the re-encoded body stays byte-faithful
// apart from the pinned keys (and Go's canonical map-key ordering).
func pinJSONSubjectFields(raw []byte, subject string) ([]byte, bool) {
	if t := bytes.TrimLeft(raw, " \t\r\n"); len(t) == 0 || t[0] != '{' {
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return nil, false
	}
	changed := false
	for _, k := range billingSubjectKeys {
		if _, ok := obj[k]; ok {
			obj[k] = subject
			changed = true
		}
	}
	if !changed {
		return nil, false
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(obj); err != nil {
		return nil, false
	}
	return bytes.TrimRight(b.Bytes(), "\n"), true
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
// here. It is HONORED only when claims.IsSuperAdmin() holds; a non-admin's
// ?org is read, discarded, and the subject stays pinned to their own org.
// Returns (subject, override) where override means the namespace header
// (X-Org-Id) must be re-pointed at subject.
//
// SuperAdmin is the ONE canonical predicate (auth.IAMClaims.IsSuperAdmin, owner
// =="admin"), shared by every cross-tenant gate (this override, checkout tenant
// admin, the money-mint gate). Plain IsAdmin is deliberately NOT trusted: it is an
// ORG-level role (an org owner carries IsAdmin=true within their own org), so gating
// cross-org reads on it would let any org owner view another org via ?org= — the
// exact isolation break this boundary exists to stop.
func resolveBillingSubject(r *fasthttp.Request, claims *auth.IAMClaims) (string, bool) {
	own := strings.ToLower(strings.TrimSpace(claims.Owner))
	reqOrg := consumeOrgOverride(r)
	if reqOrg != "" && claims.IsSuperAdmin() {
		return reqOrg, true
	}
	return own, false
}

// consumeOrgOverride removes and returns a normalized ?org=<slug> billing-view
// override. It ALWAYS deletes the param (so it never leaks downstream) and
// returns "" when the param is absent or not a syntactically valid org slug.
func consumeOrgOverride(r *fasthttp.Request) string {
	q, _ := url.ParseQuery(string(r.URI().QueryString()))
	if !q.Has("org") {
		return ""
	}
	raw := strings.ToLower(strings.TrimSpace(q.Get("org")))
	q.Del("org")
	r.URI().SetQueryString(q.Encode())
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
