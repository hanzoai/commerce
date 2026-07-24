// Package paywall is the commerce admin subscription gate. Org == store ==
// namespace: an org may use the commerce admin only when it has paid ($20/mo pro
// plan), is inside a funded trial, or holds a redeemed invite code. Otherwise the
// gate returns 402 subscription_required.
//
// It reuses the existing money primitives — it does NOT invent a parallel billing
// stack:
//
//   - subscription: models/subscription (Status active|trialing) on the pro plan
//     (billing/trial.PlanSlug), the same records SubscribeWithCard writes.
//   - trial credit: the billing/trial.CreditTag deposit auto-granted on signup.
//   - invite: billing/invite (the referrals code→org primitive, reimplemented
//     natively — commerce never imports cloud).
//
// Require is the per-request middleware — a sibling to middleware.TokenRequired
// in the zip chain, mounted AFTER it so the org is already resolved. It is NOT
// mounted on /healthz, /v1/billing/*, the subscribe/trial/invite endpoints, or
// the public catalog (those are how an org acquires access, or are unauthenticated
// probes/reads), and it always passes internal service-token and platform-admin
// callers straight through.
package paywall

import (
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/invite"
	"github.com/hanzoai/commerce/billing/trial"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transaction"
)

// DeniedCode is the machine-readable denial code the console routes to an
// upgrade/redeem prompt. Mirrors the cloud edge gate's denyVerdict shape.
const DeniedCode = "subscription_required"

// Allow reasons (also the machine-readable status returned to the caller).
const (
	reasonActive      = "active"
	reasonTrialing    = "trialing"
	reasonTrialCredit = "trial_credit"
	reasonInvite      = "invite"
)

// Allowed is the single access decision. It ALLOWS when the org has, in order:
//
//  1. an active|trialing pro subscription in ANY of subDBs, OR
//  2. a live trial credit — an unexpired, positive trial-credit deposit (txDB,
//     org namespace) — the funded-trial-window signal, OR
//  3. a redeemed invite (invDB, the system-namespace invite directory).
//
// Otherwise it returns (false, DeniedCode, nil). A backing-store error is
// surfaced so the gate can fail closed with 503 rather than silently allow.
//
// subDBs is a slice because in production a subscription may live in EITHER the
// per-org file store (billing/trial via NewNamespaced) OR the shared
// namespace-scoped store (SubscribeWithCard via New) — the gate must be a strict
// superset of both money paths and never block a paying customer. txDB carries
// the trial-credit ledger; invDB is the global "system" invite directory.
func Allowed(subDBs []*datastore.Datastore, txDB, invDB *datastore.Datastore, org string, at time.Time) (bool, string, error) {
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

	if ok, err := hasTrialCredit(txDB, at); err != nil {
		return false, "", err
	} else if ok {
		return true, reasonTrialCredit, nil
	}

	if ok, err := invite.OrgRedeemed(invDB, org); err != nil {
		return false, "", err
	} else if ok {
		return true, reasonInvite, nil
	}

	return false, DeniedCode, nil
}

// hasSubscription reports whether the org holds a current pro subscription. It is
// the org-level twin of api/store.HasAccess (which is store-bound): any pro-plan
// subscription in the org namespace whose active period / trial window still
// covers `at` unlocks the org.
func hasSubscription(db *datastore.Datastore, at time.Time) (bool, string, error) {
	subs := make([]*subscription.Subscription, 0)
	if _, err := subscription.Query(db).GetAll(&subs); err != nil {
		return false, "", err
	}
	for _, sub := range subs {
		slug := sub.PlanId
		if slug == "" {
			slug = sub.Plan.Slug
		}
		if slug != trial.PlanSlug {
			continue
		}
		switch sub.Status {
		case subscription.Trialing:
			if sub.TrialEnd.After(at) {
				return true, reasonTrialing, nil
			}
		case subscription.Active:
			if sub.PeriodEnd.After(at) {
				return true, reasonActive, nil
			}
		}
	}
	return false, "", nil
}

// hasTrialCredit reports whether the org has a LIVE trial credit: an unexpired,
// positive trial-credit deposit in the ledger. The trial credit is the single
// unified, tag-scoped, expiring deposit billing/trial grants on signup (tag
// trial.CreditTag, destination kind trial.Kind); while it is unexpired and
// positive the org is inside its funded trial window and may use the admin.
func hasTrialCredit(db *datastore.Datastore, at time.Time) (bool, error) {
	root := db.NewKey("synckey", "", 1, nil)
	deps := make([]*transaction.Transaction, 0, 8)
	if _, err := transaction.Query(db).Ancestor(root).
		Filter("DestinationKind=", trial.Kind).GetAll(&deps); err != nil {
		return false, err
	}
	for _, t := range deps {
		if t.Type != transaction.Deposit || t.Amount <= 0 {
			continue
		}
		if !isTrialCreditTag(t.Tags) {
			continue
		}
		// Zero ExpiresAt means no expiry; otherwise it must still be in the future.
		if t.ExpiresAt.IsZero() || t.ExpiresAt.After(at) {
			return true, nil
		}
	}
	return false, nil
}

// isTrialCreditTag matches both the account trial tag ("trial-credit") and the
// store-scoped form ("trial-credit:<storeID>") that billing/trial writes.
func isTrialCreditTag(tags string) bool {
	return tags == trial.CreditTag || strings.HasPrefix(tags, trial.CreditTag+":")
}

// Require is the per-request access gate. Mount it as a sibling to
// middleware.TokenRequired, AFTER it, on the commerce admin route surface.
func Require(c *zip.Ctx) error {
	// Internal platform callers are never paywalled: the service token is the
	// trusted cloud→commerce dispatch, and a platform superadmin administrates
	// across orgs. Both bypass the org subscription gate.
	if middleware.IsServiceToken(c) {
		return c.Next()
	}
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
	// Subscriptions may live in EITHER money store (see Allowed); trial credit is
	// read from the shared namespace-scoped store the balance gate uses.
	shared := datastore.New(ctx)
	perOrg := datastore.NewNamespaced(ctx)
	invDB := invite.SystemDB(c.Context()) // invites — global system namespace

	allowed, reason, err := Allowed([]*datastore.Datastore{shared, perOrg}, shared, invDB, org.Name, time.Now())
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
			"message": "This store needs an active subscription. Subscribe to the $20/mo pro plan, start your trial, or redeem an invite code.",
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
