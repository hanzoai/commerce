package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	types "github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/json/http"
)

type createSubscriptionRequest struct {
	UserId               string `json:"userId"`
	PlanId               string `json:"planId"`
	StoreId              string `json:"storeId,omitempty"`
	DefaultPaymentMethod string `json:"defaultPaymentMethod"`
	// Quantity is the billable seat count for per-seat plans (catalog
	// price_ref.recurring.per_seat); defaults to 1 and must meet the catalog's
	// limits.minSeats. Ignored as a multiplier on flat plans.
	Quantity int `json:"quantity"`
	// Level picks which of the plan's published prices to open at — an INDEX into
	// the catalog's `prices`, never an amount. 0 (the default) is the plan's base
	// price. See plan.LevelPrice for why the wire carries a choice and not a sum.
	Level int `json:"level,omitempty"`
	// Members are the seat holders of a per-seat subscription. Each gets a
	// zero-price bundle-child subscription row so the monthly allotment run
	// grants their own per-user included credit. Never more than Quantity.
	Members  []string               `json:"members"`
	Metadata map[string]interface{} `json:"metadata"`
}

type updateSubscriptionRequest struct {
	PlanId   string   `json:"planId"`
	Prorate  bool     `json:"prorate"`
	Quantity int      `json:"quantity"`
	Members  []string `json:"members"` // see createSubscriptionRequest.Members
}

type cancelSubscriptionRequest struct {
	AtPeriodEnd bool `json:"atPeriodEnd"`
}

// CreateBillingSubscription creates a new subscription and starts the billing lifecycle.
//
//	POST /v1/billing/subscriptions
func CreateBillingSubscription(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	var req createSubscriptionRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	if req.UserId == "" {
		return http.Fail(c, 400, "userId is required", nil)
	}

	if req.PlanId == "" {
		return http.Fail(c, 400, "planId is required", nil)
	}

	p, err := resolveSubscriptionPlan(db, req.PlanId)
	if err != nil {
		return http.Fail(c, 404, "plan not found", err)
	}
	if !p.Listed() {
		// Retired/draft tier: it still resolves (renewals need it) but is not on
		// sale — see plan.Listed. Same 404 as a slug that never existed, so the
		// refusal tells a prober nothing about which slugs are real.
		return http.Fail(c, 404, "plan not found", nil)
	}

	// The price this subscription opens at, chosen from what the catalog
	// publishes. A level the plan does not publish is refused outright.
	p, err = planAtLevel(p, req.Level)
	if err != nil {
		return http.Fail(c, 400, err.Error(), nil)
	}

	// C1-a: a PAID-tier subscription confers a spendable entitlement — its
	// included monthly allotment is MINTED onto the user's gateway balance — yet
	// this endpoint starts it Active instantly with ZERO payment
	// (ProviderType="internal", no CollectInvoice). An org-level admin must NOT be
	// able to self-mint a paid tier ("max" → $100/mo, recurring) by naming it
	// here; only a proven mint principal (internal service token / platform global
	// admin — e.g. cloud-api after a real payment) may create a paid-tier
	// subscription. A FREE ($0) tier stays self-serve even when it carries a small
	// included credit as a perk (developer's $5/mo) — price is the paid-tier gate,
	// not the allotment. Pairs with the payment-backed clamp in subscriptionPlanSlug.
	//
	// The card path (POST /v1/billing/subscribe/card) bypasses THIS gate legitimately:
	// it calls createSubscription directly AFTER a real card charge settles — the
	// settled payment IS the mint authority (mirrors topup_token's
	// mintauth.WithAuthorized rationale). So the gate lives HERE, in the
	// zero-payment HTTP entrypoint, not in the shared core.
	// Gate on the IMMUTABLE embed paidTier of the RESOLVED slug (p.Slug), NOT the
	// editable DB p.Price and NOT the raw req.PlanId (Red F2 + R1). Plan pricing is
	// now admin-editable (increment 3a) and the minted allotment AMOUNT
	// (IncludedMonthlyCents) is embed-derived, so reading p.Price would let a
	// SuperAdmin lowering an embed-paid plan's DB Price to 0 skip C1-a. And because
	// resolveSubscriptionPlan→GetById also accepts a plan's DB HASHID, scoring the
	// raw req.PlanId (a hashid → paidTier false) would skip the gate while p is the
	// real paid plan — so score p.Slug, the canonical slug both id forms resolve to.
	// paidTier reads the same embed authority as the allotment, so they never desync.
	if paidTier(p.Slug) && !middleware.MayMintMoney(c) {
		return http.Fail(c, 403,
			"creating a paid-tier subscription requires platform-administrator or internal-service credentials", nil)
	}

	sub, err := createSubscription(db, p, &req)
	if err != nil {
		return subscriptionCreateError(c, err)
	}

	emitSubscriptionCreated(c, org.Name, sub)

	return c.JSON(201, subscriptionResponse(sub))
}

// subValidationError marks a client-side (HTTP 400) subscription failure — a
// seat-floor or members>seats violation on the way in, a lifecycle move the
// engine refuses once the row exists — as distinct from an internal (500)
// failure, so a core can refuse the caller's values without knowing what a
// status code is.
type subValidationError struct{ msg string }

