package billing

import (
	"net/http"
	"slices"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
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

// A PLAN THE CATALOG DOES NOT SELL IS PUBLISHED AND SOLD BY NOTHING. @hanzo/plans
// 1.10.1 keeps the rungs commerce retired (dev, max-5x, max-20x, max-50x)
// archived, exactly as they were sold, and states no other plan unlisted. A boot
// creates each row at its status, the public catalog does not list it, and every
// purchase entrypoint treats it as a slug that never existed — so a deploy that
// carries the price charges no one. Each still resolves at its price, for the
// renewals and invoices that recorded it. Max 100x, the one draft before, is gone
// from the catalog.
func TestUnlistedTiersAreSeededUnlistedAndSoldByNothing(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(nscontext.WithNamespace(c, plan.Namespace))
	if _, _, err := SeedPlans(c); err != nil {
		t.Fatalf("seed: %v", err)
	}

	unlisted := map[string]struct {
		status string
		cents  int64
	}{
		"dev":     {plan.StatusArchived, 2000},
		"max-5x":  {plan.StatusArchived, 10000},
		"max-20x": {plan.StatusArchived, 20000},
		"max-50x": {plan.StatusArchived, 50000},
	}
	held := 0
	for i := range catalog {
		if !plan.ListedStatus(catalog[i].Status) {
			held++
			if _, named := unlisted[catalog[i].Slug]; !named {
				t.Errorf("the catalog holds %s %s, which this test does not name", catalog[i].Slug, catalog[i].Status)
			}
		}
	}
	if held != len(unlisted) {
		t.Errorf("the catalog holds %d plans unlisted, want the %d named here", held, len(unlisted))
	}

	listed, ok := planAuthorityRows(c)
	if !ok {
		t.Fatal("authority read failed")
	}
	for _, r := range listed {
		if _, hidden := unlisted[r.Slug]; hidden {
			t.Errorf("%s is in the public catalog while it is %s", r.Slug, unlisted[r.Slug].status)
		}
	}
	for slug, want := range unlisted {
		p, err := resolveSubscriptionPlan(db, slug)
		if err != nil {
			t.Fatalf("%s must still resolve (renewals, invoices): %v", slug, err)
		}
		if int64(p.Price) != want.cents {
			t.Errorf("%s resolves at %d cents, want %d", slug, p.Price, want.cents)
		}
		if p.Status != want.status || p.Listed() {
			t.Errorf("%s resolves status=%q listed=%v, want %q and unlisted", slug, p.Status, p.Listed(), want.status)
		}
		if lookupPlan(slug) == nil {
			t.Errorf("%s must still be answerable by lookupPlan", slug)
		}
	}
	// The embed fallback, used when the authority cannot answer, offers none of
	// them to anyone.
	for _, p := range listedCatalog() {
		if _, hidden := unlisted[p.Slug]; hidden {
			t.Errorf("the embed fallback offers %s", p.Slug)
		}
	}
	for _, p := range StaticPlans() {
		if _, hidden := unlisted[p.Slug]; hidden {
			t.Errorf("StaticPlans hands %s to a seeder", p.Slug)
		}
	}
	if lookupPlan("max-100x") != nil {
		t.Error("max-100x is answerable, and @hanzo/plans 1.10.0 drops it")
	}

	// Every purchase entrypoint refuses each of them as a slug that never existed,
	// and nothing is charged: a card sale, a subscription opened by the one caller
	// allowed to open a paid plan with no payment, and a move onto it.
	org := moneyOrg("acme")
	odb := datastore.New(org.Namespaced(c))
	m := squareMock("cust_ul", "ccof_ul", "sqpay_ul")
	withFakeSquare(t, m)
	onGo := subscription.New(odb)
	onGo.UserId, onGo.Status, onGo.ProviderType, onGo.Plan.Slug, onGo.PlanId = "acme/held", subscription.Active, "square", "go", "go"
	if err := onGo.Create(); err != nil {
		t.Fatalf("seed a go subscription: %v", err)
	}
	for slug := range unlisted {
		if r := invokeSubscribeCard(org, c, `{"sourceId":"cnon:ok","planId":"`+slug+`"}`, nil); r.StatusCode != http.StatusNotFound {
			t.Errorf("card sale of %s: %d %s, want 404", slug, r.StatusCode, bodyOf(r))
		}
		if r := invokeSub(org, c, c1MintPrincipal, CreateBillingSubscription, `{"userId":"acme/self","planId":"`+slug+`"}`); r.StatusCode != http.StatusNotFound {
			t.Errorf("opening %s as the platform: %d %s, want 404", slug, r.StatusCode, bodyOf(r))
		}
		if r := invokeSubPatch(org, c, c1MintPrincipal, onGo.Id(), `{"planId":"`+slug+`"}`); r.StatusCode != http.StatusNotFound {
			t.Errorf("a move onto %s as the platform: %d %s, want 404", slug, r.StatusCode, bodyOf(r))
		}
	}
	if m.chargeCalls != 0 || m.addCardNonce != "" {
		t.Errorf("refused sales charged %d time(s) and vaulted %q", m.chargeCalls, m.addCardNonce)
	}
	for _, who := range []string{"acme", "acme/self"} {
		if n, err := subscription.Query(odb).Filter("UserId=", who).Count(); err != nil || n != 0 {
			t.Errorf("refused sales opened %d subscription(s) for %s (%v)", n, who, err)
		}
	}
	if got := reload(t, odb, onGo.Id()); got.Plan.Slug != "go" {
		t.Errorf("a refused move left the subscription on %q, want go", got.Plan.Slug)
	}
}

// Every rung of the 1.10.1 ladder, Ultimate and Unlimited included, is bought by
// card at its list price by the buyer themselves, with no price agreed and no
// staff: the ladder is self-serve from Go to Unlimited.
func TestEveryLadderRungSellsByCardAtList(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	if _, _, err := SeedPlans(c); err != nil {
		t.Fatalf("seed: %v", err)
	}
	for _, slug := range []string{"go", "pro-3x", "dev-5x", "max-15x", "ultra", "ultimate", "unlimited"} {
		org := moneyOrg("rung-" + slug)
		m := squareMock("cust_"+slug, "ccof_"+slug, "sqpay_"+slug)
		withFakeSquare(t, m)
		r := invokeSubscribeCard(org, c, `{"sourceId":"cnon:ok","planId":"`+slug+`"}`, nil)
		if r.StatusCode != http.StatusCreated {
			t.Errorf("%s by card: %d %s, want 201", slug, r.StatusCode, bodyOf(r))
			continue
		}
		if want := lookupPlan(slug).Price; m.chargeCalls != 1 || m.lastChargeAmount != want {
			t.Errorf("%s by card charged %d time(s), last %d; want once at its list %d", slug, m.chargeCalls, m.lastChargeAmount, want)
		}
	}
}
