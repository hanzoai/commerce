package billing

import (
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

func minSeatsOf(p *staticPlan) int {
	if p.Limits == nil || p.Limits.MinSeats == nil {
		return -1
	}
	return *p.Limits.MinSeats
}

// TestProdPath_SeedReadbackAllFields is the prod-path guard the in-memory
// TestSeededPricesEqualEmbed missed: it seeds via the REAL path into a real
// datastore, then reads EVERY plan back through the SAME projections prod serves
// (planAuthorityRows→staticPlanFromModel for GET /v1/billing/plans, and
// resolveSubscriptionPlan for the charge) and asserts Price/PriceAnnual/Category/
// minSeats/perSeat/limits == embed for ALL 20. It FAILS on the Metadata_
// round-trip bug (envelope/minSeats dropped) and passes once persisted.
func TestProdPath_SeedReadbackAllFields(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	created, _, err := SeedPlans(c)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if created != len(hanzoPlans) {
		t.Fatalf("seed created %d, want %d", created, len(hanzoPlans))
	}

	rows, ok := planAuthorityRows(c)
	if !ok {
		t.Fatal("authority empty after seed")
	}
	list := map[string]staticPlan{}
	for _, r := range rows {
		list[r.Slug] = r
	}
	orgDB := datastore.New(nscontext.WithNamespace(c, "acme"))

	for i := range hanzoPlans {
		e := hanzoPlans[i]
		g, present := list[e.Slug]
		if !present {
			t.Errorf("%s: missing from ListPlans", e.Slug)
			continue
		}
		// Display projection (GET /v1/billing/plans).
		if g.Price != e.Price || g.PriceAnnual != e.PriceAnnual {
			t.Errorf("%s: LIST price=%d/%d want %d/%d", e.Slug, g.Price, g.PriceAnnual, e.Price, e.PriceAnnual)
		}
		if g.Category != e.Category {
			t.Errorf("%s: LIST category=%q want %q", e.Slug, g.Category, e.Category)
		}
		if g.PerSeat != e.PerSeat {
			t.Errorf("%s: LIST perSeat=%v want %v", e.Slug, g.PerSeat, e.PerSeat)
		}
		if minSeatsOf(&g) != minSeatsOf(&e) {
			t.Errorf("%s: LIST minSeats=%d want %d (envelope must round-trip)", e.Slug, minSeatsOf(&g), minSeatsOf(&e))
		}
		if (g.Limits == nil) != (e.Limits == nil) {
			t.Errorf("%s: LIST limits nil=%v want nil=%v", e.Slug, g.Limits == nil, e.Limits == nil)
		}
		// Charge path.
		rp, rerr := resolveSubscriptionPlan(orgDB, e.Slug)
		if rerr != nil {
			t.Errorf("%s: resolve err %v", e.Slug, rerr)
			continue
		}
		if int64(rp.Price) != e.Price || int64(rp.PriceAnnual) != e.PriceAnnual {
			t.Errorf("%s: RESOLVE price=%d/%d want %d/%d (CHARGE)", e.Slug, rp.Price, rp.PriceAnnual, e.Price, e.PriceAnnual)
		}
	}
}

// TestProdPath_CorrectsPreexistingBadRows proves fix #3: the corrective seed
// repairs PRE-EXISTING unmanaged partial rows on the (persistent) store — not
// just an empty DB. It plants the exact prod corruption (world-pro Price=0 like
// the bundle write; a row with no Category; team with no minSeats), then runs
// SeedPlans and asserts the charge + display are corrected to the embed.
func TestProdPath_CorrectsPreexistingBadRows(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	adb := plan.AuthorityDB(c)

	plant := func(slug string, price int64, category string) {
		p := plan.New(adb)
		p.Slug = slug
		p.Price = currency.Cents(price)
		p.Category = category
		p.Managed = false // written by a subscription-flow path, not the seed
		if err := p.Create(); err != nil {
			t.Fatalf("plant %s: %v", slug, err)
		}
	}
	// The prod corruption, re-planted on CURRENT slugs: a subscription-flow path
	// wrote a $0 or category-less row where the catalog has a real one.
	plant("go", 0, "")          // wrote $0 (the under-charge)
	plant("dev", 0, "")         // wrote $0
	plant("max", 0, "")         // wrote $0
	plant("pro", 2000, "")      // stale price, no Category/PriceAnnual/envelope
	plant("team", 2500, "team") // no minSeats/PriceAnnual

	if _, corrected, err := SeedPlans(c); err != nil {
		t.Fatalf("seed: %v", err)
	} else if corrected < 5 {
		t.Fatalf("corrected=%d, want >=5 (the planted bad rows)", corrected)
	}

	orgDB := datastore.New(nscontext.WithNamespace(c, "acme"))
	// The charge path resolves the CATALOG price — never the $0 under-charge, and
	// never the stale 2000 that was planted over pro.
	for _, slug := range []string{"go", "dev", "pro", "max"} {
		want := lookupPlan(slug).Price
		rp, err := resolveSubscriptionPlan(orgDB, slug)
		if err != nil {
			t.Fatalf("resolve %s: %v", slug, err)
		}
		if int64(rp.Price) != want {
			t.Fatalf("RESOLVE %s price=%d, want %d (bad row NOT corrected → under-charge)", slug, rp.Price, want)
		}
	}
	// Display projection is corrected too (category + minSeats restored).
	rows, ok := planAuthorityRows(c)
	if !ok {
		t.Fatal("authority empty")
	}
	byslug := map[string]staticPlan{}
	for _, r := range rows {
		byslug[r.Slug] = r
	}
	if byslug["go"].Category != "personal" {
		t.Fatalf("go category=%q, want personal (not corrected)", byslug["go"].Category)
	}
	if ms := minSeatsOf(ptr(byslug["team"])); ms != 2 {
		t.Fatalf("team minSeats=%d, want 2 (not corrected)", ms)
	}
	if want := lookupPlan("pro").PriceAnnual; byslug["pro"].PriceAnnual != want {
		t.Fatalf("pro priceAnnual=%d, want %d (not corrected)", byslug["pro"].PriceAnnual, want)
	}
}

// TestProdPath_BundleSubscriptionNoAuthorityCorruption proves fix B: subscribing
// to a bundle PARENT must NOT write a partial $0 child row into the authority.
// Before the fix, the bundle expansion created the child at Price=0, so a DIRECT
// subscription to that child then resolved $0 and under-charged.
//
// The published ladder carries no bundles, so the parent/child pair is CREATED
// here rather than borrowed from the catalog. The bundle-expansion code is still
// live and still reachable by any plan an admin gives a `bundles` list, so the
// coverage belongs to the mechanism — not to whatever the catalog happens to sell.
func TestProdPath_BundleSubscriptionNoAuthorityCorruption(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	org := moneyOrg("acme")

	adb := plan.AuthorityDB(c)
	child := plan.New(adb)
	child.Slug, child.Name, child.Category, child.Price, child.Managed = "bundle-child", "Bundle Child", "personal", 2900, true
	if err := child.Create(); err != nil {
		t.Fatalf("create child: %v", err)
	}
	parent := plan.New(adb)
	parent.Slug, parent.Name, parent.Category, parent.Price, parent.Managed = "bundle-parent", "Bundle Parent", "personal", 4900, true
	parent.Bundles = []string{"bundle-child"}
	if err := parent.Create(); err != nil {
		t.Fatalf("create parent: %v", err)
	}

	// A mint principal subscribes to the PARENT (a paid tier), which expands the
	// bundle and touches the child row.
	w := invokeSub(org, c, c1MintPrincipal, CreateBillingSubscription, `{"userId":"acme/owner","planId":"bundle-parent"}`)
	if w.StatusCode != 201 {
		t.Fatalf("create parent sub: status=%d body=%s", w.StatusCode, bodyOf(w))
	}

	// A direct child resolution must still be its real price, never a $0 partial
	// the bundle expansion overwrote it with.
	orgDB := datastore.New(nscontext.WithNamespace(c, "acme"))
	rp, err := resolveSubscriptionPlan(orgDB, "bundle-child")
	if err != nil {
		t.Fatalf("resolve bundle-child: %v", err)
	}
	if int64(rp.Price) != 2900 {
		t.Fatalf("bundle-child resolves %d after a parent bundle sub, want 2900 (bundle corrupted the authority)", rp.Price)
	}
}

func ptr(p staticPlan) *staticPlan { return &p }
