package billing

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/api/promo"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/util/json/http"
)

//go:embed plans/subscription.json
var subscriptionJSON embed.FS

//go:embed plans/dns.json
var dnsJSON embed.FS

// planLimits is the catalog `limits` block. It is an ALIAS of plan.Limits, not a
// second declaration: the block is plan data, so it lives with the plan, and the
// catalog JSON, the stored row and this wire type are then one shape by
// construction rather than by three structs agreeing.
type planLimits = plan.Limits

// canonicalPlan is the JSON shape from @hanzo/plans/*.json.
//
// Bundles: a plan can grant entitlement to other plans without
// charging separately for them. Example: subscribing to "pro"
// auto-grants "world-pro" because `"bundles": ["world-pro"]` is set on
// the pro tier. The reverse view `includedIn` lets product surfaces
// (world.hanzo.ai, chat, etc.) tell users "this plan is included in
// these higher tiers" so the upsell story is symmetric.
type canonicalPlan struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	PriceMonthly *float64 `json:"priceMonthly"` // dollars per month (null for custom)
	PriceAnnual  *float64 `json:"priceAnnual"`  // dollars per month billed annually (null for custom)
	// Prices is every price the plan is sold at, in dollars per month, ascending,
	// with Prices[0] == PriceMonthly. Absent for a plan sold at one price.
	Prices       []float64 `json:"prices,omitempty"`
	Category     string    `json:"category"`
	Popular      bool      `json:"popular,omitempty"`
	ContactSales bool      `json:"contactSales,omitempty"`
	// TrialPeriodDays is the base (no-card) free-trial length advertised for the
	// plan. The actual on-ramp length is decided at signup by billing/trial
	// (7 days without a card, 30 with one) — this only surfaces the base offer
	// on GET /v1/billing/plans.
	TrialPeriodDays *int        `json:"trialPeriodDays,omitempty"`
	Features        []string    `json:"features"`
	Bundles         []string    `json:"bundles,omitempty"`    // slugs of plans whose entitlement this plan also grants
	IncludedIn      []string    `json:"includedIn,omitempty"` // slugs of plans that include this plan as a bundle
	Limits          *planLimits `json:"limits,omitempty"`
	// PriceRef carries the catalog's billing reference; recurring.per_seat marks
	// the plan as billed per seat (price × quantity, floored at limits.minSeats).
	PriceRef *struct {
		Recurring *struct {
			PerSeat bool `json:"per_seat"`
		} `json:"recurring,omitempty"`
	} `json:"price_ref,omitempty"`
	Payouts *struct {
		IdleResalePercent int    `json:"idleResalePercent"`
		Description       string `json:"description"`
	} `json:"payouts,omitempty"`
	// Entitlements is the catalog's typed entitlement block. Only the licensing.*
	// keys are read here — they decide what a subscriber may RUN, so they must be
	// persisted onto the plan row rather than re-read from the catalog at question
	// time (see plan.Licensing). The other namespaces (ai.*, cloud.*, commerce.*)
	// are served from the catalog by the plans vocabulary and are not row data.
	Entitlements struct {
		Products []string `json:"licensing.product_ids,omitempty"`
		Apps     []string `json:"licensing.app_ids,omitempty"`
		Features []string `json:"licensing.engine_features,omitempty"`
		Seats    *int     `json:"licensing.seats,omitempty"`
	} `json:"entitlements,omitempty"`
}

// licensingOf projects the catalog's licensing.* entitlement keys onto the row's
// typed block, or nil when the tier licenses nothing. nil rather than an empty
// struct so "licenses nothing" and "we never recorded it" stay distinguishable on
// the row — Backfill only fills the second.
func licensingOf(cp *canonicalPlan) *plan.Licensing {
	e := cp.Entitlements
	if len(e.Products) == 0 && len(e.Apps) == 0 && len(e.Features) == 0 && e.Seats == nil {
		return nil
	}
	return &plan.Licensing{Products: e.Products, Apps: e.Apps, Features: e.Features, Seats: e.Seats}
}

