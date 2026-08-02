package billing

import (
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The production cutover, end to end: a store holding the OLD catalog boots
// against the NEW one and converges — without a token, a script, or a human.
//
// This is the case the previous seed could not handle. Every stored row was
// Managed (the seed set that flag itself), so the seed skipped all of them: it
// would create `go` and `dev`, leave `pro` at $20, and leave `developer` and
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
	for slug, want := range map[string]int64{"go": 900, "dev": 1900, "pro": 4900, "max": 9900} {
		if got[slug] != want {
			t.Errorf("public %q = %d cents, want %d", slug, got[slug], want)
		}
	}
	// The retired tiers are gone from the offer.
	for _, slug := range []string{"developer", "plus", "world-pro"} {
		if _, still := got[slug]; still {
			t.Errorf("retired %q is still on sale", slug)
		}
	}
	// But their ROWS survive, so anything that recorded the slug still resolves.
	for _, slug := range []string{"developer", "plus", "world-pro"} {
		p := plan.New(db)
		if found, err := p.Query().Filter("Slug=", slug).Get(); err != nil || !found {
			t.Errorf("retired %q was DESTROYED (found=%v err=%v); history cannot resolve it", slug, found, err)
		}
	}
}

func currencyCents(v int64) currency.Cents { return currency.Cents(v) }
