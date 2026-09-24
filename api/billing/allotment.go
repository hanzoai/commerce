package billing

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/allotment"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transaction"
	txutil "github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"
)

// userSubscriptions loads every subscription keyed to `user` that speaks to a
// reader in this mode. Subscriptions are registered ancestor-less and keyed by
// UserId (the canonical create path, billing/grant.Grant + billing/trial), so they
// are queried by a bare UserId filter — NOT under the synckey ancestor, which
// matches nothing. Shared by the money-path plan resolver and the tier derivation;
// each applies its own fail-safe policy to the returned error (the mint path
// denies on error; the tier read must not downgrade a paid subscriber on a
// transient error).
//
// test is the reader's mode (org.TestMode). A subscription bought in test mode
// is left out of a live read: a sandbox processor settles test cards, so that
// sale moved no money and opens no live tier or allotment.
func userSubscriptions(db *datastore.Datastore, user string, test bool) ([]*subscription.Subscription, error) {
	all := make([]*subscription.Subscription, 0)
	if _, err := subscription.Query(db).Filter("UserId=", user).GetAll(&all); err != nil {
		return nil, err
	}
	subs := all[:0]
	for _, s := range all {
		if speaks(s, test) {
			subs = append(subs, s)
		}
	}
	return subs, nil
}

// speaks reports whether s confers its plan on a reader in this mode: a live
// subscription speaks to every reader, a test one only to a test reader.
func speaks(s *subscription.Subscription, test bool) bool {
	return !s.Test || test
}

// subscriptionPlanSlug returns the plan slug of `user`'s newest active/trialing
// subscription, or "" when none. This is the user's REAL, un-spoofable
// entitlement — the SOLE authority for how much included allotment may be minted
// on their behalf. It never trusts a client-supplied plan.
//
// Subscriptions are registered ancestor-less (orm.Register without WithParent)
// and keyed by UserId, so they are queried the SAME way
// billing/trial.findTrialSubscription queries them — a bare UserId filter, NOT
// under the synckey ancestor. The prior Ancestor(synckey) filter (inherited by
// the old resolvePlanSlug) matched NOTHING, which would make the grant clamp
// reject even a legitimate self-service grant for the user's actual plan.
func subscriptionPlanSlug(db *datastore.Datastore, user string, test bool) string {
	subs, err := userSubscriptions(db, user, test)
	if err != nil {
		return ""
	}
	return planSlugOf(subs, false)
}

// servedPlanSlug is the plan `user` is served as: subscriptionPlanSlug's answer,
// and also a past_due subscription's plan. A past_due subscriber keeps the tier
// through the retry window (deriveTier), so they keep the plan's bounds with it.
// It never mints: grants read subscriptionPlanSlug.
func servedPlanSlug(db *datastore.Datastore, user string, test bool) string {
	subs, err := userSubscriptions(db, user, test)
	if err != nil {
		return ""
	}
	return planSlugOf(subs, true)
}

// planSlugOf is the plan of the newest of one subject's subscriptions that
// speaks for them: active or trialing — and past_due when pastDue is set — not
// lapsed (engine.Lapsed), and, for a paid plan, payment-backed. "" when none
// does.
func planSlugOf(subs []*subscription.Subscription, pastDue bool) string {
	var best *subscription.Subscription
	now := time.Now()
	for _, s := range subs {
		switch s.Status {
		case subscription.Active, subscription.Trialing:
		case subscription.PastDue:
			if !pastDue {
				continue
			}
		default:
			continue
		}
		if engine.Lapsed(s, now) {
			continue
		}
		// C1-a: a PAID tier's included allotment may anchor ONLY on a
		// payment-backed subscription — never a zero-payment self-created internal
		// Active sub (CreateBillingSubscription starts one instantly). A free ($0)
		// tier is self-serve even when it carries a small included credit (a perk),
		// so it anchors as-is; price, not the allotment, is the paid-tier gate.
		slug := s.Plan.Slug
		if slug == "" {
			slug = s.PlanId
		}
		if paidTier(slug) && !subscriptionPaymentBacked(s) {
			continue
		}
		if best == nil || s.PeriodStart.After(best.PeriodStart) {
			best = s
		}
	}
	if best == nil {
		return ""
	}
	if best.Plan.Slug != "" {
		return best.Plan.Slug
	}
	return best.PlanId
}