// PlanView is a plan as this service SELLS it — the wire type GET /billing/plans
// returns, promo annotation and all. Fields match the Plan type in the billing
// frontend's commerce-client.ts.
type PlanView struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Price       int64  `json:"price"`       // monthly price in cents (0 = free)
	PriceAnnual int64  `json:"priceAnnual"` // annual price in cents per month
	// Prices is every price this plan is sold at, in cents, ascending, with
	// Prices[0] == Price. A client renders one control over this list and sends
	// back the INDEX it landed on (subscribe/card's `level`) — never a price. It
	// is absent for a plan sold at a single price, so a client that ignores it
	// keeps working and a plan that gains a ladder needs no client release.
	Prices          []int64 `json:"prices,omitempty"`
	Currency        string  `json:"currency"`
	Interval        string  `json:"interval"`
	IntervalCount   int     `json:"intervalCount"`
	TrialPeriodDays int     `json:"trialPeriodDays"`
	ContactSales    bool    `json:"contactSales,omitempty"`
	Popular         bool    `json:"popular,omitempty"`
	// PromoPercent / PromoUntil surface the ACTIVE, admin-configured platform plan
	// promo for this plan (percent off + when it ends) — sourced from the promo
	// package (a Promotion), never hardcoded in the catalog. Zero/empty when no promo
	// covers this plan, so the client shows a discount only while one is live.
	PromoPercent int      `json:"promoPercent,omitempty"`
	PromoUntil   string   `json:"promoUntil,omitempty"`
	Features     []string `json:"features,omitempty"`
	Bundles      []string `json:"bundles,omitempty"`    // see canonicalPlan.Bundles
	IncludedIn   []string `json:"includedIn,omitempty"` // see canonicalPlan.IncludedIn
	// PerSeat marks a plan billed per seat (catalog price_ref.recurring.per_seat):
	// invoices charge Price × subscription quantity, floored at Limits.MinSeats.
	PerSeat bool        `json:"perSeat,omitempty"`
	Limits  *planLimits `json:"limits,omitempty"`
	// Licensing is what the tier licenses — carried so the seed can persist it onto
	// the row, where it survives the tier's retirement.
	Licensing *plan.Licensing `json:"licensing,omitempty"`
}

// staticPlan is the name the rest of the package writes for a PlanView. It is an
// ALIAS, not a second declaration: the plan a peer reads off the internal plane
// and the plan the endpoint serves are then one type by construction, rather than
// two structs kept in agreement.
type staticPlan = PlanView

// catalog contains every plan this service sells, loaded at init from the
// operator's directory when one is named and from the embed otherwise. It was
// `catalog` — a brand in an identifier, in a package a second brand is meant
// to be able to run.
// Subscription plans have category "personal", "team", or "enterprise".
// DNS plans have category "dns".
var catalog []staticPlan

// dnsPlans is a filtered view containing only DNS plans for the /dns/plans endpoint.
var dnsPlans []staticPlan

func init() {
	catalog = loadPlansFromEmbed(subscriptionJSON, "plans/subscription.json")

	dns := loadPlansFromEmbed(dnsJSON, "plans/dns.json")
	dnsPlans = dns
	catalog = append(catalog, dns...)
}

// loadPlansFromEmbed reads an embedded JSON file and converts canonical plans
// to the staticPlan wire format. Panics on failure because plan data is required
// for the service to operate.
func loadPlansFromEmbed(fs embed.FS, path string) []staticPlan {
	data, err := fs.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("billing: failed to read embedded %s: %v", path, err))
	}

	plans, err := parsePlans(data)
	if err != nil {
		panic(fmt.Sprintf("billing: failed to parse %s: %v", path, err))
	}
	return plans
}

