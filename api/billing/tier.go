package billing

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/tier"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"
)

// TierLimits is what a tier ALLOWS, as the wire has always carried it: the
// registry's configuration plus the one fact that is derived rather than stored.
//
// tier.Config is embedded, not copied field by field. The registry is the
// authority on what a tier allows, and a second declaration of its five fields
// here is how a read and a gate come to disagree about maxAgents.
type TierLimits struct {
	tier.Config
	// MaxAgents and MaxBots are what the customer's PLAN includes — the capacity
	// published in the catalog (subscription.json `limits.agents` / `limits.bots`),
	// read by slug through AgentsIncluded and BotsIncluded. They are composed here
	// rather than stored on tier.Config, because a tier is not a plan: six slugs
	// collapse onto four tier names, so a registry keyed by name cannot answer a
	// question the catalog answers per slug.
	//
	// The WIRE reading is the one this field has always had, and it is not the
	// catalog's: 0 means NO CEILING. The two agree wherever it matters — an
	// unlimited plan (catalog -1) and a plan the catalog is silent about both
	// serve without a bound, which is 0 here — and `capacity` is the one place the
	// translation happens, with TestTheWireKeepsItsUnlimitedReading pinning it.
	MaxAgents int `json:"maxAgents"`
	MaxBots   int `json:"maxBots"`
	// UnlimitedAgents reports that MaxAgents 0 means "no ceiling" rather than
	// "no agents" — the reading a bare zero cannot carry.
	UnlimitedAgents bool `json:"unlimitedAgents"`
}

// capacity translates the catalog's published roster into the wire's reading.
//
// The catalog says three things and the wire has room for two, so this is where
// the difference is decided rather than at each reader:
//
//	(-1, true)   unlimited             -> 0, the wire's own "no ceiling"
//	(n>0, true)  a real bound          -> n
//	(_, false)   the catalog is silent -> 0, serve WITHOUT a bound
//
// The silent case is the important one and it admits, per the catalog's own rule:
// "a missing CAPACITY has no safe number. Zero would refuse a customer their
// first agent." So does a plan that includes NONE of a kind — (0, true) — which
// this wire cannot distinguish from unlimited; it is returned as 0 with the same
// admitting reading, because enforcing a bound no reader can tell apart from its
// opposite is worse than enforcing none.
func capacity(n int, known bool) int {
	if !known || n < 0 {
		return 0
	}
	return n
}

// tierLimits composes what a TIER allows with what the PLAN includes. One
// constructor, so a read and a check cannot compose them differently.
func tierLimits(cfg *tier.Config, slug string) TierLimits {
	agents := capacity(AgentsIncluded(slug))
	return TierLimits{
		Config:          *cfg,
		MaxAgents:       agents,
		MaxBots:         capacity(BotsIncluded(slug)),
		UnlimitedAgents: agents == 0,
	}
}

// TierBalance is what the subject can actually spend, from the same three-bucket
// split the balance endpoint serves.
//
// EffectiveAvailable is the only figure a gate should compare against zero. The
// others are its parts, and prepaid + credits + daily are three sources of one
// spend, not three balances to add up a second time.
type TierBalance struct {
	// Currency is the ISO code the figures are denominated in.
	Currency currency.Type `json:"currency"`
	// PrepaidAvailable is real money on the ledger, less holds, floored at zero.
	PrepaidAvailable currency.Cents `json:"prepaidAvailable"`
	// CreditsRemaining is granted, still-active, non-cash credit — spendable on
	// metered usage, never on GPUs.
	CreditsRemaining currency.Cents `json:"creditsRemaining"`
	// DailyRemaining is what is left of the tier's daily allowance today. It is 0
	// for every registered tier; see ReadTier.
	DailyRemaining int64 `json:"dailyRemaining"`
	// EffectiveAvailable is spendable balance plus the daily term — what the gate
	// in front of the models reads.
	EffectiveAvailable int64 `json:"effectiveAvailable"`
}

