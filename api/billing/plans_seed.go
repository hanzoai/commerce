package billing

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/types/currency"
	types "github.com/hanzoai/commerce/types"
)

// The plan authority (models/plan, "system" namespace) is seeded from — and
// falls back to — the SAME embedded @hanzo/plans catalog (hanzoPlans) that
// SyncStripe and StaticPlans read. Seeding therefore changes NO charge: a
// seeded row's typed money fields equal the embed's. Once seeded, admin.hanzo.ai
// edits the rows and BOTH GET /v1/billing/plans and resolveSubscriptionPlan read
// the edited price. SyncStripe is untouched — it still projects StaticPlans().

// PinnedPlansVersion is the @hanzo/plans package version the vendored JSON under
// api/billing/plans is a copy of. Re-vendoring the JSON and bumping this pin go
// together; plans_drift_test.go asserts the embedded bytes still hash to the
// recorded digest for this version, so a stray edit to a vendored price (which
// the seed writes and resolveSubscriptionPlan charges) fails CI loudly.
// NOTE: the vendored plans/package.json is gitignored (a broad package.json
// ignore), so it is NOT the pin — this const + the digest test are.
const PinnedPlansVersion = "1.4.4"

// planEnvelopeKey is the single Metadata key that carries a plan's rich display
// envelope (the fields that are NOT typed money columns). One key, one JSON
// string, so it round-trips cleanly through the Map[string]interface{} the ORM
// deserializes Metadata into (a string survives; a nested struct would degrade
// to map[string]interface{} and lose the typed limits shape).
const planEnvelopeKey = "envelope"

// planEnvelope is the display-only envelope stored in plan.Metadata. The money
// authority stays the typed columns; these are presentation the client renders.
type planEnvelope struct {
	Features   []string    `json:"features,omitempty"`
	Bundles    []string    `json:"bundles,omitempty"`
	IncludedIn []string    `json:"includedIn,omitempty"`
	Limits     *planLimits `json:"limits,omitempty"`
}

// SeedRows projects the embedded plan catalog onto authority model rows. Typed
// money fields (Price/PriceAnnual/Category/ContactSales/PerSeat/…) become
// columns; the rich display envelope rides Metadata. This is the ONE seed
// source, so the DB authority starts byte-for-byte equal to the embed.
func SeedRows() []*plan.Plan {
	rows := make([]*plan.Plan, 0, len(hanzoPlans))
	for i := range hanzoPlans {
		rows = append(rows, planFromStatic(&hanzoPlans[i]))
	}
	return rows
}

// planFromStatic maps one embed wire plan onto an authority model row. It
// PRESERVES the free($0)-vs-custom(null) distinction: the embed already carries
// it as Price=0 + ContactSales, and we copy ContactSales verbatim — never
// coercing a null-priced custom plan into a chargeable $0.
func planFromStatic(sp *staticPlan) *plan.Plan {
	p := &plan.Plan{
		Slug:            sp.Slug,
		Name:            sp.Name,
		Description:     sp.Description,
		Category:        sp.Category,
		Price:           currency.Cents(sp.Price),
		PriceAnnual:     currency.Cents(sp.PriceAnnual),
		Currency:        currency.Type(sp.Currency),
		Interval:        types.Interval(sp.Interval),
		IntervalCount:   sp.IntervalCount,
		TrialPeriodDays: sp.TrialPeriodDays,
		PerSeat:         sp.PerSeat,
		ContactSales:    sp.ContactSales,
		Popular:         sp.Popular,
	}
	p.Metadata = envelopeMeta(sp)
	return p
}

// envelopeMeta packs the display envelope into a plan's Metadata under one key.
// Returns nil when there is nothing to carry (so a bare plan has empty Metadata).
func envelopeMeta(sp *staticPlan) types.Map {
	env := planEnvelope{Features: sp.Features, Bundles: sp.Bundles, IncludedIn: sp.IncludedIn, Limits: sp.Limits}
	if len(env.Features) == 0 && len(env.Bundles) == 0 && len(env.IncludedIn) == 0 && env.Limits == nil {
		return nil
	}
	b, err := json.Marshal(env)
	if err != nil {
		return nil
	}
	return types.Map{planEnvelopeKey: string(b)}
}

