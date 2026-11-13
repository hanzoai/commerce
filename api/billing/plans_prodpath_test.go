package billing

import (
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/plan"
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
	if created != len(catalog) {
		t.Fatalf("seed created %d, want %d", created, len(catalog))
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

	for i := range catalog {
		e := catalog[i]
		g, present := list[e.Slug]
		if !plan.ListedStatus(e.Status) {
			// Draft, private and archived rows are not on sale, and still charge
			// their price to the renewals and invoices that recorded them.
			if present {
				t.Errorf("%s: a %s plan is on sale in ListPlans", e.Slug, e.Status)
			}
			rp, rerr := resolveSubscriptionPlan(orgDB, e.Slug)
			if rerr != nil {
				t.Errorf("%s: resolve err %v", e.Slug, rerr)
				continue
			}
			if int64(rp.Price) != e.Price || int64(rp.PriceAnnual) != e.annual() || rp.Status != e.Status {
				t.Errorf("%s: RESOLVE price=%d/%d status=%q want %d/%d %q (CHARGE)", e.Slug, rp.Price, rp.PriceAnnual, rp.Status, e.Price, e.annual(), e.Status)
			}
			continue
		}
		if !present {
			t.Errorf("%s: missing from ListPlans", e.Slug)
			continue
		}
		// Display projection (GET /v1/billing/plans).
		if g.Price != e.Price || !sameCents(g.PriceAnnual, e.PriceAnnual) {
			t.Errorf("%s: LIST price=%d/%s want %d/%s", e.Slug, g.Price, centsText(g.PriceAnnual), e.Price, centsText(e.PriceAnnual))
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
		if int64(rp.Price) != e.Price || int64(rp.PriceAnnual) != e.annual() {
			t.Errorf("%s: RESOLVE price=%d/%d want %d/%d (CHARGE)", e.Slug, rp.Price, rp.PriceAnnual, e.Price, e.annual())
		}
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