// TierView is a subject's tier as a TYPED value: which tier they are on, what it
// allows, what they can spend, and how much of each plan window is left.
type TierView struct {
	// User is the billing subject this answers for.
	User string `json:"user"`
	// Tier is the tier's own bounds.
	Tier TierLimits `json:"tier"`
	// Balance is what they can spend right now.
	Balance TierBalance `json:"balance"`
	// Windows are the plan's own bounds, over four nested spans. A plan sells
	// usage over them and nothing on the request path could see it: they were
	// published in the catalog and reported to the account page, and the gate had
	// no way to ask. They are CARRIED here, not enforced — enforcing is the
	// router's call and a refusal is money. A window with limit 0 declares no
	// bound at that span, so a reader skips it rather than treating it as spent.
	Windows []Window `json:"windows"`
}

// errTierNoOrg is ReadTier REFUSING the question rather than failing at it: with
// no tenant named there is no ledger to read, and a tier answered from nothing
// would be a tier invented.
var errTierNoOrg = errors.New("tier: no organization")

// IsTierRefusal reports whether ReadTier refused the question — no tenant named —
// as against failing to read one that exists.
//
// The two must stay apart at every caller, which is why the class is exported
// rather than left as a sentinel only this package can see. The router in front
// of the models reads ANY non-2xx from the tier endpoint as the free tier, so a
// caller that cannot tell a question it malformed (fix the call) from a ledger it
// could not read (retry, or hold the last-known tier) pins a paying customer to
// the most restrictive tier in the table, silently. The endpoint maps the first to
// 400 and the second to 500; a peer on the internal plane splits them here.
func IsTierRefusal(err error) bool { return errors.Is(err, errTierNoOrg) }