// parsePlans is the ONE decoder. It was inline in the embed reader, so a second
// source would have meant a second copy of the projection — and the projection
// is what decides which JSON field becomes which charged column.
func parsePlans(data []byte) ([]staticPlan, error) {
	var canonical []canonicalPlan
	if err := json.Unmarshal(data, &canonical); err != nil {
		return nil, err
	}

	plans := make([]staticPlan, len(canonical))
	for i, cp := range canonical {
		sp := staticPlan{
			Slug:          cp.ID,
			Name:          cp.Name,
			Description:   cp.Description,
			Category:      cp.Category,
			Currency:      "usd",
			Interval:      "monthly",
			IntervalCount: 1,
			ContactSales:  cp.ContactSales,
			Popular:       cp.Popular,
			Features:      cp.Features,
			Bundles:       cp.Bundles,
			IncludedIn:    cp.IncludedIn,
		}

		if cp.PriceMonthly != nil {
			sp.Price = int64(math.Round(*cp.PriceMonthly * 100))
		}
		if cp.PriceAnnual != nil {
			sp.PriceAnnual = int64(math.Round(*cp.PriceAnnual * 100))
		}
		for _, d := range cp.Prices {
			sp.Prices = append(sp.Prices, int64(math.Round(d*100)))
		}
		if cp.TrialPeriodDays != nil {
			sp.TrialPeriodDays = *cp.TrialPeriodDays
		}
		if cp.PriceRef != nil && cp.PriceRef.Recurring != nil {
			sp.PerSeat = cp.PriceRef.Recurring.PerSeat
		}
		sp.Limits = cp.Limits
		sp.Licensing = licensingOf(&canonical[i])

		plans[i] = sp
	}

	return plans, nil
}

// withPromo returns a COPY of the catalog (never the shared catalog var) with
// each paid plan annotated by the ACTIVE, admin-configured platform promo. Applying
// it here — at the read edge — is what makes the discount admin-controlled: the
// catalog JSON carries no promo, the promo package (a Promotion) is the single
// source, and a plan shows a discount only while a promo is live and covers it.
//
// The live promo is passed IN. Resolving it is a read of the reserved platform
// namespace off the request (promo.Active), and taking the resolved value keeps
// the annotation usable by a caller that has no request. A nil promo annotates
// nothing and the plans come back at list price.
func withPromo(pr *promo.Promo, plans []staticPlan) []staticPlan {
	out := make([]staticPlan, len(plans))
	copy(out, plans)
	if pr == nil {
		return out
	}
	until := ""
	if pr.End != nil {
		until = pr.End.UTC().Format(time.RFC3339)
	}
	for i := range out {
		// Only PAID plans carry a percent-off promo (a $0 plan has nothing to discount).
		if out[i].Price > 0 && pr.AppliesTo(out[i].Slug) {
			out[i].PromoPercent = pr.PercentOff
			out[i].PromoUntil = until
		}
	}
	return out
}

// ReadPlans is what this service sells, optionally narrowed to one category and
// annotated with a live promo — the QUESTION, with no HTTP in it.
//
// It takes values rather than a request so a caller that is not a request can
// ask: the same catalog is read over the internal plane by a peer that holds no
// plan authority, and a copy of it there would be a second answer to "what do we
// sell and for how much" — the answer a customer is charged against.
//
// An empty category means "everything", which is what an absent query parameter
// has always meant here.
//
// The list is freshly allocated, never the shared catalog, so a caller may
// annotate its own copy without editing what the next reader sees. The error is
// the shape every core on this plane answers in; this read has no failure of its
// own, because an unreadable or empty plan authority falls back to the embedded
// catalog — loudly (planAuthorityRows logs) — rather than serving a blank list.
func ReadPlans(ctx context.Context, category string, pr *promo.Promo) ([]PlanView, error) {
	// The DB plan authority (admin-editable) is the source of truth; the embed is a
	// LOUD-failing fallback (planAuthorityRows logs when it fires) so a failed seed
	// or query serves the known catalog, never a silently blank list.
	plans, ok := planAuthorityRows(ctx)
	if !ok {
		plans = catalog
	}
	if category == "" {
		return withPromo(pr, plans), nil
	}

	filtered := make([]staticPlan, 0)
	for _, p := range plans {
		if p.Category == category {
			filtered = append(filtered, p)
		}
	}
	return withPromo(pr, filtered), nil
}

