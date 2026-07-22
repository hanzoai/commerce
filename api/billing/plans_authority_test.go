package billing

import (
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestSeedPlansIfEmpty_SeedsAndIdempotent: first boot seeds all embed plans; a
// second boot is a no-op (count-gated).
func TestSeedPlansIfEmpty_SeedsAndIdempotent(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	created, err := SeedPlansIfEmpty(c)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if created != len(hanzoPlans) {
		t.Fatalf("first seed created %d, want %d (17 subscription + 3 dns)", created, len(hanzoPlans))
	}

	created2, err := SeedPlansIfEmpty(c)
	if err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	if created2 != 0 {
		t.Fatalf("re-seed created %d, want 0 (idempotent)", created2)
	}
}

// TestSeededPricesEqualEmbed: every seeded row's typed money fields equal the
// embed's, so seeding changes NO charge (the whole point — it only makes prices
// editable).
func TestSeededPricesEqualEmbed(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	if _, err := SeedPlansIfEmpty(c); err != nil {
		t.Fatalf("seed: %v", err)
	}

	adb := plan.AuthorityDB(c)
	for _, embed := range hanzoPlans {
		p := plan.New(adb)
		ok, err := p.Query().Filter("Slug=", embed.Slug).Get()
		if err != nil || !ok {
			t.Fatalf("seeded plan %q missing: ok=%v err=%v", embed.Slug, ok, err)
		}
		if int64(p.Price) != embed.Price || int64(p.PriceAnnual) != embed.PriceAnnual {
			t.Errorf("seeded %q price %d/%d != embed %d/%d (seed must not change charge)", embed.Slug, p.Price, p.PriceAnnual, embed.Price, embed.PriceAnnual)
		}
		if p.ContactSales != embed.ContactSales {
			t.Errorf("seeded %q contactSales %v != embed %v (null-vs-0 preserved)", embed.Slug, p.ContactSales, embed.ContactSales)
		}
	}
}

// TestEditPrice_ResolveReflects is THE control proof: an admin edit to the plan
// authority's Price is what resolveSubscriptionPlan (→ the internal-ledger charge)
// returns — this is how admin.hanzo.ai governs subscription pricing.
func TestEditPrice_ResolveReflects(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	if _, err := SeedPlansIfEmpty(c); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Admin raises "pro" from $20 to $99 in the authority.
	adb := plan.AuthorityDB(c)
	p := plan.New(adb)
	if ok, _ := p.Query().Filter("Slug=", "pro").Get(); !ok {
		t.Fatal("pro not seeded")
	}
	p.Price = 9900
	if err := p.Update(); err != nil {
		t.Fatalf("edit: %v", err)
	}

	// resolveSubscriptionPlan — called with ANY org db — reads the NEW price.
	orgDB := datastore.New(nscontext.WithNamespace(c, "acme"))
	got, err := resolveSubscriptionPlan(orgDB, "pro")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Price != 9900 {
		t.Fatalf("resolveSubscriptionPlan(pro).Price = %d, want 9900 (admin edit MUST reflect in the charge)", got.Price)
	}
}

// TestResolveSeededEqualsEmbed: the seed itself changes no charge — resolve
// returns the SAME price before (embed fallback) and after (DB row) seeding.
func TestResolveSeededEqualsEmbed(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	orgDB := datastore.New(nscontext.WithNamespace(c, "acme"))
	embed := lookupPlan("pro")
	if embed == nil {
		t.Fatal("embed pro missing")
	}

	// Before seed: authority empty → resolve falls back to the embed VALUE.
	before, err := resolveSubscriptionPlan(orgDB, "pro")
	if err != nil {
		t.Fatalf("resolve pre-seed: %v", err)
	}
	if int64(before.Price) != embed.Price {
		t.Fatalf("pre-seed resolve price %d != embed %d", before.Price, embed.Price)
	}

	// After seed: resolve reads the DB row — SAME price (seed == embed).
	if _, err := SeedPlansIfEmpty(c); err != nil {
		t.Fatalf("seed: %v", err)
	}
	after, err := resolveSubscriptionPlan(orgDB, "pro")
	if err != nil {
		t.Fatalf("resolve post-seed: %v", err)
	}
	if int64(after.Price) != embed.Price {
		t.Fatalf("post-seed resolve price %d != embed %d (seed must not change charge)", after.Price, embed.Price)
	}
}

// TestPlanAuthorityRows_LoudFallbackThenAuthority: the read edge signals fallback
// on an empty authority (ListPlans then serves the embed, never blank), and reads
// the DB — with embed-equal content — once seeded.
func TestPlanAuthorityRows_LoudFallbackThenAuthority(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	if _, ok := planAuthorityRows(c); ok {
		t.Fatal("empty authority must signal fallback (ok=false) so ListPlans serves the embed")
	}

	if _, err := SeedPlansIfEmpty(c); err != nil {
		t.Fatalf("seed: %v", err)
	}
	rows, ok := planAuthorityRows(c)
	if !ok {
		t.Fatal("seeded authority must read ok")
	}
	if len(rows) != len(hanzoPlans) {
		t.Fatalf("authority rows = %d, want %d", len(rows), len(hanzoPlans))
	}
	bySlug := map[string]staticPlan{}
	for _, r := range rows {
		bySlug[r.Slug] = r
	}
	for _, embed := range hanzoPlans {
		got := bySlug[embed.Slug]
		if got.Price != embed.Price || got.Category != embed.Category || got.PriceAnnual != embed.PriceAnnual {
			t.Errorf("projected %q = %d/%d/%s, want %d/%d/%s", embed.Slug, got.Price, got.PriceAnnual, got.Category, embed.Price, embed.PriceAnnual, embed.Category)
		}
	}
}

// TestSyncStripeUntouchedByDBEdit: StaticPlans() — the source SyncStripe reads —
// is the EMBED and is unaffected by a DB authority edit. Confirms SyncStripe is
// untouched (deferred Step 3b): editing a plan changes the charge (resolve) but
// NOT the Stripe-dashboard projection source.
func TestSyncStripeUntouchedByDBEdit(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	if _, err := SeedPlansIfEmpty(c); err != nil {
		t.Fatalf("seed: %v", err)
	}

	before := staticMonth(StaticPlans(), "pro")

	adb := plan.AuthorityDB(c)
	p := plan.New(adb)
	if ok, _ := p.Query().Filter("Slug=", "pro").Get(); !ok {
		t.Fatal("pro missing")
	}
	p.Price = 12345
	if err := p.Update(); err != nil {
		t.Fatalf("edit: %v", err)
	}

	after := staticMonth(StaticPlans(), "pro")
	if before != after || after != 2000 {
		t.Fatalf("StaticPlans(pro) changed by a DB edit: before=%d after=%d (SyncStripe must read the untouched embed)", before, after)
	}
}

func staticMonth(plans []Plan, slug string) int64 {
	for _, p := range plans {
		if p.Slug == slug {
			return p.PriceMonth
		}
	}
	return -1
}