func (e subValidationError) Error() string { return e.msg }

// errSubscriptionNotFound is the miss: no such subscription in the caller's org.
// The namespace IS the boundary, so a row belonging to another tenant is simply
// not found — there is no window in which the wrong row is in hand.
var errSubscriptionNotFound = errors.New("subscription: no such subscription for this org")

// IsSubscriptionNotFound reports whether err is that miss. A caller outside this
// package has to answer it the same way the door does, and an unexported
// sentinel cannot be asked about.
func IsSubscriptionNotFound(err error) bool { return errors.Is(err, errSubscriptionNotFound) }

// IsSubscriptionRefused reports whether err is a refusal of what the CALLER
// asked for — seats below the plan's floor, members beyond the seats paid for,
// a lifecycle move the engine will not make — as distinct from the store
// failing. The asker can fix the first and can do nothing about the second,
// which is why they carry different statuses, and why a caller that could not
// tell them apart would report every typo as an outage.
func IsSubscriptionRefused(err error) bool {
	var ve subValidationError
	return errors.As(err, &ve)
}

// subscriptionCreateError maps a createSubscription error to the right HTTP status:
// a seat/members validation error is a 400, anything else (persist failure) a 500.
func subscriptionCreateError(c *zip.Ctx, err error) error {
	if IsSubscriptionRefused(err) {
		return http.Fail(c, 400, err.Error(), nil)
	}
	return http.Fail(c, 500, "failed to create subscription", err)
}

// resolveSubscriptionPlan resolves a plan by slug — the DB row when present, else
// the static catalog (seeded into the DB best-effort). The returned plan carries
// the SERVER-AUTHORITATIVE price; a client-supplied amount is never consulted. It
// is the single plan-resolution path shared by CreateBillingSubscription and the
// card subscribe endpoint.
func resolveSubscriptionPlan(db *datastore.Datastore, planId string) (*plan.Plan, error) {
	// The plan authority is platform-global (models/plan, "system" ns) — the SAME
	// rows admin.hanzo.ai edits and GET /v1/billing/plans serves — so an admin
	// price edit flows straight into this charge. Read the authority, NOT the
	// caller's per-org db. The embed is the fallback (seed did not run / a
	// never-seeded slug), and the row is lazily materialized in the authority ns
	// so its Id stays stable. The returned Price is the SERVER-AUTHORITATIVE
	// amount; a client-supplied value is never consulted.
	pdb := plan.AuthorityDB(db.Context)
	p := plan.New(pdb)
	if err := p.GetById(planId); err != nil {
		var staticP *staticPlan
		for i := range catalog {
			if catalog[i].Slug == planId {
				staticP = &catalog[i]
				break
			}
		}
		if staticP == nil {
			return nil, err
		}
		p.Slug = staticP.Slug
		p.Name = staticP.Name
		p.Description = staticP.Description
		p.Category = staticP.Category
		p.Price = currency.Cents(staticP.Price)
		p.PriceAnnual = currency.Cents(staticP.PriceAnnual)
		p.Prices = centsOf(staticP.Prices)
		p.ContactSales = staticP.ContactSales
		p.Popular = staticP.Popular
		p.PerSeat = staticP.PerSeat
		p.Currency = currency.Type(staticP.Currency)
		p.Interval = types.Interval(staticP.Interval)
		p.IntervalCount = staticP.IntervalCount
		p.TrialPeriodDays = staticP.TrialPeriodDays
		_ = p.Create() // best-effort; ignore dup-key errors
	}
	return p, nil
}

// planAtLevel returns p priced at the level the purchase chose — a COPY, always,
// priced at exactly one of the prices the catalog publishes for that plan. An
// unpublished level is refused here, before any card is touched.
//
// It copies rather than repricing p because p is the AUTHORITY row: the same row
// admin.hanzo.ai edits, GET /v1/billing/plans serves, and every other
// subscription on this plan resolves through. A level is one customer's choice,
// so it belongs to one subscription, and a level written onto the shared row
// would be one persist away from repricing the tier for everyone.
//
// Where the choice then lives is what makes it survive: StartSubscription
// snapshots the plan it is handed onto sub.Plan, the snapshot persists with the
// subscription, and buildPeriodInvoice invoices sub.Plan.Price — so a
// subscription opened at a level renews at that level for its whole life,
// without the renewal path knowing levels exist.
func planAtLevel(p *plan.Plan, level int) (*plan.Plan, error) {
	price, err := p.LevelPrice(level)
	if err != nil {
		return nil, fmt.Errorf("plan %q is not sold at level %d: %w", p.Slug, level, err)
	}
	at := *p
	at.Price = price
	return &at, nil
}