// subscriptionPaymentBacked reports whether a subscription represents a REAL paid
// relationship — one managed by an external payment provider (Stripe/Square/…),
// or one that has progressed through invoicing (a linked invoice) — as opposed to
// the zero-payment internal subscription CreateBillingSubscription starts Active
// instantly. It is the entitlement anchor for a PAID tier's minted allotment: a
// forged internal Active sub (ProviderType="internal", no collected invoice) is
// NOT payment-backed, so its higher-tier allotment can never be minted (C1-a).
func subscriptionPaymentBacked(s *subscription.Subscription) bool {
	if s.Type == subscription.External {
		return true // opened only by recording a payment the processor already collected
	}
	switch strings.ToLower(strings.TrimSpace(s.ProviderType)) {
	case "stripe", "square", "paypal", "authorizenet", "authorize.net", "braintree":
		return true // a real external payment provider manages billing
	}
	// internal / bundle / unset: backed only once it has actually been invoiced.
	return strings.TrimSpace(s.CurrentInvoiceId) != ""
}

// resolvePlanSlug is the READ-side plan resolver (usage rollup): an explicit
// query param wins as a harmless projection preview, else the user's real
// subscription plan. It NEVER gates money — the mint path (GrantAllotment) uses
// planForGrant, which does not honor an unprivileged caller's plan override.
func resolvePlanSlug(db *datastore.Datastore, user, explicit string, test bool) string {
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		return explicit
	}
	return subscriptionPlanSlug(db, user, test)
}

// planForGrant resolves the plan whose included allotment `user` may be GRANTED.
// The cents an allotment mints (IncludedMonthlyCents) is a pure function of the
// plan, so the plan itself is the money lever — an org-level admin must NOT be
// able to name a higher tier ("max") and mint its $100/mo for free. Therefore:
//
//   - a client-supplied override is honored ONLY for a privileged MINT caller
//     (the internal service token or a platform global admin — the SAME
//     principals PlatformOnly admits and the ONLY ones allowed to mint an
//     arbitrary amount via /deposit), for legitimate comps/backfills; and
//   - for EVERYONE else it is verified against the real subscription and IGNORED
//     on mismatch — the grant clamps to the user's actually-paid entitlement
//     (subscriptionPlanSlug), which yields 0 cents when there is no subscription.
//
// This makes /allotment/grant self-service-safe (an org grants its OWN allotment,
// but only ever its OWN paid plan's amount) without a blanket platform-only gate.
func planForGrant(c *zip.Ctx, db *datastore.Datastore, user, explicit string, test bool) string {
	sub := subscriptionPlanSlug(db, user, test)
	explicit = strings.TrimSpace(explicit)
	if explicit == "" {
		return sub
	}
	if middleware.MayMintMoney(c) {
		return explicit
	}
	if sub != "" && strings.EqualFold(explicit, sub) {
		return explicit
	}
	return sub
}

type grantAllotmentRequest struct {
	User string `json:"user"`
	Plan string `json:"plan"` // optional override; else resolved from subscription
}

// GrantAllotment grants the calling/target user's plan-included monthly usage
// credit for the current UTC month, idempotently.
//
//	POST /v1/billing/allotment/grant   { "user": "hanzo/alice", "plan": "pro" }
//
// The credit lands as an expiring balance deposit, so the gateway prepaid
// balance gate (available > 0) passes while the tenant is within allotment and
// fails closed once both the included credit and any purchased balance are
// exhausted. Admin token required.
func GrantAllotment(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	var req grantAllotmentRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}
	req.User = strings.ToLower(strings.TrimSpace(req.User))
	if req.User == "" {
		return http.Fail(c, 400, "user is required", nil)
	}

	// MINT path: clamp the plan to the caller's REAL subscription unless the
	// caller is a privileged mint principal. An org-level admin cannot inflate
	// their allotment by naming a higher tier (was: resolvePlanSlug trusted
	// req.Plan verbatim → org-admin grants "max" $100/mo free — C1 miss).
	plan := planForGrant(c, db, req.User, req.Plan, org.TestMode())
	cents := IncludedMonthlyCents(plan)

	res, err := allotment.Grant(db, req.User, plan, cents, time.Now(), org.TestMode())
	if err != nil {
		log.Error("Failed to grant monthly allotment for %s: %v", req.User, err, c)
		return http.Fail(c, 500, "failed to grant monthly allotment", err)
	}

	status := 200
	if res.Granted {
		status = 201
	}
	return c.JSON(status, map[string]any{
		"user":          req.User,
		"plan":          plan,
		"currency":      "usd",
		"granted":       res.Granted,
		"reason":        res.Reason,
		"amountCents":   res.AmountCents,
		"period":        res.Period,
		"transactionId": res.TransactionId,
	})
}

