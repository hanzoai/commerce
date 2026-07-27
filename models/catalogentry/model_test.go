package catalogentry

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/util/test/ae"
)

// ensoRow is the shape a syncer publishes for one of our own SKUs: cost only,
// per component, with the family's own per-context rungs preserved.
func ensoRow() ModelRow {
	return ModelRow{
		Slug: "enso",
		Name: "Enso",
		Spec: ModelSpec{
			Vendor:        "Hanzo",
			Family:        FamilyEnso,
			Serves:        "enso",
			Modality:      "chat",
			ContextWindow: 1000000,
			MinTier:       "trial",
			Enabled:       true,
		},
		Costs: []Rate{
			{Key: RateIn, Unit: UnitMTok, Cost: "6.00"},
			{Key: RateOut, Unit: UnitMTok, Cost: "18.00"},
		},
	}
}

func TestUpsertModels_CreatesModelRows(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	stats, err := UpsertModels(db, []ModelRow{ensoRow()})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if stats.Created != 1 || stats.Updated != 0 {
		t.Fatalf("first sync = %+v, want {created:1 updated:0}", stats)
	}

	e := New(db)
	ok, err := e.Query().Filter("Slug=", "enso").Get()
	if err != nil || !ok {
		t.Fatalf("load enso: ok=%v err=%v", ok, err)
	}
	if !IsModel(e) || e.Category != CategoryEnso {
		t.Fatalf("category = %q, want %q (our family is its own category)", e.Category, CategoryEnso)
	}
	if e.Spec == nil || e.Spec.ContextWindow != 1000000 || e.Spec.MinTier != "trial" {
		t.Fatalf("spec did not round-trip: %+v", e.Spec)
	}
	if len(e.Rates) != 2 || e.Rates[0].Cost != "6.00" {
		t.Fatalf("rates did not round-trip: %+v", e.Rates)
	}
	if !e.Published {
		t.Fatal("a model row must be published — visibility is not the entitlement gate")
	}
}

// A sync owns cost; admin owns price. This is the money-critical invariant: an
// upstream price move must never change what a customer pays.
func TestUpsertModels_SyncOwnsCostAdminOwnsPrice(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	if _, err := UpsertModels(db, []ModelRow{ensoRow()}); err != nil {
		t.Fatalf("seed sync: %v", err)
	}

	// Admin sets a retail price, a markup, a tier and takes the model offline.
	e := New(db)
	if ok, _ := e.Query().Filter("Slug=", "enso").Get(); !ok {
		t.Fatal("enso missing")
	}
	e.Rates[0].Price = "20.00"
	e.Markup = "3.0"
	e.Name = "Enso (flagship)"
	e.Spec.MinTier = "paid"
	e.Spec.Enabled = false
	e.Spec.Unavailable = "capacity"
	if err := e.Update(); err != nil {
		t.Fatalf("admin edit: %v", err)
	}

	// Upstream raises its cost and re-states its (stale) policy defaults.
	row := ensoRow()
	row.Name = "Enso"
	row.Costs[0].Cost = "9.00"
	row.Costs[0].Price = "1.00" // a syncer must not be able to state retail
	row.Spec.MinTier = "trial"
	row.Spec.Enabled = true
	row.Spec.ContextWindow = 2000000
	stats, err := UpsertModels(db, []ModelRow{row})
	if err != nil {
		t.Fatalf("re-sync: %v", err)
	}
	if stats.Created != 0 || stats.Updated != 1 {
		t.Fatalf("re-sync = %+v, want {created:0 updated:1}", stats)
	}

	got := New(db)
	if ok, _ := got.Query().Filter("Slug=", "enso").Get(); !ok {
		t.Fatal("enso missing after re-sync")
	}
	if got.Rates[0].Cost != "9.00" {
		t.Fatalf("cost = %q, want 9.00 (sync owns cost)", got.Rates[0].Cost)
	}
	if got.Rates[0].Price != "20.00" {
		t.Fatalf("price = %q, want 20.00 (admin owns price; a sync must never move it)", got.Rates[0].Price)
	}
	if got.Markup != "3.0" {
		t.Fatalf("markup = %q, want 3.0 (admin owns markup)", got.Markup)
	}
	if got.Name != "Enso (flagship)" {
		t.Fatalf("name = %q, want the admin's name", got.Name)
	}
	if got.Spec.MinTier != "paid" || got.Spec.Enabled || got.Spec.Unavailable != "capacity" {
		t.Fatalf("routing policy was clobbered by a sync: %+v", got.Spec)
	}
	if got.Spec.ContextWindow != 2000000 {
		t.Fatalf("contextWindow = %d, want 2000000 (a machine fact IS synced)", got.Spec.ContextWindow)
	}
}

