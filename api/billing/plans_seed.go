package billing

import (
	"context"
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
const PinnedPlansVersion = "1.4.14"

// The display envelope (features/bundles/includedIn/limits) is the MODEL's, not
// this package's: plan.Plan carries those fields and packs them itself. This file
// used to declare a private copy plus its own pack/unpack pair, which is why the
// admin CRUD could not write them — the shape lived here, where the admin handler
// could not reach it.

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
		Prices:          centsOf(sp.Prices),
		Currency:        currency.Type(sp.Currency),
		Interval:        types.Interval(sp.Interval),
		IntervalCount:   sp.IntervalCount,
		TrialPeriodDays: sp.TrialPeriodDays,
		PerSeat:         sp.PerSeat,
		ContactSales:    sp.ContactSales,
		Popular:         sp.Popular,
		Features:        sp.Features,
		Bundles:         sp.Bundles,
		IncludedIn:      sp.IncludedIn,
		Limits:          sp.Limits,
		Licensing:       sp.Licensing,
	}
	return p
}

// centsOf and plainOf carry the price ladder across the wire/row boundary. The
// row types money as currency.Cents and the wire sends plain cents, exactly as
// Price already does on the line above each call; these only do it for a list.
// nil in, nil out — an absent ladder must stay absent rather than become an empty
// one, because the seed compares them and a nil/[] difference would rewrite every
// row on every boot.
func centsOf(in []int64) []currency.Cents {
	if in == nil {
		return nil
	}
	out := make([]currency.Cents, len(in))
	for i, v := range in {
		out[i] = currency.Cents(v)
	}
	return out
}

func plainOf(in []currency.Cents) []int64 {
	if in == nil {
		return nil
	}
	out := make([]int64, len(in))
	for i, v := range in {
		out[i] = int64(v)
	}
	return out
}

// retired records what a tier licensed WHEN IT WAS LAST ON SALE, for the rows that
// were archived before the plan row carried a licensing block at all.
//
// Provenance: @hanzo/plans@1.4.4 subscription.json — the last published version
// that still listed these tiers (1.4.5 dropped them). Those exact bytes are already
// pinned in this package: plans_drift_test.go's versionDigests["1.4.4"].subscription
// is their sha256, e490185e58b4e83d925eaf2dfd4778e28023655b610d0504b8058670bbdf2f79.
// So this is not a second catalog and not a new source of truth — it is a reading
// of the one catalog at the version that last described these tiers.
//
// Of the thirteen retired slugs, only these two licensed anything: plus and
// team-max each carried licensing.product_ids ["team"]. developer, custom and the
// world-*/social-* lines carried no licensing block in 1.4.4, so for them "no
// block" is already the correct answer and there is nothing to restore.
//
// This map is closed. A tier retired from here on carries its licensing on the row
// before it is archived, so nothing will ever need to be added.
var retired = map[string]*plan.Licensing{
	"plus":     {Products: []string{"team"}},
	"team-max": {Products: []string{"team"}},
}

// staticPlanFromModel projects an authority row back onto the wire type served by
// GET /v1/billing/plans. Money + core fields come from typed columns (so an admin
// edit wins); the rich display envelope comes from Metadata. The promo overlay is
// applied separately at the read edge (withPromo), exactly as for the embed.
func staticPlanFromModel(p *plan.Plan) staticPlan {
	return staticPlan{
		Slug:            p.Slug,
		Name:            p.Name,
		Description:     p.Description,
		Category:        p.Category,
		Price:           int64(p.Price),
		PriceAnnual:     int64(p.PriceAnnual),
		Prices:          plainOf(p.Prices),
		Currency:        string(p.Currency),
		Interval:        string(p.Interval),
		IntervalCount:   p.IntervalCount,
		TrialPeriodDays: p.TrialPeriodDays,
		ContactSales:    p.ContactSales,
		Popular:         p.Popular,
		PerSeat:         p.PerSeat,
		Features:        p.Features,
		Bundles:         p.Bundles,
		IncludedIn:      p.IncludedIn,
		Limits:          p.Limits,
		Licensing:       p.Licensing,
	}
}

// SeedPlans reconciles the subscription + DNS plan authority to the embed
// (SyncStripe/StaticPlans read the SAME source). Safe on every boot: it creates
// missing plans and FORCE-CORRECTS any unmanaged partial row (Red: the bundle
// expansion wrote Price=0 rows the old count-gated seed then skipped) while
// leaving admin-edited (managed) rows authoritative. Seed values equal the embed,
// so it changes NO charge — it only makes prices editable + repairs bad rows.
// Returns (created, corrected); corrected counts reconciles, archives, and the
// licensing backfilled onto rows retired before the row carried a licensing block.
func SeedPlans(ctx context.Context) (created, corrected int, err error) {
	db := plan.AuthorityDB(ctx)
	created, corrected, err = plan.Seed(db, SeedRows())
	if err != nil {
		return created, corrected, err
	}
	filled, err := plan.Backfill(db, retired)
	return created, corrected + filled, err
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