// grantOrgAllotments grants the month's included allotment, for the UTC month
// containing now, to every subscriber in subs holding an active or trialing
// subscription paid through now, at the plan of the newest such subscription. A
// seat row is a subscription too, which is how each member of a per-seat plan is
// granted their own allotment. A past_due subscriber keeps the tier through the
// retry window but is not granted a new month, and neither is one whose paid
// period is over. Idempotent per (user, month).
//
// subs are the org's rows as the caller holds them, so the billing cycle grants
// against the statuses its own run just settled, and a dry run reports exactly
// the grants the real run makes. With dryRun nothing is written, and a
// subscriber already granted this month counts as skipped.
func grantOrgAllotments(db *datastore.Datastore, subs []*subscription.Subscription, now time.Time, live, dryRun bool) (granted, skipped int, results []map[string]any) {
	// Collapse to one plan per user (newest active/trialing wins), so each
	// user is granted exactly once per run regardless of subscription count.
	planByUser := make(map[string]string)
	startByUser := make(map[string]time.Time)
	for _, s := range subs {
		switch s.Status {
		case subscription.Active, subscription.Trialing:
		default:
			continue
		}
		if s.UserId == "" || !speaks(s, !live) {
			continue
		}
		if !s.PeriodEnd.IsZero() && !now.Before(s.PeriodEnd) {
			continue // its paid period is over: no month is granted on a period nobody paid for
		}
		if prev, ok := startByUser[s.UserId]; !ok || s.PeriodStart.After(prev) {
			planByUser[s.UserId] = planOf(s)
			startByUser[s.UserId] = s.PeriodStart
		}
	}

	users := make(map[string]bool, len(planByUser))
	for u := range planByUser {
		users[u] = true
	}
	results = make([]map[string]any, 0, len(planByUser))
	for _, user := range sortedUsers(users) {
		plan := planByUser[user]
		cents := IncludedMonthlyCents(plan)
		if dryRun {
			reason := ""
			switch {
			case cents <= 0:
				reason = "no_included_allotment"
			case allotment.GrantedCents(db, user, now, !live) > 0:
				reason = "already_granted"
			}
			if reason == "" {
				granted++
			} else {
				skipped++
			}
			results = append(results, map[string]any{"user": user, "plan": plan, "granted": reason == "", "reason": reason, "amountCents": cents})
			continue
		}
		res, err := allotment.Grant(db, user, plan, cents, now, !live)
		if err != nil {
			log.Error("allotment run: grant failed for %s: %v", user, err)
			results = append(results, map[string]any{"user": user, "plan": plan, "granted": false, "error": err.Error()})
			continue
		}
		if res.Granted {
			granted++
		} else {
			skipped++
		}
		results = append(results, map[string]any{
			"user":        user,
			"plan":        plan,
			"granted":     res.Granted,
			"reason":      res.Reason,
			"amountCents": res.AmountCents,
		})
	}
	return granted, skipped, results
}

// sortedUsers returns the keys of a user set in a stable order, so a run's
// grants and its report read the same way every time.
func sortedUsers(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for u := range set {
		out = append(out, u)
	}
	sort.Strings(out)
	return out
}