// createSubscription is the reusable CORE of subscription creation: seat gate →
// create the row → expand bundles → provision member seats. It does NOT enforce
// the C1-a paid-tier mint gate — that is the caller's job (the HTTP
// CreateBillingSubscription gates it; the card path is authorized by its settled
// charge). p carries the server-authoritative price; req.DefaultPaymentMethod is
// set on the row (the card path passes the vaulted payment-method id).
//
// It takes the datastore and the priced plan and nothing else, so it is reachable
// from a core as well as from a door. The analytics emit belongs to the door that
// rendered the result — it reads the collector off the request — so each
// entrypoint fires its own once this hands back the row.
//
// p is ALREADY priced at the chosen level (planAtLevel), because the card path
// has to know the amount before it charges. So req.Level is the wire field, and
// p.Price is the answer — this core reads the price and never re-derives it.
func createSubscription(db *datastore.Datastore, p *plan.Plan, req *createSubscriptionRequest) (*subscription.Subscription, error) {
	// Seat gate: the catalog is the sole authority for per-seat-ness and the
	// seat floor. A per-seat plan bills Price × quantity, never below
	// limits.minSeats, and member rows (each minting a per-user allotment) can
	// never exceed the seats paid for.
	quantity := req.Quantity
	if quantity < 1 {
		quantity = 1
	}
	if perSeat(req.PlanId) {
		p.PerSeat = true
		if min := minSeats(req.PlanId); quantity < min {
			return nil, subValidationError{fmt.Sprintf("plan %q requires at least %d seats (got %d)", req.PlanId, min, quantity)}
		}
		if len(req.Members) > quantity {
			return nil, subValidationError{fmt.Sprintf("members (%d) exceed seats (%d)", len(req.Members), quantity)}
		}
	}

	// Create subscription
	sub := subscription.New(db)
	sub.UserId = req.UserId
	sub.StoreId = req.StoreId
	sub.DefaultPaymentMethod = req.DefaultPaymentMethod
	sub.ProviderType = "internal"
	sub.Quantity = quantity

	if req.Metadata != nil {
		sub.Metadata = req.Metadata
	}

	// Initialize subscription lifecycle
	engine.StartSubscription(sub, p)

	if err := sub.Create(); err != nil {
		log.Error("Failed to create subscription: %v", err)
		return nil, err
	}

	// Bundle expansion. Plans declare their bundle list in the
	// canonical catalog (subscription.json:bundles) — e.g. "pro"
	// includes "world-pro", "team" includes "world-team", etc. Mint a
	// child zero-cost subscription for each bundled slug so the
	// downstream feature gate (world.hanzo.ai, chat, etc.) sees the
	// entitlement without the customer having to pay twice. The child
	// subscriptions reference the parent via Metadata.bundleParent so
	// cancellation can cascade them.
	bundled := bundledPlansForSlug(req.PlanId)
	for _, childSlug := range bundled {
		// A bundle child is ALWAYS zero-cost (the parent's invoice carries the
		// charge) and must NEVER touch the plan authority: persisting a Price=0,
		// envelope-less partial row there under-charged a DIRECT sub to the child
		// plan (prod: world-pro/world-team served $0), and on a HIT it would have
		// snapshotted the child's FULL price and double-charged. Build a LOCAL
		// zero-price snapshot from the embed — the authority is owned only by the
		// seed + CRUD. Mirrors provisionMembers' in-memory child copy.
		childPlan := childSnapshotPlan(db, childSlug)
		if childPlan == nil {
			log.Error("bundled plan %q not found for parent %q (skipping)", childSlug, req.PlanId, nil)
			continue
		}

		childSub := subscription.New(db)
		childSub.UserId = sub.UserId
		childSub.DefaultPaymentMethod = sub.DefaultPaymentMethod
		childSub.ProviderType = "bundle"
		childSub.Quantity = 1
		childMeta := map[string]interface{}{
			"bundleParent":     sub.Id(),
			"bundleParentPlan": req.PlanId,
		}
		for k, v := range req.Metadata {
			childMeta[k] = v
		}
		childSub.Metadata = childMeta
		engine.StartSubscription(childSub, childPlan)
		childSub.PlanId = childSlug // stable label; the child plan is a local snapshot, not an authority row
		if err := childSub.Create(); err != nil {
			log.Error("Failed to create bundled subscription %q for parent %q: %v", childSlug, req.PlanId, err)
			continue
		}
	}

	// Seat → credits: one zero-price bundle-child row per member, so the
	// monthly allotment run grants each member their own per-user included
	// credit (never a multiplied org wallet).
	if perSeat(req.PlanId) {
		provisionMembers(db, sub, req.PlanId, p, req.Members)
	}

	return sub, nil
}

// bundledPlansForSlug returns the slugs of plans that ride along with
// a parent plan. Reads from the static catalog so we never need to hit
// the DB for plan metadata that is already baked into the binary.
func bundledPlansForSlug(slug string) []string {
	for i := range catalog {
		if catalog[i].Slug == slug {
			out := make([]string, len(catalog[i].Bundles))
			copy(out, catalog[i].Bundles)
			return out
		}
	}
	return nil
}

