package billing

import (
	"slices"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A STORE THAT HOLDS PLAN ROWS IS NEVER RECONCILED TO THE VENDORED CATALOG. The
// rows are what is sold and only a SuperAdmin's edit changes one: a boot against
// a newer vendored catalog reprices nothing, retires nothing, relists nothing and
// adds nothing, whether a row was written by an earlier import, by an admin or by
// nobody anyone can name.
func TestAHeldStoreIsNeverReconciled(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(nscontext.WithNamespace(c, plan.Namespace))

	planted := []struct {
		slug, category, status string
		cents                  int64
		managed, edited        bool
	}{
		{"developer", "personal", "", 0, true, false},                    // a tier the vendored catalog dropped
		{"dev", "personal", "", 1700, true, false},                       // imported once, vendored price differs
		{"max-5x", "personal", "", 9900, true, true},                     // a SuperAdmin's price
		{"team", "", "", 0, false, false},                                // written by nobody anyone can name
		{"max-20x", "personal", plan.StatusArchived, 20000, true, false}, // retired by an admin; the vendored copy sells it
	}
	for _, old := range planted {
		p := plan.New(db)
		p.Slug, p.Name, p.Category, p.Price = old.slug, old.slug, old.category, currencyCents(old.cents)
		p.Status, p.Managed, p.AdminEdited = old.status, old.managed, old.edited
		if err := p.Create(); err != nil {
			t.Fatalf("plant %s: %v", old.slug, err)
		}
	}

	for boot := 1; boot <= 2; boot++ {
		created, _, err := SeedPlans(c)
		if err != nil {
			t.Fatalf("boot %d: %v", boot, err)
		}
		if created != 0 {
			t.Fatalf("boot %d created %d rows in a store that holds rows; want 0", boot, created)
		}
		var all []*plan.Plan
		if _, err := plan.Query(db).GetAll(&all); err != nil {
			t.Fatalf("read: %v", err)
		}
		if len(all) != len(planted) {
			t.Fatalf("boot %d: the store holds %d rows, want the %d planted", boot, len(all), len(planted))
		}
		for _, old := range planted {
			p := plan.New(db)
			if ok, err := p.Query().Filter("Slug=", old.slug).Get(); err != nil || !ok {
				t.Fatalf("boot %d: %s: ok=%v err=%v", boot, old.slug, ok, err)
			}
			if int64(p.Price) != old.cents || p.Category != old.category || p.Status != old.status ||
				p.Managed != old.managed || p.AdminEdited != old.edited || len(p.Features) != 0 {
				t.Errorf("boot %d: %s was changed by the boot: price %d category %q status %q managed %v edited %v features %v",
					boot, old.slug, p.Price, p.Category, p.Status, p.Managed, p.AdminEdited, p.Features)
			}
		}
	}
}

// What a held store's boot does write, once: the entitlement block and the
// renamed slugs on a row that never recorded either. A row a SuperAdmin stated,
// even as granting nothing, is left as stated, and a second boot writes nothing.
func TestAHeldStoreRecordsEntitlementsOnce(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(nscontext.WithNamespace(c, plan.Namespace))

	var vendored *plan.Plan
	for _, r := range SeedRows() {
		if len(r.Entitlements) > 0 && vendored == nil {
			vendored = r
		}
	}
	if vendored == nil {
		t.Fatal("the vendored catalog states no entitlements")
	}
	for _, slug := range []string{vendored.Slug, "stated", "unknown"} {
		p := plan.New(db)
		p.Slug, p.Name, p.Category, p.Price, p.Managed = slug, slug, "personal", 1234, true
		if slug == "stated" {
			p.Entitlements = map[string]any{}
		}
		if err := p.Create(); err != nil {
			t.Fatalf("plant %s: %v", slug, err)
		}
	}
	// "stated" is a slug the vendored catalog does not publish, so plant a second
	// stated row on one it does.
	other := ""
	for _, r := range SeedRows() {
		if r.Slug != vendored.Slug && len(r.Entitlements) > 0 {
			other = r.Slug
			break
		}
	}
	p := plan.New(db)
	p.Slug, p.Name, p.Category, p.Managed, p.AdminEdited = other, other, "personal", true, true
	p.Entitlements = map[string]any{"ai.agents": float64(1)}
	if err := p.Create(); err != nil {
		t.Fatalf("plant %s: %v", other, err)
	}

	if _, filled, err := SeedPlans(c); err != nil || filled != 1 {
		t.Fatalf("first boot filled %d, err %v; want exactly the one row that never recorded what it grants", filled, err)
	}
	read := func(slug string) *plan.Plan {
		t.Helper()
		p := plan.New(db)
		if ok, err := p.Query().Filter("Slug=", slug).Get(); err != nil || !ok {
			t.Fatalf("%s: ok=%v err=%v", slug, ok, err)
		}
		return p
	}
	got := read(vendored.Slug)
	if len(got.Entitlements) != len(vendored.Entitlements) || int64(got.Price) != 1234 {
		t.Fatalf("%s: entitlements %v price %d; want the vendored block and the row's own price 1234", vendored.Slug, got.Entitlements, got.Price)
	}
	if e := read(other).Entitlements; len(e) != 1 || e["ai.agents"] != float64(1) {
		t.Fatalf("%s: a SuperAdmin's entitlements were overwritten: %v", other, e)
	}
	if e := read("stated").Entitlements; e == nil || len(e) != 0 {
		t.Fatalf("stated: %v; want the empty block it was stated with", e)
	}
	if e := read("unknown").Entitlements; e != nil {
		t.Fatalf("unknown: %v; want nothing recorded for a slug the vendored catalog does not publish", e)
	}
	if _, filled, err := SeedPlans(c); err != nil || filled != 0 {
		t.Fatalf("second boot filled %d, err %v; want 0", filled, err)
	}
}

// The one import: a store that holds no row takes the vendored catalog, with
// what each plan grants and replaces on its row.
func TestAnEmptyStoreImportsOnce(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(nscontext.WithNamespace(c, plan.Namespace))
	created, _, err := SeedPlans(c)
	if err != nil || created != len(SeedRows()) {
		t.Fatalf("import created %d, err %v; want %d", created, err, len(SeedRows()))
	}
	for _, r := range SeedRows() {
		p := plan.New(db)
		if ok, err := p.Query().Filter("Slug=", r.Slug).Get(); err != nil || !ok {
			t.Fatalf("%s: ok=%v err=%v", r.Slug, ok, err)
		}
		if p.Entitlements == nil || len(p.Entitlements) != len(r.Entitlements) || !slices.Equal(p.Replaces, r.Replaces) {
			t.Errorf("%s: entitlements %v replaces %v; want %v and %v", r.Slug, p.Entitlements, p.Replaces, r.Entitlements, r.Replaces)
		}
	}
	// An admin reprices a row; the next boot leaves it.
	p := plan.New(db)
	if ok, err := p.Query().Filter("Slug=", "dev").Get(); err != nil || !ok {
		t.Fatalf("dev: ok=%v err=%v", ok, err)
	}
	p.Price, p.AdminEdited = 4242, true
	if err := p.Update(); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if created, corrected, err := SeedPlans(c); err != nil || created != 0 || corrected != 0 {
		t.Fatalf("second boot = created %d corrected %d err %v; want nothing written", created, corrected, err)
	}
	p = plan.New(db)
	if ok, err := p.Query().Filter("Slug=", "dev").Get(); err != nil || !ok || p.Price != 4242 {
		t.Fatalf("dev after the second boot: price %d ok %v err %v; want the admin's 4242", p.Price, ok, err)
	}
}

func currencyCents(v int64) currency.Cents { return currency.Cents(v) }

// THE TOP OF THE LADDER IS PUBLISHED AND SOLD BY NOTHING UNTIL THE OWNER SAYS. Max
// 100x ($1,000) is a draft in the catalog: a boot creates its row unlisted, the
// public catalog does not list it, and every purchase entrypoint treats it as a
// slug that never existed — so a deploy that carries the price charges no one.
// Max 50x ($500) left draft on the owner's word (@hanzo/plans 1.8.19): it is
// listed and sold like every other rung.
func TestDraftTiersAreSeededUnlistedAndSoldByNothing(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(nscontext.WithNamespace(c, plan.Namespace))
	if _, _, err := SeedPlans(c); err != nil {
		t.Fatalf("seed: %v", err)
	}

	listed, ok := planAuthorityRows(c)
	if !ok {
		t.Fatal("authority read failed")
	}
	sold := false
	for _, r := range listed {
		if r.Slug == "max-100x" {
			t.Errorf("%s is in the public catalog while it is a draft", r.Slug)
		}
		sold = sold || r.Slug == "max-50x"
	}
	if !sold {
		t.Error("max-50x is on sale and missing from the public catalog")
	}
	for slug, cents := range map[string]int64{"max-50x": 50000, "max-100x": 100000} {
		p, err := resolveSubscriptionPlan(db, slug)
		if err != nil {
			t.Fatalf("%s must still resolve (renewals, invoices): %v", slug, err)
		}
		if int64(p.Price) != cents {
			t.Errorf("%s resolves at %d cents, want %d", slug, p.Price, cents)
		}
		if on := p.Listed(); on != (slug == "max-50x") {
			t.Errorf("%s resolves listed=%v", slug, on)
		}
	}
	// The embed fallback, used when the authority cannot answer, offers the draft to no one.
	for _, p := range listedCatalog() {
		if p.Slug == "max-100x" {
			t.Errorf("the embed fallback offers %s", p.Slug)
		}
	}
	for _, p := range StaticPlans() {
		if p.Slug == "max-100x" {
			t.Errorf("StaticPlans hands %s to a seeder", p.Slug)
		}
	}
	if lookupPlan("max-100x") == nil {
		t.Error("a draft plan must still be answerable by lookupPlan")
	}
}