// RunAllotments grants the monthly included allotment to every user with an
// active/trialing subscription in the request's organization, for the current
// UTC month. Idempotent per (user, period). The billing cycle runs the same
// grants; this endpoint runs them alone.
//
//	POST /v1/billing/allotment/run
func RunAllotments(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	subs, err := orgSubscriptions(db, "")
	if err != nil {
		log.Error("Failed to list subscriptions for allotment run: %v", err, c)
		return http.Fail(c, 500, "failed to list subscriptions", err)
	}
	now := time.Now()
	// live = !TestMode: allotment grants land in the SAME bucket as charges/usage.
	granted, skipped, results := grantOrgAllotments(db, subs, now, !org.TestMode(), false)

	return c.JSON(200, map[string]any{
		"period":  allotment.Period(now),
		"granted": granted,
		"skipped": skipped,
		"results": results,
	})
}

// RollupAllotment is the PLAN side of the month: what the subscription includes,
// what has been granted against it, what has been spent out of that grant, and
// what is left of it.
//
// Every figure here is measured against the GRANT, never against the wallet. A
// holder within their allotment has spent nothing of their own, and adding these
// cents to RollupBalance counts one month of usage twice.
type RollupAllotment struct {
	// MonthlyCents is what the catalog says the plan includes each month.
	MonthlyCents int64 `json:"monthlyCents"`
	// GrantedCents is what the allotment run has actually placed on the balance
	// for this period. It matches MonthlyCents once the grant has run; before the
	// first grant it is 0 while the catalog figure already shows the entitlement.
	GrantedCents int64 `json:"grantedCents"`
	// ConsumedCents is the part of the month's spend the grant covered — never
	// more than GrantedCents, so this block stays a statement about the plan.
	ConsumedCents int64 `json:"consumedCents"`
	// RemainingCents is what is left of the grant, floored at zero.
	RemainingCents int64 `json:"remainingCents"`
}

// RollupBalance is the WALLET side: prepaid money the holder bought, exactly as
// the gateway's balance gate computes it. It is a separate block from
// RollupAllotment because the two are separate monies — one was sold with the
// plan, one was bought with a card — and a sum of them is not a number anyone
// holds.
type RollupBalance struct {
	// BalanceCents is the ledger balance.
	BalanceCents int64 `json:"balanceCents"`
	// HoldsCents is what is held against pending charges.
	HoldsCents int64 `json:"holdsCents"`
	// AvailableCents is balance less holds, floored at zero.
	AvailableCents int64 `json:"availableCents"`
}

// RollupView is a subject's month as a TYPED value: the plan, its included
// allotment, what was consumed, what ran over, and the wallet beside it.
type RollupView struct {
	// User is the billing subject this answers for.
	User string `json:"user"`
	// Plan is the slug the figures were computed against, after resolution.
	Plan string `json:"plan"`
	// Currency is the ISO code the cents are denominated in.
	Currency string `json:"currency"`
	// Period is the UTC month, YYYY-MM.
	Period string `json:"period"`
	// Windows are the plan's four nested request bounds and how much of each is
	// left — the half a holder actually asks about.
	Windows []Window `json:"windows"`
	// Included is the plan side: see RollupAllotment.
	Included RollupAllotment `json:"included"`
	// ConsumedCents is everything spent this month, inside the allotment and out.
	// Included.ConsumedCents is the part of it the grant covered.
	ConsumedCents int64 `json:"consumedCents"`
	// OverageCents is the part that ran past the grant — what the wallet pays for.
	OverageCents int64 `json:"overageCents"`
	// Balance is the wallet side: see RollupBalance.
	Balance RollupBalance `json:"balance"`
}

// errRollupNoOrg is ReadRollup REFUSING the question: with no tenant named there
// is no ledger to read, and an empty month is not the same answer as an unread one.
var errRollupNoOrg = errors.New("rollup: no organization")

// IsRollupRefusal reports whether ReadRollup refused the question — no tenant
// named — as against failing to read the ledger. The endpoint answers the first 400
// and the second 500, and a caller that collapses them shows a customer a month
// in which they spent nothing when the truth is that nothing could be read.
func IsRollupRefusal(err error) bool { return errors.Is(err, errRollupNoOrg) }

