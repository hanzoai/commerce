package billing

import (
	"slices"
	"testing"

	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestPublishedTiersCarryLicensing: the seed persists what the catalog publishes
// under licensing.*, onto the row's own typed block. This is the mapping every
// other guarantee here rests on — if the row never records it, nothing downstream
// can survive the tier's retirement.
func TestPublishedTiersCarryLicensing(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	if _, _, err := SeedPlans(c); err != nil {
		t.Fatalf("seed: %v", err)
	}

	want := map[string][]string{
		"free":       nil, // $0, and licenses no proprietary product
		"dev":        {"team"},
		"max":        {"engine", "team"},
		"team":       {"team"},
		"enterprise": {"engine", "team"},
	}
	for slug, w := range want {
		p := plan.New(plan.AuthorityDB(c))
		ok, err := p.Query().Filter("Slug=", slug).Get()
		if err != nil || !ok {
			t.Fatalf("seeded plan %q missing: ok=%v err=%v", slug, ok, err)
		}
		var got []string
		if p.Licensing != nil {
			got = p.Licensing.Products
		}
		if !slices.Equal(got, w) {
			t.Errorf("plan %q licenses %v, want %v (catalog licensing.product_ids must land on the row)", slug, got, w)
		}
	}
}

// TestLicensingSurvivesRetirement is the whole point, stated as the rule: a tier
// the catalog STOPS publishing keeps knowing what it licensed.
//
// It seeds the published catalog, then reconciles against a catalog that no longer
// carries "dev" — exactly what shipping a release that retires a tier does. The row
// is archived, as it should be, and it must STILL answer the licensing question,
// because the subscriber on it is still being charged.
func TestLicensingSurvivesRetirement(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := plan.AuthorityDB(c)
	if _, _, err := SeedPlans(c); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Publish a catalog with "dev" removed — retire the tier.
	var kept []*plan.Plan
	for _, r := range SeedRows() {
		if r.Slug != "dev" {
			kept = append(kept, r)
		}
	}
	if _, _, err := plan.Seed(db, kept); err != nil {
		t.Fatalf("re-seed without dev: %v", err)
	}

	p := plan.New(db)
	ok, err := p.Query().Filter("Slug=", "dev").Get()
	if err != nil || !ok {
		t.Fatalf("retired plan %q must stay resolvable: ok=%v err=%v", "dev", ok, err)
	}
	if p.Listed() {
		t.Fatalf("retired plan %q is still listed; archiving is what stops the sale", "dev")
	}
	if p.Licensing == nil || !slices.Equal(p.Licensing.Products, []string{"team"}) {
		t.Fatalf("retired plan %q licenses %v, want [team] — a retired tier must not lose its licences", "dev", p.Licensing)
	}
}

// TestRetiredTiersResolveLicensing is the migration, and it is the part that fixes
// the customers who are stranded RIGHT NOW.
//
// It reconstructs the production state exactly: a row archived back when Plan had
// no licensing field, so it carries none, for a tier the catalog has long since
// dropped. Persisting the block on new rows does nothing for this row — only the
// backfill does. Without it the row licenses nothing and a paying subscriber is
// refused the product they bought.
func TestRetiredTiersResolveLicensing(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := plan.AuthorityDB(c)

	// The world as it exists today: archived, licensing-less rows for tiers the
	// catalog retired at @hanzo/plans 1.4.5.
	for _, slug := range []string{"plus", "team-max", "custom", "developer"} {
		p := plan.New(db)
		p.Slug = slug
		p.Category = "personal"
		p.Price = 10000
		p.Status = plan.StatusArchived
		p.Managed = true
		if err := p.Create(); err != nil {
			t.Fatalf("create archived %q: %v", slug, err)
		}
	}

	if _, _, err := SeedPlans(c); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// plus and team-max each licensed "team" when they were last on sale.
	for _, slug := range []string{"plus", "team-max"} {
		p := plan.New(db)
		ok, err := p.Query().Filter("Slug=", slug).Get()
		if err != nil || !ok {
			t.Fatalf("archived plan %q missing: ok=%v err=%v", slug, ok, err)
		}
		if p.Licensing == nil {
			t.Errorf("archived plan %q carries no licensing; a subscriber still paying for it is refused every product it licensed", slug)
			continue
		}
		if !slices.Equal(p.Licensing.Products, []string{"team"}) {
			t.Errorf("archived plan %q licenses %v, want [team]", slug, p.Licensing.Products)
		}
	}

	// custom and developer licensed NOTHING when they were last on sale, so no
	// block is the correct answer. The backfill must not invent a grant.
	for _, slug := range []string{"custom", "developer"} {
		p := plan.New(db)
		ok, err := p.Query().Filter("Slug=", slug).Get()
		if err != nil || !ok {
			t.Fatalf("archived plan %q missing: ok=%v err=%v", slug, ok, err)
		}
		if p.Licensing != nil {
			t.Errorf("archived plan %q licenses %v, want none — the backfill must never fabricate a grant", slug, p.Licensing.Products)
		}
	}

	// Archiving is untouched by the backfill: these tiers are still not for sale.
	p := plan.New(db)
	if ok, _ := p.Query().Filter("Slug=", "plus").Get(); ok && p.Listed() {
		t.Errorf("backfilled plan %q became listed; restoring a licence must never put a retired tier back on sale", "plus")
	}
}

// TestBackfillIsIdempotent: the backfill runs on every boot, so a second run must
// write nothing, and it must never overwrite a licensing block the catalog or an
// admin has since supplied.
func TestBackfillIsIdempotent(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := plan.AuthorityDB(c)

	p := plan.New(db)
	p.Slug = "plus"
	p.Category = "personal"
	p.Status = plan.StatusArchived
	if err := p.Create(); err != nil {
		t.Fatalf("create: %v", err)
	}

	first, err := plan.Backfill(db, retired)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if first != 1 {
		t.Fatalf("first backfill filled %d rows, want 1", first)
	}
	second, err := plan.Backfill(db, retired)
	if err != nil {
		t.Fatalf("re-backfill: %v", err)
	}
	if second != 0 {
		t.Fatalf("re-backfill filled %d rows, want 0 (idempotent)", second)
	}

	// An admin-supplied block wins: the migration restores a missing fact, it does
	// not re-decide one that is already recorded.
	q := plan.New(db)
	if ok, _ := q.Query().Filter("Slug=", "team-max").Get(); ok {
		t.Fatalf("team-max should not exist in this fixture")
	}
	q.Slug = "team-max"
	q.Category = "team"
	q.Status = plan.StatusArchived
	q.Licensing = &plan.Licensing{Products: []string{"engine"}}
	if err := q.Create(); err != nil {
		t.Fatalf("create team-max: %v", err)
	}
	if _, err := plan.Backfill(db, retired); err != nil {
		t.Fatalf("backfill over existing: %v", err)
	}
	r := plan.New(db)
	if ok, _ := r.Query().Filter("Slug=", "team-max").Get(); !ok {
		t.Fatalf("team-max missing")
	}
	if r.Licensing == nil || !slices.Equal(r.Licensing.Products, []string{"engine"}) {
		t.Errorf("backfill overwrote an existing licensing block: got %v, want [engine]", r.Licensing)
	}
}

// TestBackfillSkipsARevivedSlug: a retired name that is on sale again must not
// inherit the retired tier's grants.
//
// The backfill matched on slug alone, and it runs on every boot from SeedPlans.
// So the moment a retired slug is published again — for any reason, by anyone —
// the boot seed would write the OLD tier's licensing onto the new row, and keep
// writing it. `plus` was a $100/mo personal tier licensing the `team` product;
// republished at any price with no licensing of its own, it would silently
// acquire that grant, and a grant is the half of a plan that costs the platform
// rather than the tenant.
//
// The reuse itself stays a mistake this cannot fix: an archived row is kept so
// that invoices and renewals which recorded the slug still price from it, and
// aiming the name at a different product rewrites what every one of those
// records means. This only stops the mistake from also minting entitlement.
func TestBackfillSkipsARevivedSlug(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := plan.AuthorityDB(c)

	// `plus` is in the retired map, carrying licensing.product_ids ["team"].
	if retired["plus"] == nil {
		t.Fatal("fixture assumes `plus` is in the retired map; it is not, so this test proves nothing")
	}

	// On sale again, licensing nothing of its own.
	p := plan.New(db)
	p.Slug = "plus"
	p.Category = "personal"
	p.Status = plan.StatusActive
	if err := p.Create(); err != nil {
		t.Fatalf("create: %v", err)
	}
	if !p.Listed() {
		t.Fatal("fixture row is not listed, so it does not exercise the guard")
	}

	filled, err := plan.Backfill(db, retired)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if filled != 0 {
		t.Errorf("backfill filled %d rows, want 0 — it wrote to a tier that is on sale", filled)
	}

	got := plan.New(db)
	if ok, _ := got.Query().Filter("Slug=", "plus").Get(); !ok {
		t.Fatal("row vanished")
	}
	if got.Licensing != nil {
		t.Errorf("a listed %q was granted %v by the boot seed — the retired tier's terms, "+
			"applied to a product that merely reuses its name", "plus", got.Licensing.Products)
	}
}