// ListPlans is the endpoint over ReadPlans. Catalog data is admin-editable and
// embedded as a fallback; the promo is admin-configured and resolved per request.
//
//	GET /v1/billing/plans
//	GET /v1/billing/plans?category=dns
func ListPlans(c *zip.Ctx) error {
	plans, err := ReadPlans(c.Context(), c.Query("category"), promo.Active(c))
	if err != nil {
		return http.Fail(c, 500, "failed to list plans", err)
	}
	return c.JSON(200, plans)
}

// GetPlan returns a single plan by slug, annotated with the active platform promo.
//
//	GET /v1/billing/plans/:id
func GetPlan(c *zip.Ctx) error {
	id := c.Param("id")
	// DB authority first; embed is the loud-failing fallback (see ListPlans).
	plans, ok := planAuthorityRows(c.Context())
	if !ok {
		plans = catalog
	}
	for _, p := range plans {
		if p.Slug == id {
			return c.JSON(200, withPromo(promo.Active(c), []staticPlan{p})[0])
		}
	}
	return http.Fail(c, 404, "plan not found", nil)
}

// lookupPlan finds a plan by slug across all loaded plans.
// Returns nil if not found.
func lookupPlan(slug string) *staticPlan {
	for i := range catalog {
		if catalog[i].Slug == slug {
			return &catalog[i]
		}
	}
	return nil
}

// IncludedMonthlyCents returns the recurring monthly included-usage allotment
// for a plan slug, in cents. Returns 0 when the plan is unknown or declares no
// included allotment. This is the single catalog-derived input to the monthly
// allotment grant — the dollar value is the plan's declared cloud credit
// (@hanzo/plans limits.includedCloudCredits / includedCloudCreditsPerUser,
// i.e. the cloud.included_credits_usd entitlement).
func IncludedMonthlyCents(slug string) int64 {
	p := lookupPlan(slug)
	if p == nil || p.Limits == nil {
		return 0
	}
	// The monthly allotment grants the plan's declared cloud-credit allowance:
	// @hanzo/plans publishes it as limits.includedCloudCredits (flat) or
	// includedCloudCreditsPerUser (per seat) — the canonical
	// cloud.included_credits_usd entitlement. includedCreditUsd is a legacy alias
	// no published plan sets. Prefer the real fields, in that order.
	usd := p.Limits.IncludedCloudCredits
	if usd == nil {
		usd = p.Limits.IncludedCloudCreditsPerUser
	}
	if usd == nil {
		usd = p.Limits.IncludedCreditUsd
	}
	if usd == nil || *usd <= 0 {
		return 0
	}
	return int64(*usd) * 100
}

// paidTier reports whether the plan identified by slug charges money — a monthly
// price above zero. This, NOT the included allotment, is what makes a subscription
// a paid tier: a free ($0) plan may still carry a small included credit as a perk
// (e.g. developer's $5/mo) yet stays self-serve. The price is read from the catalog
// by slug so a stored subscription's spoofable plan copy can never inflate it.
// Unknown slugs are not paid. The self-subscribe gate and the entitlement-anchor
// clamp gate on this; the allotment AMOUNT stays IncludedMonthlyCents.
//
// A CONTACT-SALES plan counts as paid even though it stores Price=0. Its price is
// null, not free — the row records "talk to us", and a plan you must negotiate for
// is by definition not self-serve. Reading Price alone made a null-priced plan
// indistinguishable from a $0 one, so a catalog holding a contact-sales tier with
// a real included allotment would let an org admin self-subscribe and mint it with
// no payment. That was previously true only by luck: the tier with the large
// allotment happened to also carry a large price, and the null-priced tier happened
// to carry no allotment. Luck is not the gate.
func paidTier(slug string) bool {
	p := lookupPlan(slug)
	return p != nil && (p.Price > 0 || p.ContactSales)
}

// perSeat reports whether the catalog bills the plan per seat
// (price_ref.recurring.per_seat). Unknown slugs are flat.
func perSeat(slug string) bool {
	p := lookupPlan(slug)
	return p != nil && p.PerSeat
}