// ReadRollup is a subject's month — plan, included allotment, consumption,
// overage and balance — the QUESTION, with no HTTP in it.
//
// It takes values rather than a request so a caller that is not a request can
// ask: the same month is read over the internal plane by a peer that holds no
// ledger, and re-deriving it there would be a second answer to "how much of the
// plan is left", which is the figure a customer is shown and a gate acts on.
//
// Every figure comes off the SAME transactions the gateway's balance gate reads,
// at ONE instant (`now`), so no two of them can straddle a period boundary.
//
// An empty plan means "resolve it from the subscription", which is what an absent
// query parameter has always meant here; a named one is a projection preview and
// never mints, so it is safe to honour from anyone (the mint path resolves its
// plan through planForGrant instead). The plan the figures were actually computed
// against comes back on the value.
func ReadRollup(ctx context.Context, org *organization.Organization, user, plan string, now time.Time) (*RollupView, error) {
	if org == nil {
		return nil, errRollupNoOrg
	}
	ctx = org.Namespaced(ctx)
	db := datastore.New(ctx)

	plan = resolvePlanSlug(db, user, plan, org.TestMode())

	// Catalog-declared included allotment, and what is actually granted on the
	// balance this month (they match once the grant has run; before the first
	// grant the granted amount is 0 while the catalog amount shows the plan's
	// entitlement).
	includedMonthlyCents := IncludedMonthlyCents(plan)
	includedGrantedCents := allotment.GrantedCents(db, user, now, org.TestMode())

	// Consumption this UTC month (api-usage withdrawals).
	consumedCents := monthlyUsageCents(ctx, user, org.TestMode())

	includedRemaining := includedGrantedCents - consumedCents
	if includedRemaining < 0 {
		includedRemaining = 0
	}
	overageCents := consumedCents - includedGrantedCents
	if overageCents < 0 {
		overageCents = 0
	}
	// The part of the month's spend the GRANT covered — never more than was
	// granted, so the plan block stays a statement about the plan.
	consumedAgainstGrant := consumedCents
	if consumedAgainstGrant > includedGrantedCents {
		consumedAgainstGrant = includedGrantedCents
	}

	// Balance exactly as the gateway gate computes it.
	var balance, holds currency.Cents
	datas, err := txutil.GetTransactionsByCurrency(ctx, user, "iam-user", currency.USD, org.TestMode())
	if err != nil {
		return nil, err
	}
	if data, ok := datas.Data[currency.USD]; ok {
		balance = data.Balance
		holds = data.Holds
	}
	available := balance - holds
	if available < 0 {
		available = 0
	}

	return &RollupView{
		User:     user,
		Plan:     plan,
		Currency: "usd",
		Period:   allotment.Period(now),
		Windows:  usageWindows(ctx, user, plan, org.TestMode(), now),
		Included: RollupAllotment{
			MonthlyCents:   includedMonthlyCents,
			GrantedCents:   includedGrantedCents,
			ConsumedCents:  consumedAgainstGrant,
			RemainingCents: includedRemaining,
		},
		ConsumedCents: consumedCents,
		OverageCents:  overageCents,
		Balance: RollupBalance{
			BalanceCents:   int64(balance),
			HoldsCents:     int64(holds),
			AvailableCents: int64(available),
		},
	}, nil
}

// GetUsageRollup is the endpoint over ReadRollup. This is the single read surface
// the console billing UI renders.
//
//	GET /v1/billing/usage/rollup?user=hanzo/alice&plan=pro
//
// `plan` is optional; when omitted it is resolved from the user's subscription.
func GetUsageRollup(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	user := strings.ToLower(strings.TrimSpace(c.Query("user")))
	if user == "" {
		return http.Fail(c, 400, "user query parameter is required", nil)
	}

	view, err := ReadRollup(c.Context(), org, user, c.Query("plan"), time.Now())
	if err != nil {
		return http.Fail(c, 500, "failed to query balance", err)
	}
	return c.JSON(200, view)
}

// monthlyUsageCents sums api-usage withdrawals for a user since the first
// instant of the current UTC month. Mirrors dailyUsageCents but over the
// billing month, so the rollup's consumption aligns with the monthly allotment
// window.
func monthlyUsageCents(ctx context.Context, user string, isTest bool) int64 {
	db := datastore.New(ctx)
	rootKey := db.NewKey("synckey", "", 1, nil)

	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

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
		if !t.CreatedAt.Before(monthStart) {
			total += int64(t.Amount)
		}
	}
	return total
}
