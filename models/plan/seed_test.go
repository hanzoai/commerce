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

// TestSeed_PreservesAdminEdit: a price a HUMAN set through the admin CRUD
// survives a re-seed. The published catalog decides; an admin edit overrides.
func TestSeed_PreservesAdminEdit(t *testing.T) {
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
	// Modelled the way api/plan's UpdateEntry writes it — the flag is what makes
	// this an override rather than just a mutated row.
	p.Price = 9900
	p.AdminEdited = true
	if err := p.Update(); err != nil {
		t.Fatalf("edit: %v", err)
	}

	_, corrected, err := Seed(db, sampleRows())
	if err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	if corrected != 0 {
		t.Fatalf("re-seed corrected=%d, want 0 (admin edit preserved)", corrected)
	}
	got := New(db)
	if ok, _ := got.Query().Filter("Slug=", "pro").Get(); !ok {
		t.Fatal("pro gone")
	}
	if got.Price != 9900 {
		t.Fatalf("re-seed CLOBBERED an admin edit: pro price=%d, want 9900", got.Price)
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


// The seed must be able to correct ITS OWN prior output. This is the property
// Managed made impossible: it was set by the seed and by the admin CRUD alike, so
// once a row existed nothing could tell a stale seeded price from a deliberate
// one, and publishing a new catalog left every changed plan at its old price
// while creating the new ones beside them — half-new, half-stale.
func TestSeed_ReconcilesItsOwnRowsToANewCatalog(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	if _, _, err := Seed(db, sampleRows()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// The catalog reprices pro. Nobody edited the row by hand.
	next := sampleRows()
	for _, r := range next {
		if r.Slug == "pro" {
			r.Price = 4900
		}
	}
	if _, corrected, err := Seed(db, next); err != nil {
		t.Fatalf("re-seed: %v", err)
	} else if corrected != 1 {
		t.Fatalf("re-seed corrected=%d, want 1 (the repriced row)", corrected)
	}
	got := New(db)
	if ok, _ := got.Query().Filter("Slug=", "pro").Get(); !ok {
		t.Fatal("pro gone")
	}
	if int64(got.Price) != 4900 {
		t.Fatalf("pro price=%d after the catalog repriced it, want 4900", got.Price)
	}

	// And it is genuinely idempotent: nothing left to change, nothing written.
	if _, corrected, err := Seed(db, next); err != nil {
		t.Fatalf("third seed: %v", err)
	} else if corrected != 0 {
		t.Fatalf("third seed corrected=%d, want 0 (idempotent)", corrected)
	}
}

// A plan the catalog stops publishing is ARCHIVED on the next boot — not deleted,
// and not left on sale. Otherwise retiring a tier needs a second manual step, and
// the tier goes on selling until someone remembers to take it.
func TestSeed_ArchivesWhatTheCatalogStoppedPublishing(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	if _, _, err := Seed(db, sampleRows()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// The next catalog drops every row but the first.
	next := sampleRows()[:1]
	if _, _, err := Seed(db, next); err != nil {
		t.Fatalf("re-seed: %v", err)
	}

	var all []*Plan
	if _, err := Query(db).GetAll(&all); err != nil {
		t.Fatalf("read back: %v", err)
	}
	kept := next[0].Slug
	for _, p := range all {
		if p.Slug == kept {
			if !p.Listed() {
				t.Fatalf("still-published %q was archived", p.Slug)
			}
			continue
		}
		if p.Listed() {
			t.Errorf("unpublished %q is still listed", p.Slug)
		}
		// The ROW SURVIVES — an invoice that recorded the slug must still resolve.
		if p.Price == 0 && p.Name == "" {
			t.Errorf("unpublished %q was destroyed rather than archived", p.Slug)
		}
	}
}

// An admin-created plan is not "missing from the catalog" — it is theirs, and the
// archive sweep must leave it alone.
func TestSeed_DoesNotArchiveAnAdminsOwnPlan(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	if _, _, err := Seed(db, sampleRows()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	mine := New(db)
	mine.Slug, mine.Name, mine.Category, mine.Price = "bespoke", "Bespoke", "personal", 12345
	mine.AdminEdited, mine.Managed = true, true
	if err := mine.Create(); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, _, err := Seed(db, sampleRows()); err != nil {
		t.Fatalf("re-seed: %v", err)
	}

	got := New(db)
	if ok, _ := got.Query().Filter("Slug=", "bespoke").Get(); !ok {
		t.Fatal("admin plan gone")
	}
	if !got.Listed() {
		t.Fatal("the seed archived a plan an admin created; it is not the catalog's to retire")
	}
	if int64(got.Price) != 12345 {
		t.Fatalf("admin plan price=%d, want 12345 untouched", got.Price)
	}
}

// limitsEqual must compare Limits BY VALUE. Every field is a *int, so `*a == *b`
// compares ADDRESSES — two Limits decoded from the same JSON would read unequal,
// and the seed would rewrite every row on every boot. The values would still be
// right, so nothing would look broken; what breaks is `corrected` never reaching
// zero, which is the only signal that says the catalog has converged.
func TestLimitsEqual_ComparesValuesNotAddresses(t *testing.T) {
	two, alsoTwo, three := 2, 2, 3
	if !limitsEqual(&Limits{MinSeats: &two}, &Limits{MinSeats: &alsoTwo}) {
		t.Fatal("equal values at different addresses read as unequal")
	}
	if limitsEqual(&Limits{MinSeats: &two}, &Limits{MinSeats: &three}) {
		t.Fatal("different values read as equal")
	}
	if !limitsEqual(nil, nil) {
		t.Fatal("both-absent must be equal")
	}
	if limitsEqual(&Limits{MinSeats: &two}, nil) {
		t.Fatal("present and absent must differ")
	}
	// A field set on one side only is a difference, not a match.
	if limitsEqual(&Limits{MinSeats: &two}, &Limits{}) {
		t.Fatal("set-vs-unset field read as equal")
	}
}

// And the whole point: a second boot against an unchanged catalog writes NOTHING,
// including for rows that carry limits.
func TestSeed_IdempotentWithLimits(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	two := 2
	rows := []*Plan{{Slug: "seat", Name: "Seat", Category: "team", Price: 2500, Limits: &Limits{MinSeats: &two}}}
	if _, _, err := Seed(db, rows); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Fresh values at fresh addresses — exactly what a re-decoded catalog looks like.
	twoAgain := 2
	same := []*Plan{{Slug: "seat", Name: "Seat", Category: "team", Price: 2500, Limits: &Limits{MinSeats: &twoAgain}}}
	if _, corrected, err := Seed(db, same); err != nil {
		t.Fatalf("re-seed: %v", err)
	} else if corrected != 0 {
		t.Fatalf("re-seed corrected=%d, want 0 — an unchanged catalog must write nothing", corrected)
	}
}