// childSnapshotPlan builds a ZERO-COST snapshot plan for a bundle child from the
// embed. It NEVER reads or writes the plan authority: a bundle child must not
// create a partial ($0, envelope-less) row there (Red: that under-charged a
// DIRECT sub to the child plan). Price is ALWAYS 0 — the parent's invoice carries
// the charge — so a child snapshot can never be invoiced. Returns nil if the slug
// is unknown to the embed.
func childSnapshotPlan(db *datastore.Datastore, childSlug string) *plan.Plan {
	var sc *staticPlan
	for i := range catalog {
		if catalog[i].Slug == childSlug {
			sc = &catalog[i]
			break
		}
	}
	if sc == nil {
		return nil
	}
	// Bound to db (so StartSubscription's .Id() is safe) but NEVER Create()d — the
	// child is an in-memory snapshot, exactly like provisionMembers' *base copy;
	// the authority is never written from the subscription flow.
	p := plan.New(db)
	p.Slug = sc.Slug
	p.Name = sc.Name
	p.Description = sc.Description
	p.Category = sc.Category
	p.Currency = currency.Type(sc.Currency)
	p.Interval = types.Interval(sc.Interval)
	p.IntervalCount = sc.IntervalCount
	p.Price = 0 // ALWAYS zero — a bundle child never charges
	return p
}

// provisionMembers mints one zero-price bundle-child subscription row per
// member of a per-seat subscription — the SAME machinery as plan bundles, so
// the monthly allotment run (grantOrgAllotments) grants each member their own
// per-user included credit off their own row. Idempotent per (parent, member):
// a live child row is never duplicated. The child plan is an in-memory
// zero-price copy of the parent's plan — the customer pays once, on the
// parent's per-seat invoice.
func provisionMembers(db *datastore.Datastore, parent *subscription.Subscription, planSlug string, base *plan.Plan, members []string) {
	seen := map[string]bool{parent.UserId: true}
	for _, m := range members {
		m = strings.ToLower(strings.TrimSpace(m))
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		if hasMemberSub(db, planSlug, m) {
			continue
		}
		childPlan := *base
		childPlan.Price = 0 // the member rides the parent's per-seat charge
		childSub := subscription.New(db)
		childSub.UserId = m
		childSub.DefaultPaymentMethod = parent.DefaultPaymentMethod
		childSub.ProviderType = "bundle"
		childSub.Quantity = 1
		childSub.Metadata = map[string]interface{}{
			"bundleParent":     parent.Id(),
			"bundleParentPlan": planSlug,
		}
		engine.StartSubscription(childSub, &childPlan)
		if err := childSub.Create(); err != nil {
			log.Error("Failed to create member subscription for %q (parent %q): %v", m, parent.Id(), err)
		}
	}
}

// hasMemberSub reports whether member already holds a live bundle-child row
// for the plan — the persisted match key (subscription Metadata does not
// survive the datastore round-trip). One per-user grant anchor per plan per
// org is exactly the allotment semantics.
func hasMemberSub(db *datastore.Datastore, planSlug, member string) bool {
	subs := make([]*subscription.Subscription, 0)
	if _, err := subscription.Query(db).Filter("UserId=", member).GetAll(&subs); err != nil {
		return false
	}
	for _, s := range subs {
		if s.ProviderType != "bundle" || s.Plan.Slug != planSlug {
			continue
		}
		switch s.Status {
		case subscription.Active, subscription.Trialing:
			return true
		}
	}
	return false
}

// billingSubscription returns the subject's live, payment-backed PAID
// subscription — the one a second paid sale would bill twice — or nil.
//
// ONE SUBSCRIPTION SPEAKS FOR A SUBJECT. That is not a rule invented here; it is
// what the platform already assumes. subscriptionPlanSlug reduces every live
// subscription a subject holds to a SINGLE governing plan by picking the newest
// PeriodStart, with no notion of a product line — so a second concurrent
// subscription does not merely charge twice, it SHADOWS the first: hold "max"
// ($100/mo included) and start "dns-pro" and the newer row answers "what plan is
// this?" with a tier whose IncludedMonthlyCents is 0. Selling a second one cannot
// be made correct without first giving that resolver a notion of which
// subscription governs what, so this refuses the sale instead of corrupting the
// entitlement.
//
// Two exclusions, each for its own reason, and neither is the other's:
//
//	FREE tiers do not count. Nothing bills, so nothing double-bills, and a free
//	row must never block the customer who is finally paying — free → paid is the
//	main funnel.
//	NOT-payment-backed paid rows do not count either. A zero-payment internal row
//	collects nothing, so it cannot double-charge, and counting it would trap the
//	holder of a forged Active row outside any door that could fix it.
//
// It answers a different question from subscriptionPlanSlug ("what may this
// subject's allotment anchor on") and shares its filters by coincidence of
// meaning, not by being the same predicate; keeping them apart is what lets FREE
// anchor an allotment while not blocking a sale.
func billingSubscription(db *datastore.Datastore, subject string) *subscription.Subscription {
	subs, err := userSubscriptions(db, subject)
	if err != nil {
		// Fail OPEN on a read failure: refusing a legitimate first purchase because
		// the subscription store hiccuped costs a customer, while the worst case of
		// proceeding is the duplicate this exists to prevent — which the stable
		// Square idempotency key and the guard still bound.
		return nil
	}
	for _, s := range subs {
		switch s.Status {
		case subscription.Active, subscription.Trialing:
		default:
			continue
		}
		// A bundle child is an entitlement row the parent minted, not a purchase.
		if strings.EqualFold(strings.TrimSpace(s.ProviderType), "bundle") {
			continue
		}
		slug := s.Plan.Slug
		if slug == "" {
			slug = s.PlanId
		}
		if paidTier(slug) && subscriptionPaymentBacked(s) {
			return s
		}
	}
	return nil
}