// A rung upstream withdraws disappears; a rung that survives keeps its price.
func TestUpsertModels_RateSetFollowsUpstream(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	row := ensoRow()
	row.Costs = []Rate{
		{Key: RateIn, Unit: UnitMTok, Cost: "6.00"},
		{Key: RateIn, Unit: UnitMTok, MaxContext: 200000, Cost: "12.00"},
	}
	if _, err := UpsertModels(db, []ModelRow{row}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	e := New(db)
	e.Query().Filter("Slug=", "enso").Get()
	e.Rates[0].Price = "20.00"
	e.Rates[1].Price = "40.00"
	if err := e.Update(); err != nil {
		t.Fatalf("price: %v", err)
	}

	row.Costs = []Rate{{Key: RateIn, Unit: UnitMTok, Cost: "6.50"}}
	if _, err := UpsertModels(db, []ModelRow{row}); err != nil {
		t.Fatalf("re-sync: %v", err)
	}

	got := New(db)
	got.Query().Filter("Slug=", "enso").Get()
	if len(got.Rates) != 1 {
		t.Fatalf("rates = %d, want 1 (the withdrawn rung is gone)", len(got.Rates))
	}
	if got.Rates[0].Cost != "6.50" || got.Rates[0].Price != "20.00" {
		t.Fatalf("surviving rate = %+v, want cost 6.50 with the admin price 20.00 kept", got.Rates[0])
	}
}

func TestRetailPrice_ExplicitElseCostTimesMarkup(t *testing.T) {
	cases := []struct {
		name   string
		rate   Rate
		markup string
		want   string
	}{
		{"explicit price wins", Rate{Cost: "6", Price: "20"}, "3.0", "20"},
		{"default markup", Rate{Cost: "6"}, "", "7.2"},
		{"per-entry markup", Rate{Cost: "6"}, "3.0", "18"},
		{"loss leader is representable", Rate{Cost: "6"}, "0.5", "3"},
		{"unknown cost stays unknown", Rate{}, "1.2", ""},
		{"token-scale precision", Rate{Cost: "0.000015"}, "1.20", "0.000018"},
	}
	for _, tc := range cases {
		if got := tc.rate.RetailPrice(tc.markup); got != tc.want {
			t.Errorf("%s: RetailPrice = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestRateMarginPct_DerivedAndHonest(t *testing.T) {
	pct := RateMarginPct(Rate{Cost: "5", Price: "20"}, "")
	if pct == nil || *pct != 75 {
		t.Fatalf("margin = %v, want 75", pct)
	}
	// Selling under cost is representable and SHOWN, never suppressed.
	neg := RateMarginPct(Rate{Cost: "6", Price: "4"}, "")
	if neg == nil || *neg != -50 {
		t.Fatalf("below-cost margin = %v, want -50 (a loss must be visible)", neg)
	}
	// An unknown cost yields an ABSENT margin, never a fabricated 100%.
	if unknown := RateMarginPct(Rate{Price: "20"}, ""); unknown != nil {
		t.Fatalf("margin over an unknown cost = %v, want nil", unknown)
	}
}

// The public projection may never carry cost or margin; the admin one must.
func TestProjectModels_PublicHidesCostAdminShowsMargin(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	row := ensoRow()
	if _, err := UpsertModels(db, []ModelRow{row}); err != nil {
		t.Fatalf("sync: %v", err)
	}
	e := New(db)
	e.Query().Filter("Slug=", "enso").Get()
	e.Rates[0].Price = "20.00"
	e.Update()

	pub, err := Project(db, "models")
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if len(pub.Products) != 1 {
		t.Fatalf("models scope returned %d products, want 1", len(pub.Products))
	}
	p := pub.Products[0]
	if len(p.Rates) != 2 || p.Rates[0].Price != "20.00" {
		t.Fatalf("public rates = %+v, want the retail vector", p.Rates)
	}
	if p.Spec == nil || p.Spec.MinTier != "trial" {
		t.Fatalf("public spec = %+v, want the entitlement projected to everyone", p.Spec)
	}
	blob, _ := json.Marshal(pub)
	for _, forbidden := range []string{`"cost"`, `"marginPct"`, `"adminRates"`, `"markup"`} {
		if strings.Contains(string(blob), forbidden) {
			t.Fatalf("public projection leaked %s", forbidden)
		}
	}

	adm, err := ProjectAdmin(db, "models")
	if err != nil {
		t.Fatalf("project admin: %v", err)
	}
	a := adm.Products[0]
	if len(a.AdminRates) != 2 {
		t.Fatalf("admin rates = %+v, want 2", a.AdminRates)
	}
	if a.AdminRates[0].Cost != "6.00" || a.AdminRates[0].Price != "20.00" {
		t.Fatalf("admin rate[0] = %+v, want cost 6.00 / price 20.00", a.AdminRates[0])
	}
	if a.AdminRates[0].MarginPct == nil || *a.AdminRates[0].MarginPct != 70 {
		t.Fatalf("admin margin = %v, want 70", a.AdminRates[0].MarginPct)
	}
	// The output rate carries no admin price, so retail is cost x default markup
	// (18.00 x 1.20 = 21.60) and the margin is the default spread.
	if a.AdminRates[1].Price != "21.6" {
		t.Fatalf("derived retail = %q, want 21.6", a.AdminRates[1].Price)
	}
}

// A model row belongs to the "models" scope ONLY — it must never appear in a
// brand's product sidebar, exactly as an infra tier does not.
func TestModelsScope_IsolatedFromProductCatalog(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	if _, err := Seed(db); err != nil {
		t.Fatalf("seed products: %v", err)
	}
	if _, err := UpsertModels(db, []ModelRow{ensoRow()}); err != nil {
		t.Fatalf("sync models: %v", err)
	}

	for _, brand := range []string{"hanzo", "lux", "zoo", "pars", "infra"} {
		cat, err := Project(db, brand)
		if err != nil {
			t.Fatalf("project %s: %v", brand, err)
		}
		for _, p := range cat.Products {
			if p.Slug == "enso" {
				t.Fatalf("brand %s surfaced a model row", brand)
			}
		}
	}
	models, err := Project(db, "models")
	if err != nil {
		t.Fatalf("project models: %v", err)
	}
	if len(models.Products) != 1 || models.Products[0].Slug != "enso" {
		t.Fatalf("models scope = %d products, want just enso", len(models.Products))
	}
	if len(models.Categories) != len(modelCategories) {
		t.Fatalf("models taxonomy = %d, want %d", len(models.Categories), len(modelCategories))
	}
}

// Every entry projects the SAME price shape: an infra tier written with the
// legacy scalar reads as a one-element vector, so there is one way to read a
// price whether it is a VM or a model.
func TestRatesOf_ScalarProjectsAsOneElementVector(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	if _, err := SeedInfra(db); err != nil {
		t.Fatalf("seed infra: %v", err)
	}
	cat, err := Project(db, "infra")
	if err != nil {
		t.Fatalf("project infra: %v", err)
	}
	if len(cat.Products) == 0 {
		t.Fatal("no infra tiers")
	}
	for _, p := range cat.Products {
		if len(p.Rates) != 1 {
			t.Fatalf("%s: rates = %d, want the degenerate one-element vector", p.Slug, len(p.Rates))
		}
		wantUnit := UnitMonth
		if p.Category == "gpu" {
			wantUnit = UnitHour
		}
		if p.Rates[0].Unit != wantUnit {
			t.Fatalf("%s: unit = %q, want %q", p.Slug, p.Rates[0].Unit, wantUnit)
		}
		if p.Rates[0].Price == "" {
			t.Fatalf("%s: a seeded tier must project a price", p.Slug)
		}
	}
}
