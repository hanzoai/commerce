// Package paywall is the commerce admin subscription gate. Org == store ==
// namespace: an org may use the commerce admin only when it holds a paid plan
// (billing.Unlocks: any plan on sale, or a retired one it still pays for) or a
// redeemed invite code. Otherwise the gate returns 402 subscription_required.
//
// It reuses the existing money primitives — it does NOT invent a parallel billing
// stack:
//
//   - subscription: models/subscription, the same records SubscribeWithCard
//     writes, asked by billing.Unlocks.
//   - invite: billing/invite (the referrals code→org primitive, reimplemented
//     natively — commerce never imports cloud).
//
// Require is the per-request middleware — a sibling to middleware.TokenRequired
// in the zip chain, mounted AFTER it so the org is already resolved. It is NOT
// mounted on /healthz, /v1/billing/*, the subscribe/invite endpoints, or
// the public catalog (those are how an org acquires access, or are unauthenticated
// probes/reads), and it always passes internal service-token and platform-admin
// callers straight through.
package paywall

import (
	"time"

	"github.com/zap-proto/zip"

	billing "github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/billing/invite"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/subscription"
)

// DeniedCode is the machine-readable denial code the console routes to an
// upgrade/redeem prompt. Mirrors the cloud edge gate's denyVerdict shape.
const DeniedCode = "subscription_required"

// Allow reasons (also the machine-readable status returned to the caller).
const (
	reasonActive   = "active"
	reasonTrialing = "trialing"
	reasonInvite   = "invite"
)

// Allowed is the single access decision. It ALLOWS when the org has, in order:
//
//  1. a paid plan in ANY of subDBs (billing.Unlocks), OR
//  2. a redeemed invite (invDB, the system-namespace invite directory).
//
// Otherwise it returns (false, DeniedCode, nil). A backing-store error is
// surfaced so the gate can fail closed with 503 rather than silently allow.
//
// subDBs is a slice because in production a subscription may live in EITHER the
// per-org file store (rows written through NewNamespaced) OR the shared
// namespace-scoped store (SubscribeWithCard via New) — the gate must be a strict
// superset of both money paths and never block a paying customer. invDB is the
// global "system" invite directory.
func Allowed(subDBs []*datastore.Datastore, invDB *datastore.Datastore, org string, at time.Time) (bool, string, error) {
	for _, subDB := range subDBs {
		if subDB == nil {
			continue
		}
		if ok, reason, err := hasSubscription(subDB, at); err != nil {
			return false, "", err
		} else if ok {
			return true, reason, nil
		}
	}

	if ok, err := invite.OrgRedeemed(invDB, org); err != nil {
		return false, "", err
	} else if ok {
		return true, reasonInvite, nil
	}

	return false, DeniedCode, nil
}

// hasSubscription reports whether the org holds a plan that unlocks it at `at`
// (billing.Unlocks): any paid plan in the org namespace, on sale or retired.
func hasSubscription(db *datastore.Datastore, at time.Time) (bool, string, error) {
	subs := make([]*subscription.Subscription, 0)
	if _, err := subscription.Query(db).GetAll(&subs); err != nil {
		return false, "", err
	}
	for _, sub := range subs {
		if !billing.Unlocks(sub, at) {
			continue
		}
		if sub.Status == subscription.Trialing {
			return true, reasonTrialing, nil
		}
		return true, reasonActive, nil
	}
	return false, "", nil
}

// Require is the per-request access gate. Mount it as a sibling to
// middleware.TokenRequired, AFTER it, on the commerce admin route surface.
func Require(c *zip.Ctx) error {
	// A platform superadmin administrates across orgs and is never paywalled.
	if claims := iammiddleware.GetIAMClaims(c); claims != nil && claims.IsSuperAdmin() {
		return c.Next()
	}

	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		// No org resolved — the route's own auth (TokenRequired) already 401'd or
		// this is an unauthenticated public route. Nothing to gate.
		return c.Next()
	}

	ctx := org.Namespaced(c.Context())
	// Subscriptions may live in EITHER money store (see Allowed).
	shared := datastore.New(ctx)
	perOrg := datastore.NewNamespaced(ctx)
	invDB := invite.SystemDB(c.Context()) // invites — global system namespace

	allowed, reason, err := Allowed([]*datastore.Datastore{shared, perOrg}, invDB, org.Name, time.Now())
	if err != nil {
		return unavailable(c)
	}
	if !allowed {
		return deny(c, reason)
	}
	return c.Next()
}

// deny renders the 402 subscription_required denial. Top-level `code` is the
// stable contract the console reads; the nested `error` envelope mirrors the
// cloud edge gate's denyVerdict shape so every Hanzo surface denies alike.
func deny(c *zip.Ctx, reason string) error {
	if reason == "" {
		reason = DeniedCode
	}
	return c.JSON(402, map[string]any{
		"code": DeniedCode,
		"error": map[string]string{
			"code":    DeniedCode,
			"reason":  reason,
			"message": "This store needs an active subscription. Subscribe to a plan, or redeem an invite code.",
		},
	})
}

// unavailable renders the fail-closed 503 when the billing backend can't be read,
// matching the cloud gate's balance_unavailable shape.
func unavailable(c *zip.Ctx) error {
	return c.JSON(503, map[string]any{
		"error": map[string]string{
			"code":    "billing_unavailable",
			"message": "Billing temporarily unavailable; retry.",
		},
	})
}