// ReadTier is a subject's tier and what it leaves them able to spend — the
// QUESTION, with no HTTP in it.
//
// It takes values rather than a request so a caller that is not a request can
// ask: the rate limiter in front of the models asks this of every call over the
// internal plane, holding no ledger of its own, and re-deriving it there would be
// a second answer to "may this subject spend" — which is the one question that
// must have exactly one.
//
// The tier NAME is passed in rather than resolved here. An override is a mint and
// only a minter may name one (resolveTierName), which is a fact about the caller's
// credential, not about the subject.
//
// Its single failure is the ledger read. The daily term and the windows report
// what they can and stay quiet otherwise, exactly as they do on the wire.
func ReadTier(ctx context.Context, org *organization.Organization, user string, name tier.Name) (*TierView, error) {
	if org == nil {
		return nil, errTierNoOrg
	}
	ctx = org.Namespaced(ctx)
	cfg := tier.Get(name)

	// Spendable balance from the SAME three-bucket split the balance endpoint
	// serves — prepaid real money AND still-active granted credits both spend
	// (credits-first). A bespoke prepaid-only read silently zeroes accounts funded
	// purely by a starter/promo grant, 402-gating orgs that hold real spendable
	// credit.
	cur := currency.Type("usd")
	split, err := bucketedSplit(ctx, user, cur, org.TestMode())
	if err != nil {
		return nil, err
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

	// ONE slug resolution, for both the usage windows and the plan's roster. It was
	// already resolved for the windows; reading it twice is two chances to answer
	// for two different plans in one payload.
	slug := subscriptionPlanSlug(datastore.New(ctx), user)

	return &TierView{
		User: user,
		Tier: tierLimits(cfg, slug),
		Balance: TierBalance{
			Currency:           cur,
			PrepaidAvailable:   prepaidAvailable,
			CreditsRemaining:   split.CreditsRemaining,
			DailyRemaining:     dailyRemaining,
			EffectiveAvailable: int64(spendable) + dailyRemaining,
		},
		Windows: usageWindows(ctx, user, slug, org.TestMode(), time.Now()),
	}, nil
}

// TierOf is the tier a subject's own subscriptions confer — the DERIVATION, with
// no request in it.
//
// It is deliberately the store half only. The two ways a caller can NAME a tier
// rather than earn one — an X-Tier header, an explicit ?tier= — are request
// facts and are a MINT, admitted only for a caller that may mint; that decision
// belongs at the endpoint that can see the credential, and a core that read them
// would be honouring a claim nobody proved.
//
// Fail-safe: a lookup error is RETURNED rather than answered as Free, so a
// transient store error can never strip a paid subscriber's tier.
func TierOf(ctx context.Context, org *organization.Organization, user string) (tier.Name, error) {
	if org == nil {
		// No org means no store to reach, so there is genuinely no subscription
		// in view. Free is the answer, not an error.
		return tier.Free, nil
	}
	return deriveTier(datastore.New(org.Namespaced(ctx)), user)
}

// GetTier is the endpoint over ReadTier.
//
// For IAM-authenticated requests the tier is read from the JWT claim.
// For service-to-service calls the tier may be passed as a query parameter.
//
//	GET /v1/billing/tier?user=hanzo/alice
//
// There is no free tier: a zero-balance account has effectiveAvailable == 0
// and is gated. Onboarding funds an account once via the starter-credit grant,
// and once that is spent the account is gated until it is topped up.
//
// Every status here is load-bearing. The router in front of the models reads
// this on every request and maps ANY non-2xx to Free, so a refusal it did not
// expect downgrades a paying customer in silence — which is why the two
// unanswerable cases refuse loudly rather than answering Free.
func GetTier(c *zip.Ctx) error {
	// No organization on the request is a REFUSAL, not a panic and not a Free.
	//
	// The one-value GetOrganization type-asserts and blew up here for every
	// service-to-service caller — measured live, a 500 on every call. That is
	// this route's main caller: IAMTokenRequired resolves an org only from a
	// gateway-validated user identity and deliberately falls through on a bare
	// X-Org-Id, so an S2S request reaches the handler with a nil org.
	//
	// Answering Free instead would be worse than the 500. Free is exactly what
	// the router falls back to on error, so serving it from a request that could
	// read nothing turns an unanswerable question into a confident wrong answer:
	// every paying customer silently pinned to the most restrictive tier, with no
	// error anywhere to find. A tier that cannot be read is reported as not read.
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return http.Fail(c, 400, "organization is required to read a tier", nil)
	}

	user := strings.ToLower(strings.TrimSpace(c.Query("user")))
	if user == "" {
		return http.Fail(c, 400, "user query parameter is required", nil)
	}

	// Resolved HERE because it reads the caller's own credential, not the
	// subject: an override is a mint and only a minter may name one.
	tierName, err := resolveTierName(c, user)
	if err != nil {
		// Fail-safe: a subscription-store hiccup must NOT downgrade a paid
		// subscriber to Free. Surface the error so the caller retries or holds
		// the last-known tier instead of asserting a wrong Free.
		return http.Fail(c, 500, "failed to resolve tier", err)
	}

	view, err := ReadTier(c.Context(), org, user, tierName)
	if err != nil {
		return http.Fail(c, 500, "failed to query balance", err)
	}
	return c.JSON(200, view)
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

	// The SAME composition ReadTier answers with, so a check and a read cannot
	// report a different roster for one customer. A missing org means no store to
	// reach and so no subscription in view — the slug is empty, the catalog is
	// silent, and `capacity` serves without a bound rather than refusing on nothing.
	slug := ""
	if org, ok := middleware.GetOrganizationOK(c); ok && org != nil {
		slug = subscriptionPlanSlug(datastore.New(org.Namespaced(c.Context())), user)
	}
	lim := tierLimits(cfg, slug)
	resp := map[string]any{
		"user": user,
		"tier": map[string]any{
			"name":          cfg.Name,
			"displayName":   cfg.DisplayName,
			"allowedModels": cfg.AllowedModels,
			"maxAgents":     lim.MaxAgents,
			"maxBots":       lim.MaxBots,
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
	// AN OVERRIDE IS A MINT, AND ONLY A MINTER MAY USE ONE.
	//
	// Both of these are CLIENT INPUT. `?tier=` is obviously so; X-Tier is too —
	// the gateway neither mints it (iamauth.MintedIdentityHeaders) nor strips it
	// (StripIdentityHeaderNames), so despite the comment calling it authoritative
	// it arrives from whoever sent the request. Honouring either unconditionally
	// let any caller name its own tier: measured live, `X-Tier: enterprise` on a
	// free subject returned enterprise with unlimitedAgents, and `?tier=max`
	// returned Pro with allowedModels ["*"].
	//
	// A tier decides which models a caller may invoke and how many agents it may
	// run, so granting one is minting. The clamp is the SAME predicate the sibling
	// resolver already applies to the same class of client string —
	// planForGrant (allotment.go) honours an explicit plan only for
	// middleware.MayMintMoney — so there is one rule for "may this caller name its
	// own entitlement", in one place, rather than three resolvers disagreeing.
	//
	// An unprivileged override is IGNORED, not refused, exactly as planForGrant
	// ignores one: the caller falls through to the tier its subscriptions actually
	// confer. Refusing would break readers that pass a hint they are not entitled
	// to, for no gain — the answer they get is simply the true one.
	//
	// The S2S readers are unaffected: ai's rate limiter and apps/metering send
	// ?user= and a service token, never ?tier=, and a service token satisfies
	// MayMintMoney anyway.
	if override := firstOverride(iammiddleware.GetIAMTier(c), c.Query("tier")); override != "" {
		if middleware.MayMintMoney(c) {
			return tierOfName(override), nil
		}
	}
	// The derivation itself is TierOf, so the endpoint and a peer asking by name read
	// one implementation. A missing org (which should not happen under the
	// billing group) is Free there for the reason it was Free here: with no store
	// to reach there is genuinely no subscription in view.
	org, _ := middleware.GetOrganizationOK(c)
	return TierOf(c.Context(), org, user)
}

// firstOverride returns the first non-empty, trimmed tier override. Both sources
// are client input; which one arrived does not change how much it is trusted.
func firstOverride(vals ...string) string {
	for _, v := range vals {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}

// tierOfName resolves a name that may be EITHER a tier or a CATALOG PLAN SLUG.
//
// Two vocabularies meet here and only one was being read. The registry holds tier
// names — free, starter, pro, enterprise. The catalog SELLS go, dev, pro, max, team
// and enterprise. `tier.Parse` knows only the first, so four of the six sold plans
// fell through to Free, which is the most restrictive configuration there is: one
// agent and two models. Measured live before this change:
//
//	?tier=go   -> Free   ?tier=dev  -> Free
//	?tier=max  -> Free   ?tier=team -> Free      (max is $99/mo)
//
// A plan name is not a tier name, and the fix is not a second hardcoded list: the
// CATALOG is the authority on what is sold, and it already carries the category
// deriveTier keys on. Resolving through it means a plan confers the same tier
// whether it arrives as a subscription row or as a name in a claim — and a plan
// added to the catalog tomorrow is covered without touching this function.
//
// Only a string that is neither a tier nor a sold plan is Free.
func tierOfName(raw string) tier.Name {
	if n, ok := tier.ParseOK(raw); ok {
		return n
	}
	p := lookupPlan(strings.ToLower(strings.TrimSpace(raw)))
	if p == nil {
		return tier.Free
	}
	// Same two rules deriveTier applies to a subscription, so the two paths cannot
	// disagree about what a plan is worth.
	if p.Category == "enterprise" || p.ContactSales {
		return tier.Enterprise
	}
	if paidTier(p.Slug) {
		return tier.Pro
	}
	return tier.Free
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
