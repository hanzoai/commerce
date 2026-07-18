package billing

import (
	"context"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/tier"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"
)

// GetTier returns the billing tier, limits, and effective balance for a user.
//
// For IAM-authenticated requests the tier is read from the JWT claim.
// For service-to-service calls the tier may be passed as a query parameter.
//
//	GET /v1/billing/tier?user=hanzo/alice
//
// Response includes the tier config plus the effective available balance.
// There is no free tier: a zero-balance account has effectiveAvailable == 0
// and is gated. The daily-credit term is 0 for every tier (see billing/tier);
// onboarding funds an account once via the starter-credit grant, and once
// that is spent the account is gated until it is topped up.
func GetTier(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c.Context())

	user := strings.ToLower(strings.TrimSpace(c.Query("user")))
	if user == "" {
		return http.Fail(c, 400, "user query parameter is required", nil)
	}

	tierName, err := resolveTierName(c, user)
	if err != nil {
		// Fail-safe: a subscription-store hiccup must NOT downgrade a paid
		// subscriber to Free. Surface the error so the caller retries or holds
		// the last-known tier instead of asserting a wrong Free.
		return http.Fail(c, 500, "failed to resolve tier", err)
	}
	cfg := tier.Get(tierName)

	// Spendable balance from the SAME three-bucket split the balance endpoint
	// serves — prepaid real money AND still-active granted credits both spend
	// (credits-first). The previous bespoke prepaid-only read silently zeroed
	// accounts funded purely by a starter/promo grant, 402-gating orgs that
	// hold real spendable credit.
	cur := currency.Type("usd")
	split, err := bucketedSplit(ctx, user, cur, org.TestMode())
	if err != nil {
		return http.Fail(c, 500, "failed to query balance", err)
	}

	prepaidAvailable := split.PrepaidAvailable
	if prepaidAvailable < 0 {
		prepaidAvailable = 0
	}
	spendable := split.Available
	if spendable < 0 {
		spendable = 0
	}

	// Daily replenishing credit. This is 0 for every tier (there is no free
	// tier), so HasDailyCredits() is false and dailyRemaining stays 0 —
	// effectiveAvailable collapses to prepaidAvailable and a zero-balance
	// account is gated. The mechanism is retained (guarded by DailyCreditsCents
	// > 0) so a tier could re-enable a daily allowance by configuration alone.
	var dailyRemaining int64
	if cfg.HasDailyCredits() {
		dailyUsed := dailyUsageCents(ctx, user, org.TestMode())
		dailyRemaining = cfg.DailyCreditsCents - dailyUsed
		if dailyRemaining < 0 {
			dailyRemaining = 0
		}
	}

	effectiveAvailable := int64(spendable) + dailyRemaining

	return c.JSON(200, map[string]any{
		"user": user,
		"tier": map[string]any{
			"name":              cfg.Name,
			"displayName":       cfg.DisplayName,
			"maxAgents":         cfg.MaxAgents,
			"unlimitedAgents":   cfg.IsUnlimitedAgents(),
			"dailyCreditsCents": cfg.DailyCreditsCents,
			"allowedModels":     cfg.AllowedModels,
		},
		"balance": map[string]any{
			"currency":           cur,
			"prepaidAvailable":   prepaidAvailable,
			"creditsRemaining":   split.CreditsRemaining,
			"dailyRemaining":     dailyRemaining,
			"effectiveAvailable": effectiveAvailable,
		},
	})
}

// TierCheck is a lightweight endpoint for model-access gating.
// It returns the tier config and whether a specific model is allowed,
// without computing the full balance. Used by Chat and white-label services.
//
//	GET /v1/billing/tier-check?user=hanzo/alice&model=zen4-max
func TierCheck(c *zip.Ctx) error {
	user := strings.ToLower(strings.TrimSpace(c.Query("user")))
	if user == "" {
		return http.Fail(c, 400, "user query parameter is required", nil)
	}

	model := strings.TrimSpace(c.Query("model"))

	tierName, err := resolveTierName(c, user)
	if err != nil {
		return http.Fail(c, 500, "failed to resolve tier", err)
	}
	cfg := tier.Get(tierName)

	resp := map[string]any{
		"user": user,
		"tier": map[string]any{
			"name":          cfg.Name,
			"displayName":   cfg.DisplayName,
			"allowedModels": cfg.AllowedModels,
			"maxAgents":     cfg.MaxAgents,
		},
	}

	if model != "" {
		resp["model"] = model
		resp["allowed"] = cfg.IsModelAllowed(model)
	}

	return c.JSON(200, resp)
}

