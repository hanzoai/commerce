// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/hanzoai/authz"
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

// identityHeaders are every identity header an edge writes and commerce trusts
// downstream (see middleware/iammiddleware), plus the retired names a consumer may
// still read. At a directly-exposed edge a client can set any of them by hand, so
// EdgeAuth strips them all unconditionally and re-mints only from a verified IAM
// JWT. The list is authz's own: one list of what an edge writes is one list of
// what it must strip, so a new authority header cannot be missed here.
var identityHeaders = append(append([]string{}, authz.Headers...), authz.Retired...)

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
//     (fail-closed, RSA-pinned) and mints X-Org-Id / X-User-Owner / X-User-Id /
//     X-User-Email / X-User-IsAdmin / X-User-IsOrgAdmin / X-User-Permissions
//     from the validated claims, by the rule cloud's identity boundary applies
//     (tenancy), so IAMTokenRequired and the handlers resolve the caller's org
//     exactly as behind cloud. X-User-IsAdmin is the SuperAdmin verdict, decided
//     here by authz.Claims.Sudo.
//
//  3. For /billing/ requests, locks the billing subject to the org the request
//     acts in — in the query params (user / userId / customerId) for reads, AND
//     in the JSON body of writes (POST/PUT/PATCH) — so a browser can only ever
//     read or WRITE the billing of an org it acts in (per-org isolation)
//     regardless of what it puts on the URL or in the body. Without the body
//     half, a write whose body customerId differs from the locked query
//     subject lands under an arbitrary key that every read (forced to the
//     slug) can never see — silently orphaning the record.
//
// An opaque bearer (an sk- API key, a legacy org access token) is not a JWT, so
// step 2 skips it, and its client-supplied X-Org-Id is simply stripped: restoring
// it would let IAMTokenRequired treat an unvalidated token as a verified identity,
// the bypass this boundary closes. There used to be a private stash of that
// selector for a shared service token to read once verified; the token and the
// stash are gone, and an opaque bearer resolves no org here.
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

		// (1) Never trust client-supplied identity at a directly-exposed edge. The
		// client's X-Org-Id is kept as a SELECTION, which tenancy grants or refuses
		// against the signed membership set below; it is never identity.
		sel := string(req.Header.Peek("X-Org-Id"))
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
				// legacy-token requests still flow through.
				log.Debug("EdgeAuth: JWT rejected: %v", err)
			case claims == nil:
				log.Debug("EdgeAuth: validated JWT has no claims")
			default:
				// This edge decides the SuperAdmin verdict, with the one predicate:
				// authz.Claims.Sudo over the verified token's signed membership set — a
				// PERSON whose own org (orgs[0]) is "admin", verbatim. The `owner` claim
				// is the minting application's org and decides nothing; a machine never
				// qualifies. It is written once, as X-User-IsAdmin, and everything
				// downstream reads that.
				ac := claims.Authz()
				claims.Sudo = ac.Sudo()
				// A billing view selects its org with ?org=<slug>, which is consumed
				// here whoever sends it and is then a selection like any other.
				billing := strings.Contains(string(req.URI().Path()), "/billing/")
				if billing {
					if o := consumeOrgOverride(req); o != "" {
						sel = o
					}
				}
				// X-User-Owner is the subject's HOME org and X-Org-Id the org the
				// request acts in; they differ when a person selects another org of
				// their set, or a SuperAdmin any org.
				home, org := tenancy(ac, sel)
				if home != "" {
					req.Header.Set("X-User-Owner", home)
				}
				if org != "" {
					req.Header.Set("X-Org-Id", org)
				}
				if uid := claims.Subject; uid != "" {
					req.Header.Set("X-User-Id", uid)
				} else if claims.Name != "" {
					req.Header.Set("X-User-Id", claims.Name)
				}
				if claims.Email != "" {
					req.Header.Set("X-User-Email", claims.Email)
				}
				if claims.Sudo {
					req.Header.Set(iammiddleware.HeaderUserIsAdmin, "true")
				}
				req.Header.Set("X-User-Permissions", permsHeader(claims))

				// (3) Per-org isolation: the browser never chooses whose billing
				// it reads beyond the org it acts in. With no org, nothing is pinned
				// and IAMTokenRequired resolves no principal (fail closed).
				if billing {
					lockBillingSubject(req, org)
					lockBillingSubjectBody(req, org)
				}
				// The org-admin bit, for the org the request acts in, by the same
				// published predicate (authz.Claims.OrgAdmin), never by a role name.
				if ac.OrgAdmin(org) {
					req.Header.Set(iammiddleware.HeaderUserIsOrgAdmin, "true")
				}
			}
		}

		return c.Next()
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
// auto-recharge/run-all, cycle/run-all, payouts, test-mode). Because
// bit.Field.Has is intersection semantics, granting Admin to a caller lets that
// caller satisfy those gates. It is therefore SuperAdmin-only: an org-level
// admin (claims.IsAdmin — an org OWNER like maxpower carries it within their own
// org) must NOT mint free balance or charge cards platform-wide. Only
// claims.IsSuperAdmin() — the verdict this edge decided above, the same one that
// lets a SuperAdmin select any org (tenancy) — grants Admin.
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
		// (money/admin) bit — that is SuperAdmin-only below.
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
// subject other than its own (the SuperAdmin ?org= override still honored,
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

// tenancy names the subject's HOME org and the org a request ACTS in, by the rule
// cloud's identity boundary applies (SanitizeIdentity), so a standalone commerce
// and one behind cloud resolve one token to one tenant.
//
// A person's home is the first entry of the signed membership set (authz Home),
// and the org they act in is the selection that set admits, else their default
// (authz.Claims.EffectiveOrg): any org for a SuperAdmin; for anyone else an org
// the set opens, which is never the admin org, a brand's directory, or an org the
// platform runs held as a plain member. The `owner` claim of a person's token is
// the org of whichever application minted it — the brand app every customer signs
// in through — and decides neither.
//
// A program (a client_credentials token, IAM's signed `type`) IS its application,
// whose org is the `owner` claim; it carries no membership set and selects
// nothing. A person's token without a membership set, or whose home bears an
// unsafe rune, acts in no org, and every org gate downstream refuses it.
func tenancy(ac *authz.Claims, sel string) (home, org string) {
	if ac.Program() {
		if ac.Owner == "" || authz.HasUnsafeRune(ac.Owner) {
			return "", ""
		}
		return ac.Owner, ac.Owner
	}
	home = ac.Home()
	if home == "" || authz.HasUnsafeRune(home) {
		return "", ""
	}
	org, _ = ac.EffectiveOrg(sel)
	return home, org
}

// consumeOrgOverride removes and returns a ?org=<slug> billing-view selection. It
// ALWAYS deletes the param (so it never leaks downstream) and returns "" when the
// param is absent or not an org slug. The slug is taken verbatim: folding case or
// space would make one org's name select another.
func consumeOrgOverride(r *fasthttp.Request) string {
	q, _ := url.ParseQuery(string(r.URI().QueryString()))
	if !q.Has("org") {
		return ""
	}
	raw := q.Get("org")
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