// minSeats returns the catalog's minimum billable seats for a plan
// (limits.minSeats, the ONE canonical home for seat minimums). 1 when the
// plan is unknown or declares no minimum.
func minSeats(slug string) int {
	p := lookupPlan(slug)
	if p == nil || p.Limits == nil || p.Limits.MinSeats == nil || *p.Limits.MinSeats < 1 {
		return 1
	}
	return *p.Limits.MinSeats
}

// AgentsIncluded and BotsIncluded report how many of each the plan may RUN,
// read from the catalog by slug — the same embed every other gate reads, so
// what is enforced cannot disagree with what was published.
//
// These bound CONCURRENCY and nothing else. What an agent costs is its runtime,
// metered by the hour, so these numbers never appear in an invoice line; a
// caller that treats "includes 1 bot" as a month of free compute is reading a
// capacity as an allowance, and a resident bot's month is ~720 hours of it.
//
// The bool is the point. Every sibling accessor above returns a bare number
// because a missing value has a safe reading there: no allotment is no grant,
// no minimum is one seat. A missing CAPACITY has no safe number. Zero would
// refuse a customer their first agent, and a large default would give an
// unbounded roster away, so the catalog's silence is returned AS silence and
// the caller decides — enforce a known bound, or serve without one.
//
// -1 is unlimited, the convention the catalog already uses for MaxMembers and
// licensing.seats. Callers compare against it before comparing against a count.
func AgentsIncluded(slug string) (int, bool) { return roster(slug, agentSeat) }

// BotsIncluded is AgentsIncluded for the persistent-bot kind.
func BotsIncluded(slug string) (int, bool) { return roster(slug, botSeat) }

// seatKind selects which roster count to read. Two kinds today, and the reason
// they share one reader is that "how many does this plan include" is one
// question — a second copy of the lookup is a second place for the unknown-vs-
// zero rule to be got wrong.
type seatKind int

const (
	agentSeat seatKind = iota
	botSeat
)

func roster(slug string, kind seatKind) (int, bool) {
	p := lookupPlan(slug)
	if p == nil || p.Limits == nil {
		return 0, false
	}
	n := p.Limits.Agents
	if kind == botSeat {
		n = p.Limits.Bots
	}
	if n == nil {
		return 0, false
	}
	return *n, true
}

// Plan is the exported snapshot used by external seeders (e.g. the
// Stripe parity seed in commerce.go). It mirrors the subset of fields
// the seed populates onto seed.Plan, with field names that match the
// caller's expectations (PriceMonth / PriceYear are cent-denominated
// monthly + annual prices). Internal callers stick with staticPlan;
// this type exists so the public surface doesn't leak the unexported
// shape and so we can evolve them independently.
type Plan struct {
	Slug        string
	Name        string
	Description string
	Category    string
	PriceMonth  int64
	PriceYear   int64
	Currency    string
}

// StaticPlans returns a snapshot of the embedded plan catalog as the
// exported Plan shape. The slice is freshly allocated so callers may
// mutate freely without bleeding into the canonical catalog var.
func StaticPlans() []Plan {
	out := make([]Plan, len(catalog))
	for i, p := range catalog {
		out[i] = toPlan(&p)
	}
	return out
}

// LookupStaticPlan resolves a single plan by slug from the embedded
// catalog and returns it in the exported Plan shape. Returns nil when
// the slug is unknown. It is the single-plan analogue of StaticPlans
// and shares the same staticPlan -> Plan projection, so external
// seeders (e.g. cmd/grant) never touch the unexported wire type.
func LookupStaticPlan(slug string) *Plan {
	sp := lookupPlan(slug)
	if sp == nil {
		return nil
	}
	p := toPlan(sp)
	return &p
}

// toPlan projects the internal staticPlan wire type onto the exported
// Plan shape. Single source of truth for the field mapping used by both
// StaticPlans and LookupStaticPlan.
func toPlan(p *staticPlan) Plan {
	return Plan{
		Slug:        p.Slug,
		Name:        p.Name,
		Description: p.Description,
		Category:    p.Category,
		PriceMonth:  p.Price,
		PriceYear:   p.PriceAnnual,
		Currency:    p.Currency,
	}
}