// ListSubscriptions is the org's subscriptions, filtered — the QUERY, with no
// HTTP in it.
//
// It takes values rather than a request so a caller that is not a request can
// ask: the same list is read over the internal plane by a peer process that
// holds no ledger of its own, and re-deriving this query there would be a second
// implementation of one question. Two copies of a subscription filter is how a
// billing page and a plan gate come to disagree about who is subscribed.
//
// Empty userID or status means "do not filter on it", which is what an absent
// query parameter has always meant here.
//
// It returns the ROWS, not a rendered view: the HTTP handler builds its own
// envelope from these, and a peer builds its own. Returning the envelope would
// put a renderer next to the datastore and make every future caller inherit it.
func ListSubscriptions(ctx context.Context, org *organization.Organization, userID, status string) ([]*subscription.Subscription, error) {
	if org == nil {
		return nil, errors.New("subscriptions: no organization")
	}
	db := datastore.New(org.Namespaced(ctx))
	q := subscription.Query(db).Ancestor(db.NewKey("synckey", "", 1, nil))
	if userID != "" {
		q = q.Filter("UserId=", userID)
	}
	if status != "" {
		q = q.Filter("Status=", status)
	}
	subs := make([]*subscription.Subscription, 0)
	if _, err := q.GetAll(&subs); err != nil {
		return nil, err
	}
	return subs, nil
}

// ListBillingSubscriptions lists subscriptions for a user.
//
//	GET /v1/billing/subscriptions?userId=...
func ListBillingSubscriptions(c *zip.Ctx) error {
	// #146 class: never panic on a missing org (co-resident embed path — see ListInvoices).
	// No org ⇒ honest empty.
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return c.JSON(200, map[string]any{"subscriptions": []map[string]any{}, "count": 0})
	}
	subs, err := ListSubscriptions(c.Context(), org, strings.TrimSpace(c.Query("userId")), strings.TrimSpace(c.Query("status")))
	if err != nil {
		log.Error("Failed to list subscriptions: %v", err, c)
		return http.Fail(c, 500, "failed to list subscriptions", err)
	}

	items := make([]map[string]any, 0, len(subs))
	for _, s := range subs {
		items = append(items, subscriptionResponse(s))
	}

	return c.JSON(200, map[string]any{
		"subscriptions": items,
		"count":         len(items),
	})
}

// GetBillingSubscription returns a single subscription.
//
//	GET /v1/billing/subscriptions/:id
func GetBillingSubscription(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	id := c.Param("id")
	sub := subscription.New(db)
	if err := sub.GetById(id); err != nil {
		return http.Fail(c, 404, "subscription not found", err)
	}

	return c.JSON(200, subscriptionResponse(sub))
}

