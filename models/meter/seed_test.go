// Copyright © 2026 Hanzo AI. MIT License.

package meter

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

func sysDB(c context.Context) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(c, Namespace))
}

func rows() []*Meter {
	return []*Meter{
		{Product: "ai", Meter: "zen-coder", Label: "Zen Coder", Unit: PerMTok, Rate: 250_000, Currency: "usd", Source: "zen-gateway"},
		{Product: "ai", Meter: "zen-embedding", Label: "Zen Embedding", Unit: PerMTok, Rate: 20_000, Currency: "usd", Source: "zen-gateway"},
		{Product: "tools", Meter: "websearch", Label: "Web Search", Unit: PerCall, Rate: 5_000_000, Currency: "usd"},
		{Product: "storage", Meter: "standard", Label: "Standard", Unit: PerGiBMonth, Rate: 23_000_000, Currency: "usd"},
	}
}

func TestSeedCreatesEveryRateThenWritesNothing(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	created, corrected, err := Seed(db, rows())
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if created != 4 || corrected != 0 {
		t.Fatalf("first seed created=%d corrected=%d, want 4/0", created, corrected)
	}

	// Idempotent: a boot is not a write. If this reports corrections, every
	// restart rewrites every rate and the audit trail becomes noise.
	created, corrected, err = Seed(db, rows())
	if err != nil {
		t.Fatalf("reseed: %v", err)
	}
	if created != 0 || corrected != 0 {
		t.Fatalf("second seed created=%d corrected=%d, want 0/0", created, corrected)
	}
}

// THE PROPERTY THE AUTHORITY EXISTS FOR.
//
// If a reseed reverted an operator's price, an editable catalog would be worse
// than a hardcoded one: you would change a rate in admin.hanzo.ai, watch it take
// effect, and find it silently gone after the next restart with nothing saying
// what happened. A hardcoded rate at least stays where you put it.
func TestAnAdminEditSurvivesEveryLaterSeed(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	if _, _, err := Seed(db, rows()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	edited := New(db)
	ok, err := edited.Query().Filter("Slug=", "ai/zen-coder").Get()
	if err != nil || !ok {
		t.Fatalf("read back: ok=%v err=%v", ok, err)
	}
	edited.Rate = 999_000
	edited.AdminEdited = true
	if err := edited.Update(); err != nil {
		t.Fatalf("edit: %v", err)
	}

	// The file still says 250_000. The row must not move.
	if _, _, err := Seed(db, rows()); err != nil {
		t.Fatalf("reseed: %v", err)
	}
	after := New(db)
	if ok, err := after.Query().Filter("Slug=", "ai/zen-coder").Get(); err != nil || !ok {
		t.Fatalf("read back after reseed: ok=%v err=%v", ok, err)
	}
	if after.Rate != 999_000 {
		t.Errorf("rate = %d after reseed, want the operator's 999000 — the seed reverted an admin edit", after.Rate)
	}

	// And a row nobody touched still tracks the file.
	other := New(db)
	if ok, _ := other.Query().Filter("Slug=", "ai/zen-embedding").Get(); ok && other.Rate != 20_000 {
		t.Errorf("untouched rate = %d, want 20000 — an unedited row must follow the published value", other.Rate)
	}
}

// A rate is identified by (Product, Key), never by Key alone: the same model
// name can be metered by two products at two different rates, and collapsing
// them would let one product's price overwrite another's.
func TestTheSameKeyUnderTwoProductsIsTwoRates(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	both := []*Meter{
		{Product: "ai", Meter: "shared", Unit: PerMTok, Rate: 100, Currency: "usd"},
		{Product: "tools", Meter: "shared", Unit: PerCall, Rate: 900, Currency: "usd"},
	}
	created, _, err := Seed(db, both)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if created != 2 {
		t.Fatalf("created=%d, want 2 — (product,key) is the identity, not key", created)
	}
}

// A rate with no identity cannot be reconciled against anything, so it is
// skipped rather than inserted as an anonymous row nothing can ever match.
func TestARateWithNoIdentityIsRefused(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	created, corrected, err := Seed(db, []*Meter{
		{Product: "", Meter: "orphan", Unit: PerCall, Rate: 1},
		{Product: "ai", Meter: "", Unit: PerMTok, Rate: 1},
		nil,
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if created != 0 || corrected != 0 {
		t.Errorf("created=%d corrected=%d, want 0/0", created, corrected)
	}
}
