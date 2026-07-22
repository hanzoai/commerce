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

func sampleRows() []*Plan {
	return []*Plan{
		{Slug: "pro", Category: "personal", Name: "Pro", Price: 2000, PriceAnnual: 1600, Currency: currency.USD},
		{Slug: "team", Category: "team", Name: "Team", Price: 2500, PriceAnnual: 2000, Currency: currency.USD, PerSeat: true},
		{Slug: "custom", Category: "enterprise", Name: "Custom", Price: 0, ContactSales: true, Currency: currency.USD},
		{Slug: "dns-pro", Category: "dns", Name: "DNS Pro", Price: 500, PriceAnnual: 400, Currency: currency.USD},
	}
}

// TestSeed_CreatesMarkedAndIdempotent: a fresh seed creates every row, marks it
// Managed, and a re-run writes nothing (managed rows are authoritative).
func TestSeed_CreatesMarkedAndIdempotent(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	created, corrected, err := Seed(db, sampleRows())
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if created != 4 || corrected != 0 {
		t.Fatalf("first seed created=%d corrected=%d, want 4/0", created, corrected)
	}
	for _, r := range sampleRows() {
		p := New(db)
		if ok, _ := p.Query().Filter("Slug=", r.Slug).Get(); !ok {
			t.Fatalf("%s not created", r.Slug)
		}
		if !p.Managed {
			t.Fatalf("%s not marked Managed", r.Slug)
		}
	}

	created2, corrected2, err := Seed(db, sampleRows())
	if err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	if created2 != 0 || corrected2 != 0 {
		t.Fatalf("re-seed created=%d corrected=%d, want 0/0 (idempotent)", created2, corrected2)
	}
}

// TestSeed_CorrectsUnmanagedPartialRow is the prod-bug repair proven at the model
// level: a subscription-flow path wrote a partial, UNMANAGED "pro" (Price=0,
// category-less) BEFORE the seed. The old count-gated seed skipped it (kept the
// bad row); the corrective seed FORCE-CORRECTS it to the embed and marks it.
func TestSeed_CorrectsUnmanagedPartialRow(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	// A partial/wrong unmanaged row already on the (persistent) store.
	bad := New(db)
	bad.Slug = "pro"
	bad.Price = 0 // WRONG (embed is 2000) — models the bundle Price=0 write
	bad.Category = ""
	bad.Managed = false
	if err := bad.Create(); err != nil {
		t.Fatalf("seed bad row: %v", err)
	}

	created, corrected, err := Seed(db, sampleRows())
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if corrected != 1 {
		t.Fatalf("corrected=%d, want 1 (the unmanaged pro row)", corrected)
	}
	if created != 3 {
		t.Fatalf("created=%d, want 3 (team/custom/dns-pro)", created)
	}
	fixed := New(db)
	if ok, _ := fixed.Query().Filter("Slug=", "pro").Get(); !ok {
		t.Fatal("pro missing")
	}
	if fixed.Price != 2000 || fixed.Category != "personal" || !fixed.Managed {
		t.Fatalf("pro not corrected: price=%d category=%q managed=%v, want 2000/personal/true", fixed.Price, fixed.Category, fixed.Managed)
	}
}

// TestSeed_PreservesManagedEdit: an admin price edit (Managed) survives a re-seed.
func TestSeed_PreservesManagedEdit(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	if _, _, err := Seed(db, sampleRows()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	p := New(db)
	if ok, _ := p.Query().Filter("Slug=", "pro").Get(); !ok {
		t.Fatal("pro missing")
	}
	p.Price = 9900 // admin edit; row stays Managed
	if err := p.Update(); err != nil {
		t.Fatalf("edit: %v", err)
	}

	_, corrected, err := Seed(db, sampleRows())
	if err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	if corrected != 0 {
		t.Fatalf("re-seed corrected=%d, want 0 (managed edit preserved)", corrected)
	}
	got := New(db)
	if ok, _ := got.Query().Filter("Slug=", "pro").Get(); !ok {
		t.Fatal("pro gone")
	}
	if got.Price != 9900 {
		t.Fatalf("re-seed CLOBBERED managed edit: pro price=%d, want 9900", got.Price)
	}
}

// TestSeed_ConcurrentNoDuplicate: seedMu makes the per-slug check-then-write
// atomic, so N concurrent seeds create each plan exactly once (no dup slug).
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
			n, _, err := Seed(db, rows)
			if err != nil {
				t.Errorf("concurrent seed: %v", err)
				return
			}
			atomic.AddInt64(&totalCreated, int64(n))
		}()
	}
	wg.Wait()

	if totalCreated != int64(len(rows)) {
		t.Fatalf("total created across %d concurrent seeds = %d, want %d (no double-create)", N, totalCreated, len(rows))
	}
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