// UpdateBillingSubscription updates a subscription (plan change, quantity).
//
//	PATCH /v1/billing/subscriptions/:id
func UpdateBillingSubscription(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))

	id := c.Param("id")
	sub := subscription.New(db)
	if err := sub.GetById(id); err != nil {
		return http.Fail(c, 404, "subscription not found", err)
	}

	var req updateSubscriptionRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}

	planChanged := req.PlanId != "" && req.PlanId != sub.PlanId

	// Seat gate: the EFFECTIVE plan + quantity after this update must satisfy
	// the catalog's per-seat floor, and member rows never exceed seats paid for.
	slug := req.PlanId
	if !planChanged {
		if slug = sub.Plan.Slug; slug == "" {
			slug = sub.PlanId
		}
	}
	quantity := sub.Quantity
	if req.Quantity > 0 {
		quantity = req.Quantity
	}
	if quantity < 1 {
		quantity = 1
	}
	if perSeat(slug) {
		if min := minSeats(slug); quantity < min {
			return http.Fail(c, 400,
				fmt.Sprintf("plan %q requires at least %d seats (got %d)", slug, min, quantity), nil)
		}
		if len(req.Members) > quantity {
			return http.Fail(c, 400,
				fmt.Sprintf("members (%d) exceed seats (%d)", len(req.Members), quantity), nil)
		}
	}

	if planChanged {
		// Resolve the target plan FIRST so the gate scores its canonical SLUG. A
		// client may pass a plan's DB HASHID (resolveSubscriptionPlan→GetById accepts
		// the ByKey path), which the slug-only IncludedMonthlyCents/paidTier would
		// score as 0/false — bypassing the gate while ChangePlan applies the real
		// high-allotment plan (Red R1). Resolving first collapses both id forms to the
		// slug. This reads/lazily-materializes only the PLAN authority row; it does
		// NOT touch the subscription or mint anything, so nothing anchors before the
		// 403 below. It also reads the admin-edited price + inherits the embed
		// fallback — never a bare per-org-ns miss.
		newPlan, err := resolveSubscriptionPlan(db, req.PlanId)
		if err != nil {
			return http.Fail(c, 404, "new plan not found", err)
		}
		if !newPlan.Listed() {
			// You may keep a retired tier you are already on; you may not MOVE onto
			// one. Without this, archiving a plan leaves it purchasable by PATCH.
			return http.Fail(c, 404, "new plan not found", nil)
		}

		// C1-a mint gate, PATCH parity (Red F1): CreateBillingSubscription gates a
		// paid-tier START, but a plan CHANGE had NO gate — and engine.ChangePlan
		// swaps sub.Plan for free (its proration line item is discarded, never
		// charged). So a cheap payment-backed sub could be laundered into a higher
		// tier's spendable allotment (subscribe dns-pro via card → PATCH to "max" →
		// allotment/grant mints ~$100). Gate a move that INCREASES the included,
		// spendable allotment on MayMintMoney. Score the IMMUTABLE embed allotment by
		// the RESOLVED slug (never the raw id, never the editable DB Price), so
		// neither a hashid nor an admin price edit can move the gate; a
		// lateral/downgrade stays self-serve.
		curSlug := sub.Plan.Slug
		if curSlug == "" {
			curSlug = sub.PlanId
		}
		if IncludedMonthlyCents(newPlan.Slug) > IncludedMonthlyCents(curSlug) && !middleware.MayMintMoney(c) {
			return http.Fail(c, 403,
				"increasing a subscription's included allotment requires platform-administrator or internal-service credentials", nil)
		}

		if perSeat(slug) {
			newPlan.PerSeat = true
		}

		if _, err := engine.ChangePlan(sub, newPlan, req.Prorate); err != nil {
			return http.Fail(c, 400, err.Error(), nil)
		}
	}

	if req.Quantity > 0 {
		sub.Quantity = req.Quantity
	}

	if err := sub.Update(); err != nil {
		log.Error("Failed to update subscription: %v", err, c)
		return http.Fail(c, 500, "failed to update subscription", err)
	}

	// Seat → credits on seat changes: same per-member machinery as create.
	if perSeat(slug) && len(req.Members) > 0 {
		provisionMembers(db, sub, slug, &sub.Plan, req.Members)
	}

	if planChanged {
		emitSubscriptionPlanChanged(c, org.Name, sub)
	}

	return c.JSON(200, subscriptionResponse(sub))
}

