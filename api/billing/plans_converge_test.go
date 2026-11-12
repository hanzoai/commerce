package billing

import (
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The production cutover, end to end: a store holding the OLD catalog boots
// against the NEW one and converges — without a token, a script, or a human.
//
// This is the case the previous seed could not handle. Every stored row was
// Managed (the seed set that flag itself), so the seed skipped all of them: it
// would create `go` and `free`, leave `pro` at its old price, and leave `developer` and
// `plus` on sale beside them. A catalog half-new and half-stale, which reads to a
// customer as a pricing page that cannot make up its mind, and to billing as two
// prices for one tier.
func TestCatalogConverges_OldStoreToPublishedLadder(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(nscontext.WithNamespace(c, plan.Namespace))

	// Plant the catalog as production actually holds it: seeded rows, Managed,
	// carrying the retired tiers and the old prices.
	for _, old := range []struct {
		slug, name, category string
		cents                int64
	}{
		{"developer", "Developer", "personal", 0},
		{"pro", "Pro", "personal", 2000},
		{"plus", "Plus", "personal", 10000},
		{"max", "Max", "personal", 20000},
		{"world-pro", "World Pro", "world", 2900},
	} {
		p := plan.New(db)
		p.Slug, p.Name, p.Category, p.Price, p.Managed = old.slug, old.name, old.category, currencyCents(old.cents), true
		if err := p.Create(); err != nil {
			t.Fatalf("plant %s: %v", old.slug, err)
		}
	}

	if _, _, err := SeedPlans(c); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rows, ok := planAuthorityRows(c)
	if !ok {
		t.Fatal("authority read failed")
	}
	got := map[string]int64{}
	for _, r := range rows {
		got[r.Slug] = r.Price
	}

	// The published ladder is what is offered, at the published prices.
	for slug, want := range map[string]int64{
		"free": 0, "go": 800, "pro-3x": 1500, "dev-5x": 4500, "max-15x": 9500, "ultra": 19500, "team": 2500,
	} {
		if v, ok := got[slug]; !ok || v != want {
			t.Errorf("public %q = %d cents (listed %v), want %d", slug, v, ok, want)
		}
	}
	// The retired tiers are gone from the offer, and so are the rungs the catalog
	// archives and the one it keeps private.
	for _, slug := range []string{"developer", "plus", "world-pro", "pro", "max", "dev", "max-5x", "max-20x", "max-50x", "unlimited"} {
		if _, still := got[slug]; still {
			t.Errorf("retired %q is still on sale", slug)
		}
	}
	// But their ROWS survive, so anything that recorded the slug still resolves.
	for _, slug := range []string{"developer", "plus", "world-pro", "pro", "max"} {
		p := plan.New(db)
		if found, err := p.Query().Filter("Slug=", slug).Get(); err != nil || !found {
			t.Errorf("retired %q was DESTROYED (found=%v err=%v); history cannot resolve it", slug, found, err)
		}
	}
}

func currencyCents(v int64) currency.Cents { return currency.Cents(v) }

// A PLAN THE CATALOG DOES NOT SELL IS PUBLISHED AND SOLD BY NOTHING. @hanzo/plans
// 1.10.0 keeps Unlimited ($995) private — staff assign it, no checkout sells it —
// and the rungs commerce retired (dev, max-5x, max-20x, max-50x) archived, exactly
// as they were sold. A boot creates each row at its status, the public catalog
// does not list it, and every purchase entrypoint treats it as a slug that never
// existed — so a deploy that carries the price charges no one. Each still resolves
// at its price, for the renewals and invoices that recorded it. Max 100x, the one
// draft before, is gone from the catalog.
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
		"unlimited": {plan.StatusPrivate, 99500},
		"dev":       {plan.StatusArchived, 2000},
		"max-5x":    {plan.StatusArchived, 10000},
		"max-20x":   {plan.StatusArchived, 20000},
		"max-50x":   {plan.StatusArchived, 50000},
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
