package catalogentry

import (
	"testing"

	"github.com/hanzoai/commerce/util/test/ae"
)

// The failure a rename invites is silence. scoped() keeps only entries whose
// Category is in canonicalCategories, so retiring a label does not move the rows
// that carry it — it deletes them from every surface at once, with nothing
// logged and nothing 500ing. Six products left the site the last time this shape
// went unhandled, so this pins the whole round trip: the store still holds the
// old label, Rename moves it, and the projection carries the same six products
// under the new one.
func TestRename_MovesRetiredCategoriesAndKeepsTheProducts(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	if _, err := Seed(db); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Age the store to what production holds: rows created under the old label,
	// before the taxonomy renamed it.
	want := []string{"projects", "environments", "builds", "registry", "releases", "pipelines"}
	for _, slug := range want {
		e := New(db)
		if ok, err := e.Query().Filter("Slug=", slug).Get(); err != nil || !ok {
			t.Fatalf("%s row: ok=%v err=%v", slug, ok, err)
		}
		e.Category = "Platform"
		if err := e.Update(); err != nil {
			t.Fatalf("age %s: %v", slug, err)
		}
	}

	// Aged, they are invisible — which is the defect, stated before the fix.
	before, err := Project(db, "hanzo")
	if err != nil {
		t.Fatalf("project before: %v", err)
	}
	for _, p := range before.Products {
		for _, slug := range want {
			if p.ID == slug {
				t.Fatalf("%s is projected while it names a retired category — the drop this test exists to catch is not happening, so it proves nothing", slug)
			}
		}
	}

	moved, err := Rename(db)
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if moved != len(want) {
		t.Fatalf("moved %d rows, want %d", moved, len(want))
	}

	after, err := Project(db, "hanzo")
	if err != nil {
		t.Fatalf("project after: %v", err)
	}
	got := map[string]string{}
	for _, p := range after.Products {
		got[p.ID] = p.Category
	}
	for _, slug := range want {
		if got[slug] != "Infrastructure" {
			t.Errorf("%s category = %q, want %q — a rename that drops the products it was meant to relabel is a deletion", slug, got[slug], "Infrastructure")
		}
	}

	// The taxonomy is still ten, and the renamed one still sits 7th: a rename is
	// not a reordering, and the nav reads this order.
	if len(after.Categories) != 10 {
		t.Fatalf("categories = %d, want 10", len(after.Categories))
	}
	if after.Categories[6].Label != "Infrastructure" || after.Categories[6].ID != "infrastructure" {
		t.Errorf("categories[6] = %+v, want label Infrastructure / id infrastructure", after.Categories[6])
	}
	for _, cat := range after.Categories {
		if cat.ID == "platform" {
			t.Errorf("the retired slug is still served — /products/platform would answer from the catalog and the redirect would never be reached")
		}
	}

	// Twice is once. A boot that keeps writing churns the store for nothing.
	again, err := Rename(db)
	if err != nil {
		t.Fatalf("re-rename: %v", err)
	}
	if again != 0 {
		t.Fatalf("re-rename wrote %d rows, want 0", again)
	}
}

// Which of the ten a product sits in is a merchandising decision made in
// admin.hanzo.ai. Rename may not touch one, and the property that guarantees it
// is that a canonical label never matches the retired map — not a special case,
// just the map missing the key.
func TestRename_LeavesACanonicalCategoryAlone(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	if _, err := Seed(db); err != nil {
		t.Fatalf("seed: %v", err)
	}

	e := New(db)
	if ok, err := e.Query().Filter("Slug=", "registry").Get(); err != nil || !ok {
		t.Fatalf("registry row: ok=%v err=%v", ok, err)
	}
	e.Category = "Dev" // an admin moved it
	if err := e.Update(); err != nil {
		t.Fatalf("age the row: %v", err)
	}

	if moved, err := Rename(db); err != nil || moved != 0 {
		t.Fatalf("rename moved %d rows (err %v), want 0 — an admin's category is not a stale label", moved, err)
	}

	got := New(db)
	if ok, err := got.Query().Filter("Slug=", "registry").Get(); err != nil || !ok {
		t.Fatalf("reread registry: ok=%v err=%v", ok, err)
	}
	if got.Category != "Dev" {
		t.Errorf("category = %q, want %q — a deploy that undoes admin.hanzo.ai is a deploy nobody can trust", got.Category, "Dev")
	}
}

// The seed is where a row is BORN, so it has to state the new label too.
// Otherwise every fresh environment births rows into a retired category and
// Rename spends its first boot repairing what the deploy just wrote.
func TestSeed_BirthsIntoTheCanonicalTaxonomy(t *testing.T) {
	rows, err := HanzoSeedRows()
	if err != nil {
		t.Fatalf("seed rows: %v", err)
	}
	canonical := map[string]bool{}
	for _, label := range canonicalCategories {
		canonical[label] = true
	}
	for _, r := range rows {
		if !canonical[r.Category] {
			t.Errorf("%s is seeded into %q, which the taxonomy does not carry — the row is created and never projected", r.ID, r.Category)
		}
	}
}
