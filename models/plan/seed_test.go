package plan

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

func sysDB(c context.Context) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(c, Namespace))
}

// sampleRows spans four distinct categories with one plan each, plus a
// null-priced (contactSales) custom plan, so the tests exercise the free($0)
// vs custom(null) distinction and the per-category seed gate.
func sampleRows() []*Plan {
	return []*Plan{
		{Slug: "pro", Category: "personal", Name: "Pro", Price: 2000, PriceAnnual: 1600, Currency: currency.USD},
		{Slug: "team", Category: "team", Name: "Team", Price: 2500, PriceAnnual: 2000, Currency: currency.USD, PerSeat: true},
		{Slug: "custom", Category: "enterprise", Name: "Custom", Price: 0, ContactSales: true, Currency: currency.USD},
		{Slug: "dns-pro", Category: "dns", Name: "DNS Pro", Price: 500, PriceAnnual: 400, Currency: currency.USD},
	}
}

// TestSeed_IdempotentAndNonDestructive proves a re-seed creates nothing AND never
// clobbers an admin edit — the money-safety property (a re-seed must not reset a
// price an admin lowered/raised).
func TestSeed_IdempotentAndNonDestructive(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	created, err := Seed(db, sampleRows())
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if created != 4 {
		t.Fatalf("first seed created %d, want 4", created)
	}

	// Admin edits a seeded plan's price.
	p := New(db)
	if ok, _ := p.Query().Filter("Slug=", "pro").Get(); !ok {
		t.Fatal("pro not found after seed")
	}
	p.Price = 9900
	if err := p.Update(); err != nil {
		t.Fatalf("edit: %v", err)
	}

	// Re-seed: no-op (idempotent) AND preserves the edit (non-destructive).
	created2, err := Seed(db, sampleRows())
	if err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	if created2 != 0 {
		t.Fatalf("re-seed created %d, want 0 (idempotent)", created2)
	}
	got := New(db)
	if ok, _ := got.Query().Filter("Slug=", "pro").Get(); !ok {
		t.Fatal("pro gone after re-seed")
	}
	if got.Price != 9900 {
		t.Fatalf("re-seed CLOBBERED edit: pro price = %d, want 9900 preserved", got.Price)
	}

	// The null-priced custom plan kept its distinction (0 + ContactSales), never
	// coerced to a chargeable $0.
	cust := New(db)
	if ok, _ := cust.Query().Filter("Slug=", "custom").Get(); !ok {
		t.Fatal("custom missing")
	}
	if cust.Price != 0 || !cust.ContactSales {
		t.Fatalf("custom = price %d contactSales %v, want 0/true (null preserved)", cust.Price, cust.ContactSales)
	}
}

// TestSeedIfEmpty_GatedAndRespectsDelete proves the count-gate: it seeds once,
// then never re-runs while any seeded category has rows — so an admin DELETE of a
// plan is not resurrected on the next boot.
func TestSeedIfEmpty_GatedAndRespectsDelete(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	created, err := SeedIfEmpty(db, sampleRows())
	if err != nil {
		t.Fatalf("seed-if-empty: %v", err)
	}
	if created != 4 {
		t.Fatalf("first SeedIfEmpty created %d, want 4", created)
	}

	// Admin deletes one plan; other categories remain populated.
	p := New(db)
	if ok, _ := p.Query().Filter("Slug=", "pro").Get(); !ok {
		t.Fatal("pro missing")
	}
	if err := p.Delete(); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Re-run: gate stays shut (team/enterprise/dns still populated) → delete respected.
	created2, err := SeedIfEmpty(db, sampleRows())
	if err != nil {
		t.Fatalf("re-run: %v", err)
	}
	if created2 != 0 {
		t.Fatalf("SeedIfEmpty re-run created %d, want 0 (must not resurrect a delete)", created2)
	}
	gone := New(db)
	if ok, _ := gone.Query().Filter("Slug=", "pro").Get(); ok {
		t.Fatal("deleted plan pro was resurrected by SeedIfEmpty")
	}
}

// TestSeed_ConcurrentNoDuplicate proves the Red F3 fix: the seedMu mutex makes the
// per-slug check-then-create atomic, so N concurrent Seed calls create each plan
// exactly once (no duplicate-slug rows). Mirrors the giftcard 25-goroutine
// same-key test.
func TestSeed_ConcurrentNoDuplicate(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	rows := sampleRows()
	const N = 25
	var wg sync.WaitGroup
	var totalCreated int64
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := Seed(db, rows)
			if err != nil {
				t.Errorf("concurrent seed: %v", err)
				return
			}
			atomic.AddInt64(&totalCreated, int64(n))
		}()
	}
	wg.Wait()

	// Exactly one create-wave landed across all goroutines.
	if totalCreated != int64(len(rows)) {
		t.Fatalf("total created across %d concurrent seeds = %d, want %d (no double-create)", N, totalCreated, len(rows))
	}
	// And no slug has a duplicate row.
	for _, r := range rows {
		var got []*Plan
		if _, err := Query(db).Filter("Slug=", r.Slug).GetAll(&got); err != nil {
			t.Fatalf("query %s: %v", r.Slug, err)
		}
		if len(got) != 1 {
			t.Fatalf("slug %q has %d rows, want 1 (no duplicate)", r.Slug, len(got))
		}
	}
}