// Subscription is one subscription as this surface reports it — the same facts
// subscriptionResponse renders for the browser, carried with a schema so a
// caller holding types instead of a response writer gets the same answer. One
// row, two projections; never two sets of facts.
type Subscription struct {
	ID       string `json:"id"`
	UserID   string `json:"userId"`
	PlanID   string `json:"planId"`
	Status   string `json:"status"`
	Quantity int    `json:"quantity"`

	CurrentPeriodStart time.Time `json:"currentPeriodStart"`
	CurrentPeriodEnd   time.Time `json:"currentPeriodEnd"`
	CancelAtPeriodEnd  bool      `json:"cancelAtPeriodEnd"`

	// MRRCents is what this subscription contributes to monthly recurring
	// revenue, computed here so no reader re-derives it from price and interval.
	MRRCents             int64  `json:"mrrCents"`
	ProviderType         string `json:"providerType"`
	DefaultPaymentMethod string `json:"defaultPaymentMethod"`

	Plan SubscriptionPlan `json:"plan"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// The four dates a subscription may not have. Each is a POINTER because the
	// key is absent on a CONDITION — a trial was opened, a cancel happened, the
	// row ended — and NOT because the value is zero: omitempty omits no
	// time.Time, so a plain field would report every subscription as trialing
	// since year one. Trial start and end appear together or not at all, which
	// is what "it had a trial" means.
	TrialStart *time.Time `json:"trialStart,omitempty"`
	TrialEnd   *time.Time `json:"trialEnd,omitempty"`
	CanceledAt *time.Time `json:"canceledAt,omitempty"`
	EndedAt    *time.Time `json:"endedAt,omitempty"`
}

// SubscriptionPlan is the plan AS THIS SUBSCRIPTION SNAPSHOTTED IT: the priced
// thing the customer bought, which is not necessarily what the catalog sells
// today. It is a different type from PlanView (the catalog's sales shape)
// because renewals invoice off this snapshot — the two are meant to be free to
// disagree, and one shared type would quietly make every price edit retroactive.
type SubscriptionPlan struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Price    int64  `json:"price"`
	Currency string `json:"currency"`
	Interval string `json:"interval"`
}

// viewSubscription projects the row onto the typed answer. ONE projection, so
// two cores cannot describe the same subscription two ways.
func viewSubscription(sub *subscription.Subscription) *Subscription {
	v := &Subscription{
		ID:                   sub.Id(),
		UserID:               sub.UserId,
		PlanID:               sub.PlanId,
		Status:               string(sub.Status),
		Quantity:             sub.Quantity,
		CurrentPeriodStart:   sub.PeriodStart,
		CurrentPeriodEnd:     sub.PeriodEnd,
		CancelAtPeriodEnd:    sub.EndCancel,
		MRRCents:             SubscriptionMRRCents(sub),
		ProviderType:         sub.ProviderType,
		DefaultPaymentMethod: sub.DefaultPaymentMethod,
		Plan: SubscriptionPlan{
			ID:       sub.Plan.Id(),
			Name:     sub.Plan.Name,
			Price:    int64(sub.Plan.Price),
			Currency: string(sub.Plan.Currency),
			Interval: string(sub.Plan.Interval),
		},
		CreatedAt: sub.CreatedAt,
		UpdatedAt: sub.UpdatedAt,
	}
	if !sub.TrialStart.IsZero() {
		start, end := sub.TrialStart, sub.TrialEnd
		v.TrialStart, v.TrialEnd = &start, &end
	}
	if !sub.CanceledAt.IsZero() {
		at := sub.CanceledAt
		v.CanceledAt = &at
	}
	if !sub.Ended.IsZero() {
		at := sub.Ended
		v.EndedAt = &at
	}
	return v
}

// loadSubscription reads one subscription inside the org's own namespace, and
// answers errSubscriptionNotFound when there is none. The namespace does the
// scoping by construction: an id from another org is not found rather than found
// and then filtered, so there is no window in which the wrong row is in hand.
func loadSubscription(ctx context.Context, org *organization.Organization, id string) (*subscription.Subscription, error) {
	if org == nil {
		return nil, errors.New("subscriptions: no organization")
	}
	sub := subscription.New(datastore.New(org.Namespaced(ctx)))
	if err := sub.GetById(id); err != nil {
		return nil, fmt.Errorf("%w: %v", errSubscriptionNotFound, err)
	}
	return sub, nil
}

// CancelSubscription ends a subscription — at the end of the period already paid
// for, or immediately — and answers with the row as it now stands.
//
// It takes values rather than a request because the customer who cancels is not
// always at a browser: the same act arrives over the internal plane from a peer
// that holds no ledger of its own, and a second implementation of "cancel" is
// how a billing page and a renewal come to disagree about who is still paying.
//
// The lifecycle belongs to the engine, so a move it will not make comes back as
// the caller's own refusal (IsSubscriptionRefused) rather than being re-checked
// here: one state machine, in one place.
func CancelSubscription(ctx context.Context, org *organization.Organization, id string, atPeriodEnd bool) (*Subscription, error) {
	sub, err := cancelSubscription(ctx, org, id, atPeriodEnd)
	if err != nil {
		return nil, err
	}
	return viewSubscription(sub), nil
}

// cancelSubscription is the worker: it returns the ROW, so the HTTP door renders
// its long-standing subscriptionResponse body and fires its analytics event off
// the model, while the typed caller reads Subscription. Two projections of one
// result, never two implementations.
//
// The emit stays at the door on purpose — it reads the collector off the
// request, and nothing that needs a request belongs in here.
func cancelSubscription(ctx context.Context, org *organization.Organization, id string, atPeriodEnd bool) (*subscription.Subscription, error) {
	sub, err := loadSubscription(ctx, org, id)
	if err != nil {
		return nil, err
	}
	if err := engine.CancelSubscription(sub, atPeriodEnd); err != nil {
		return nil, subValidationError{err.Error()}
	}
	if err := sub.Update(); err != nil {
		return nil, err
	}
	return sub, nil
}

// ReactivateSubscription puts a canceled subscription back on its plan and
// answers with the row as it now stands. Values rather than a request for the
// reason CancelSubscription gives, and with more force: reactivation is the act
// least likely to come from a browser, since what asks for it is usually a
// recovered payment method or a support tool.
func ReactivateSubscription(ctx context.Context, org *organization.Organization, id string) (*Subscription, error) {
	sub, err := reactivateSubscription(ctx, org, id)
	if err != nil {
		return nil, err
	}
	return viewSubscription(sub), nil
}

// reactivateSubscription is the worker. See cancelSubscription.
func reactivateSubscription(ctx context.Context, org *organization.Organization, id string) (*subscription.Subscription, error) {
	sub, err := loadSubscription(ctx, org, id)
	if err != nil {
		return nil, err
	}
	if err := engine.ReactivateSubscription(sub); err != nil {
		return nil, subValidationError{err.Error()}
	}
	if err := sub.Update(); err != nil {
		return nil, err
	}
	return sub, nil
}

// CancelBillingSubscription cancels a subscription.
//
//	POST /v1/billing/subscriptions/:id/cancel
func CancelBillingSubscription(c *zip.Ctx) error {
	// The OK form: IAMTokenRequired falls through WITHOUT setting the
	// "organization" local when the gateway named no principal, and the MustGet
	// form panics there — a 500 with no body, after the money has moved. Refuse
	// before touching anything. See SubscribeWithCard, where this cost a $99
	// charge with no subscription behind it.
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return http.Fail(c, 401, "sign in to change your subscription", nil)
	}
	var req cancelSubscriptionRequest
	if err := c.Bind(&req); err != nil {
		// Default to cancel at period end
		req.AtPeriodEnd = true
	}

	sub, err := cancelSubscription(c.Context(), org, c.Param("id"), req.AtPeriodEnd)
	if err != nil {
		switch {
		case IsSubscriptionNotFound(err):
			return http.Fail(c, 404, "subscription not found", err)
		case IsSubscriptionRefused(err):
			return http.Fail(c, 400, err.Error(), nil)
		}
		log.Error("Failed to cancel subscription: %v", err, c)
		return http.Fail(c, 500, "failed to cancel subscription", err)
	}

	emitSubscriptionCanceled(c, org.Name, sub)

	return c.JSON(200, subscriptionResponse(sub))
}

// ReactivateBillingSubscription reactivates a canceled subscription.
//
//	POST /v1/billing/subscriptions/:id/reactivate
func ReactivateBillingSubscription(c *zip.Ctx) error {
	// The OK form: IAMTokenRequired falls through WITHOUT setting the
	// "organization" local when the gateway named no principal, and the MustGet
	// form panics there — a 500 with no body, after the money has moved. Refuse
	// before touching anything. See SubscribeWithCard, where this cost a $99
	// charge with no subscription behind it.
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return http.Fail(c, 401, "sign in to change your subscription", nil)
	}
	sub, err := reactivateSubscription(c.Context(), org, c.Param("id"))
	if err != nil {
		switch {
		case IsSubscriptionNotFound(err):
			return http.Fail(c, 404, "subscription not found", err)
		case IsSubscriptionRefused(err):
			return http.Fail(c, 400, err.Error(), nil)
		}
		log.Error("Failed to reactivate subscription: %v", err, c)
		return http.Fail(c, 500, "failed to reactivate subscription", err)
	}

	return c.JSON(200, subscriptionResponse(sub))
}

// RenewBillingSubscription manually triggers a billing cycle renewal.
// Normally this would be automated by Temporal, but this endpoint allows
// manual triggering for testing and for deployments without Temporal.
//
//	POST /v1/billing/subscriptions/:id/renew
func RenewBillingSubscription(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	// Hydrate the org's payment credentials so the renewal can re-charge the
	// subscription's vaulted card via the per-org Square processor.
	hydratePaymentCreds(c, org)
	db := datastore.New(org.Namespaced(c.Context()))

	id := c.Param("id")
	sub := subscription.New(db)
	if err := sub.GetById(id); err != nil {
		return http.Fail(c, 404, "subscription not found", err)
	}

	inv, result, err := engine.RenewSubscription(c.Context(), db, sub, BurnCredits, chargeProviderForOrg(org))
	if err != nil {
		log.Error("Failed to renew subscription: %v", err, c)
		return http.Fail(c, 500, "failed to renew subscription", err)
	}

	if err := sub.Update(); err != nil {
		log.Error("Failed to update subscription after renewal: %v", err, c)
		return http.Fail(c, 500, "failed to update subscription", err)
	}

	resp := map[string]any{
		"subscription": subscriptionResponse(sub),
		"collection":   result,
	}
	// inv is nil when the period was not due (or a concurrent renewal is already in
	// flight): no invoice was generated and nothing was charged.
	if inv != nil {
		emitSubscriptionRenewed(c, org.Name, sub)
		resp["invoice"] = invoiceResponse(inv)
	}
	return c.JSON(200, resp)
}

func subscriptionResponse(sub *subscription.Subscription) map[string]any {
	resp := map[string]any{
		"id":                 sub.Id(),
		"userId":             sub.UserId,
		"planId":             sub.PlanId,
		"status":             sub.Status,
		"quantity":           sub.Quantity,
		"currentPeriodStart": sub.PeriodStart,
		"currentPeriodEnd":   sub.PeriodEnd,
		"cancelAtPeriodEnd":  sub.EndCancel,
		// What this subscription contributes to MRR, computed here so no reader
		// has to re-derive it from price and interval. The hanzoai/cloud admin
		// used to, with its own copy of the normalization.
		"mrrCents":             SubscriptionMRRCents(sub),
		"providerType":         sub.ProviderType,
		"defaultPaymentMethod": sub.DefaultPaymentMethod,
		"plan": map[string]any{
			"id":       sub.Plan.Id(),
			"name":     sub.Plan.Name,
			"price":    sub.Plan.Price,
			"currency": sub.Plan.Currency,
			"interval": sub.Plan.Interval,
		},
		"createdAt": sub.CreatedAt,
		"updatedAt": sub.UpdatedAt,
	}

	if !sub.TrialStart.IsZero() {
		resp["trialStart"] = sub.TrialStart
		resp["trialEnd"] = sub.TrialEnd
	}
	if !sub.CanceledAt.IsZero() {
		resp["canceledAt"] = sub.CanceledAt
	}
	if !sub.Ended.IsZero() {
		resp["endedAt"] = sub.Ended
	}

	return resp
}
