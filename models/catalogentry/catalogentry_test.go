package catalogentry

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

func sysDB(c context.Context) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(c, "system"))
}

func TestSeed_IdempotentAndComplete(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	created, err := Seed(db)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	rows, _ := HanzoSeedRows()
	if len(rows) == 0 {
		t.Fatal("embedded seed is empty")
	}
	if created != len(rows) {
		t.Fatalf("first seed created %d, want %d (all embedded rows)", created, len(rows))
	}

	// Re-seed is a no-op (idempotent, never clobbers CMS edits).
	created2, err := Seed(db)
	if err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	if created2 != 0 {
		t.Fatalf("re-seed created %d, want 0 (idempotent)", created2)
	}
}

func TestProject_HanzoBrandScopedAndOrdered(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	if _, err := Seed(db); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cat, err := Project(db, "hanzo")
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if cat.Brand != "hanzo" {
		t.Fatalf("brand = %q, want hanzo", cat.Brand)
	}
	// Hanzo surfaces all 13 canonical categories.
	if len(cat.Categories) != 13 {
		t.Fatalf("hanzo categories = %d, want 13", len(cat.Categories))
	}
	if cat.Categories[0].Label != "AI" || cat.Categories[0].ID != "ai" {
		t.Fatalf("first category = %+v, want {ai, AI, 0}", cat.Categories[0])
	}
	if len(cat.Products) == 0 {
		t.Fatal("projection has no products")
	}

	// Products are ordered by (category rank, entry order). Verify category
	// rank is non-decreasing across the product list.
	rank := map[string]int{}
	for _, c := range cat.Categories {
		rank[c.Label] = c.Order
	}
	prev := -1
	for _, p := range cat.Products {
		r, ok := rank[p.Category]
		if !ok {
			t.Fatalf("product %q in category %q not surfaced by hanzo", p.Slug, p.Category)
		}
		if r < prev {
			t.Fatalf("products not category-ordered: %q (rank %d) after rank %d", p.Slug, r, prev)
		}
		prev = r
		// Every product carries the presentation keys the client needs.
		if p.IconKey == "" {
			t.Fatalf("product %q missing iconKey", p.Slug)
		}
		if p.ID != p.Slug {
			t.Fatalf("product id %q != slug %q", p.ID, p.Slug)
		}
	}
}

func TestProject_LuxBrandScopeExcludesNonLuxCategories(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	// Seed hanzo entries (all AI/Compute/... categories) and one lux entry.
	if _, err := Seed(db); err != nil {
		t.Fatalf("seed: %v", err)
	}
	lux := New(db)
	lux.Slug = "lux-net"
	lux.Brand = "lux"
	lux.Name = "Network"
	lux.Category = "Network"
	lux.IconKey = "Network"
	lux.Status = StatusEnabled
	if err := lux.Create(); err != nil {
		t.Fatalf("create lux entry: %v", err)
	}

	cat, err := Project(db, "lux")
	if err != nil {
		t.Fatalf("project lux: %v", err)
	}
	// Lux surfaces only its 5 categories.
	if len(cat.Categories) != 5 {
		t.Fatalf("lux categories = %d, want 5", len(cat.Categories))
	}
	// The hanzo-brand entries must NOT appear in the lux projection (brand filter).
	for _, p := range cat.Products {
		if p.Slug != "lux-net" {
			t.Fatalf("lux projection leaked non-lux product %q", p.Slug)
		}
	}
	if len(cat.Products) != 1 {
		t.Fatalf("lux projection products = %d, want 1", len(cat.Products))
	}
}

func TestProject_UnpublishedExcluded(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	e := New(db)
	e.Slug = "hidden"
	e.Brand = "hanzo"
	e.Name = "Hidden"
	e.Category = "AI"
	e.IconKey = "Lock"
	e.Published = false
	if err := e.Create(); err != nil {
		t.Fatalf("create: %v", err)
	}

	cat, err := Project(db, "hanzo")
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	for _, p := range cat.Products {
		if p.Slug == "hidden" {
			t.Fatal("unpublished entry appeared in the public projection")
		}
	}
}
