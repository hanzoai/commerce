// Copyright (c) 2026-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package middleware

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/mintauth"
	"github.com/hanzoai/commerce/util/json/http"
)

// AuthorizeMint stamps this request's datastore context as authorized to mint
// spendable balance, so a downstream ledger write (through an
// Organization.Namespaced-derived datastore) passes mintauth.Enforce. It is the
// ONE way an HTTP handler grants the mint capability the ledger sink demands —
// call it AFTER establishing a proven mint authority (MayMintMoney, a settled
// payment, or a server-fixed grant). It re-stores the "context" key, which
// Organization.Namespaced reads, so any datastore the handler builds afterward
// is authorized. Writes that build their datastore BEFORE the authority is known
// (top-up after the charge, webhook after signature) instead authorize the
// specific write via mintauth.WithAuthorized on that write's context.
func AuthorizeMint(c *zip.Ctx) {
	// SetContext, not a locals key: c.Context() IS the request context under
	// the one-context model, so the mint authorization must land THERE for
	// the ledger sink (and every datastore built from c.Context()) to see it.
	// SetContext, not a locals key: c.Context() IS the request context under
	// the one-context model, so the mint authorization must land THERE for
	// the ledger sink (and every datastore built from c.Context()) to see it.
	c.SetContext(mintauth.WithAuthorized(c.Context()))
}

// PlatformOnly restricts a route to the ONLY two principals allowed to MINT
// money / spendable balance:
//
//  1. the internal service (cloud-api → commerce), authenticated by a bearer
//     equal to COMMERCE_SERVICE_TOKEN — recorded by TokenRequired's service-token
//     branch as IsServiceToken(c); and
//  2. a Hanzo PLATFORM SuperAdmin — auth.IAMClaims.IsSuperAdmin(): membership in the
//     reserved "admin" org (HOME owner=="admin", from the gateway/EdgeAuth X-User-Owner
//     header), NOT any spoofable boolean.
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
func PlatformOnly() zip.Handler {
	// The fiber-handler shape: continue by advancing fiber's own chain.
	return func(c *zip.Ctx) error { return PlatformOnlyMW(func(c *zip.Ctx) error { return c.Next() })(c) }
}

// PlatformOnlyMW is the SAME decision in middleware shape, for zip's typed-op
// path (Router.OpScope). An op's continuation is the `next` it is handed, NOT
// fiber's chain — the op's handler is wrapped inside this middleware rather than
// chained after it. A gate that continues with c.Next() therefore finds nothing
// left to run and yields 404, which the caller then reports as an error, so an
// AUTHORIZED request to a typed mint op never reaches its handler at all.
// Proven by TestMintOpScope_AuthorizedOpRunsHandlerOnce.
//
// One decision, two shapes — deliberately not two copies. This is a money-mint
// gate; a second implementation is a second thing to get wrong, and the failure
// is silent authorization rather than a compile error.
func PlatformOnlyMW(next zip.Handler) zip.Handler {
	return func(c *zip.Ctx) error {
		if MayMintMoney(c) {
			// Proven mint principal → authorize the ledger sink for this request,
			// so the PlatformOnly-gated mint handlers (deposit, refund,
			// credit-grants, payouts, cycle, auto-recharge, …) all mint without
			// per-handler changes while org-admins are still 403'd above.
			AuthorizeMint(c)
			return next(c)
		}
		return http.Fail(c, 403,
			"This operation requires platform-administrator or internal-service credentials.",
			errors.New("money-mint route: caller is neither the internal service token nor a platform global admin"))
	}
}

// IsPlatform is THE definition of the PLATFORM PRINCIPAL — the two callers that
// act for the platform itself rather than for a tenant:
//
//  1. the verified internal service token (cloud-api → commerce, a scheduled
//     job), IsServiceToken(c); and
//  2. a Hanzo PLATFORM SuperAdmin, auth.IAMClaims.IsSuperAdmin() — the
//     reserved "admin" org membership (owner=="admin").
//
// It deliberately does NOT admit the org-level Admin bit (an org OWNER's IAM
// isAdmin, or a legacy per-org access token).
//
// It answers WHO IS CALLING and nothing else. Asking grants no capability — the
// capability gates are built ON it (MayMintMoney for money, the catalog sync for
// upstream cost), so who-may-act is defined once and each capability names
// itself. Fail-closed: neither signal present → false.
func IsPlatform(c *zip.Ctx) bool {
	return IsServiceToken(c) || iammiddleware.GetIAMClaims(c).IsSuperAdmin()
}

// MayMintMoney is THE single predicate for "may this caller MINT money /
// spendable balance": the platform principal, and only it. Use it wherever a
// mint decision is made OUTSIDE the route-middleware chain — the ZAP-over-HTTP
// dispatcher gates its money-mint method (billing.deposit) with it, and the
// allotment grant clamps a client plan override on it — so there is ONE
// expression of the mint principal, shared by the route gate (PlatformOnly) and
// every in-handler gate.
func MayMintMoney(c *zip.Ctx) bool { return IsPlatform(c) }