// dailyUsageCents sums the api-usage withdrawals for a user since
// midnight UTC today. This determines how much of the free-tier daily
// credit has been consumed.
func dailyUsageCents(ctx context.Context, user string, isTest bool) int64 {
	db := datastore.New(ctx)
	rootKey := db.NewKey("synckey", "", 1, nil)

	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	transs := make([]*transaction.Transaction, 0)
	q := transaction.Query(db).Ancestor(rootKey).
		Filter("Test=", isTest).
		Filter("SourceKind=", "iam-user").
		Filter("SourceId=", user).
		Filter("Tags=", "api-usage")

	if _, err := q.GetAll(&transs); err != nil {
		return 0
	}

	var total int64
	for _, t := range transs {
		if !t.CreatedAt.Before(todayStart) {
			total += int64(t.Amount)
		}
	}

	return total
}

// resolveTierName resolves the caller's REAL billing tier for `user`. An upstream
// X-Tier claim or an explicit ?tier= override wins (the service-to-service
// contract); otherwise the tier is DERIVED from the user's active/trialing
// subscription in the org's store. Both /v1/billing/tier and
// /v1/billing/tier-check route through here, so tier resolution lives in exactly
// one place.
//
// Fail-safe: a subscription lookup error is RETURNED (not swallowed to Free) so a
// transient store error can never strip a paid subscriber's tier — the handler
// surfaces it as a 5xx and the caller holds its last-known tier.
func resolveTierName(c *zip.Ctx, user string) (tier.Name, error) {
	if iamTier := iammiddleware.GetIAMTier(c); iamTier != "" {
		return tier.Parse(iamTier), nil
	}
	if qTier := strings.TrimSpace(c.Query("tier")); qTier != "" {
		return tier.Parse(qTier), nil
	}
	org, ok := middleware.GetOrganizationOK(c)
	if !ok {
		// No org on the request (should not happen under the billing group): with
		// no store to reach there is genuinely no subscription in view — Free.
		return tier.Free, nil
	}
	return deriveTier(datastore.New(org.Namespaced(c.Context())), user)
}

// deriveTier resolves a subject's REAL billing tier from their subscriptions: the
// HIGHEST tier any active/trialing subscription confers.
//
//   - a trialing subscription → Starter (the entry on-ramp)
//   - an active PAID plan → Enterprise (enterprise-category plan) else Pro
//   - no active/trialing sub, a $0 / unknown plan, or a canceled/past_due/unpaid
//     subscription → Free
//
// A plan's paid-ness and tier are read from the embedded catalog by slug
// (paidTier/lookupPlan), never the subscription's spoofable stored plan copy, so a
// forged plan name cannot inflate a tier. The paid-tier CREATION gate
// (CreateBillingSubscription rejects an org admin self-creating a paid sub) is the
// anti-forgery boundary, so — unlike the money-mint path (subscriptionPlanSlug) —
// NO payment-backed clamp is applied here: a legitimate comped/gifted paid
// subscription (ProviderType "manual_gift", no invoice) MUST still confer its
// tier. The highest qualifying tier wins so a subscriber holding several
// subscriptions is never under-granted.
//
// Fail-safe: the query error is returned, not collapsed to Free.
func deriveTier(db *datastore.Datastore, user string) (tier.Name, error) {
	subs, err := userSubscriptions(db, user)
	if err != nil {
		return tier.Free, err
	}
	best := tier.Free
	for _, s := range subs {
		var t tier.Name
		switch s.Status {
		case subscription.Trialing:
			t = tier.Starter
		case subscription.Active:
			slug := s.Plan.Slug
			if slug == "" {
				slug = s.PlanId
			}
			t = tierForActivePaidSlug(slug)
		default:
			continue // past_due / unpaid / canceled confer no tier
		}
		if tierRank(t) > tierRank(best) {
			best = t
		}
	}
	return best, nil
}

// tierForActivePaidSlug maps an ACTIVE subscription's plan slug to its tier,
// reading price + category from the embedded catalog (never the stored copy): an
// enterprise-category paid plan → Enterprise, any other paid plan → Pro, and a
// $0 / unknown plan → Free (a $0 plan such as "developer" is self-serve and
// confers no paid tier).
func tierForActivePaidSlug(slug string) tier.Name {
	if !paidTier(slug) {
		return tier.Free
	}
	if p := lookupPlan(slug); p != nil && p.Category == "enterprise" {
		return tier.Enterprise
	}
	return tier.Pro
}

// tierRank orders tiers so deriveTier keeps the highest one a subject holds.
func tierRank(n tier.Name) int {
	switch n {
	case tier.Enterprise:
		return 3
	case tier.Pro:
		return 2
	case tier.Starter:
		return 1
	default:
		return 0 // Free / unknown
	}
}