// envelopeFrom unpacks the display envelope stored by envelopeMeta.
func envelopeFrom(m types.Map) planEnvelope {
	var env planEnvelope
	if m == nil {
		return env
	}
	if s, ok := m[planEnvelopeKey].(string); ok && s != "" {
		_ = json.Unmarshal([]byte(s), &env)
	}
	return env
}

// staticPlanFromModel projects an authority row back onto the wire type served by
// GET /v1/billing/plans. Money + core fields come from typed columns (so an admin
// edit wins); the rich display envelope comes from Metadata. The promo overlay is
// applied separately at the read edge (withPromo), exactly as for the embed.
func staticPlanFromModel(p *plan.Plan) staticPlan {
	env := envelopeFrom(p.Metadata)
	return staticPlan{
		Slug:            p.Slug,
		Name:            p.Name,
		Description:     p.Description,
		Category:        p.Category,
		Price:           int64(p.Price),
		PriceAnnual:     int64(p.PriceAnnual),
		Currency:        string(p.Currency),
		Interval:        string(p.Interval),
		IntervalCount:   p.IntervalCount,
		TrialPeriodDays: p.TrialPeriodDays,
		ContactSales:    p.ContactSales,
		Popular:         p.Popular,
		PerSeat:         p.PerSeat,
		Features:        env.Features,
		Bundles:         env.Bundles,
		IncludedIn:      env.IncludedIn,
		Limits:          env.Limits,
	}
}

// SeedPlans reconciles the subscription + DNS plan authority to the embed
// (SyncStripe/StaticPlans read the SAME source). Safe on every boot: it creates
// missing plans and FORCE-CORRECTS any unmanaged partial row (Red: the bundle
// expansion wrote Price=0 rows the old count-gated seed then skipped) while
// leaving admin-edited (managed) rows authoritative. Seed values equal the embed,
// so it changes NO charge — it only makes prices editable + repairs bad rows.
// Returns (created, corrected).
func SeedPlans(ctx context.Context) (created, corrected int, err error) {
	return plan.Seed(plan.AuthorityDB(ctx), SeedRows())
}

// planAuthorityRows reads the plan authority for the read edge (ListPlans/
// GetPlan). It returns (rows, true) on success; on a query error OR an empty
// authority (the seed did not run) it logs LOUDLY and returns (nil, false) so
// the caller serves the embed fallback — never a silently blank plan list.
func planAuthorityRows(ctx context.Context) ([]staticPlan, bool) {
	var rows []*plan.Plan
	if _, err := plan.Query(plan.AuthorityDB(ctx)).GetAll(&rows); err != nil {
		log.Error("billing plans: authority read FAILED, serving embed fallback: %v", err, nil)
		return nil, false
	}
	if len(rows) == 0 {
		log.Warn("billing plans: authority is EMPTY (seed did not run?), serving embed fallback")
		return nil, false
	}
	out := make([]staticPlan, 0, len(rows))
	for _, p := range rows {
		// Draft and archived rows exist but are not sold. This is the whole point
		// of Status: before it, the only way to unlist a plan was to DELETE it,
		// which takes the row's history and orphans any subscription that recorded
		// the slug. An archived plan still resolves for renewals and invoices —
		// it just stops being offered.
		if !p.Listed() {
			continue
		}
		out = append(out, staticPlanFromModel(p))
	}
	// An authority holding ONLY unlisted rows is not the same as an empty one: the
	// seed ran, the operator archived everything. Serving the embed fallback there
	// would resurrect retired tiers on the public page, which is the exact failure
	// this field exists to prevent. Empty-and-deliberate is a valid answer.
	sortByEmbedOrder(out)
	return out, true
}

// sortByEmbedOrder gives the authority rows the canonical embed display order
// (so a seeded==embed authority serves the same order the client expects);
// admin-added plans not in the embed sort after, by slug. Stable + deterministic.
func sortByEmbedOrder(rows []staticPlan) {
	idx := make(map[string]int, len(hanzoPlans))
	for i := range hanzoPlans {
		idx[hanzoPlans[i].Slug] = i
	}
	sort.SliceStable(rows, func(i, j int) bool {
		oi, oki := idx[rows[i].Slug]
		oj, okj := idx[rows[j].Slug]
		if oki && okj {
			return oi < oj
		}
		if oki != okj {
			return oki // embed-known plans before admin-added ones
		}
		return rows[i].Slug < rows[j].Slug
	})
}
