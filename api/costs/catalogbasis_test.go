package costs

import (
	"testing"

	"github.com/hanzoai/commerce/models/catalogentry"
)

func mtok(slug, upstream, in, out string) *catalogentry.CatalogEntry {
	return &catalogentry.CatalogEntry{
		Slug: slug,
		Spec: &catalogentry.ModelSpec{Upstream: upstream},
		Rates: []catalogentry.Rate{
			{Key: catalogentry.RateIn, Unit: catalogentry.UnitMTok, Cost: in},
			{Key: catalogentry.RateOut, Unit: catalogentry.UnitMTok, Cost: out},
		},
	}
}

// The synced catalog is the source of truth; the hand-maintained table is only
// the fallback. If the table ever wins over a synced row, the whole change is
// pointless — this is the assertion that matters most.
func TestBasisPrefersCatalogOverTable(t *testing.T) {
	// gpt-4o is in costBasisTable at 250/1000. Give the catalog a different number.
	cat := indexRates([]*catalogentry.CatalogEntry{mtok("gpt-4o", "", "1.00", "2.00")})
	r, ok := basisRate(cat, "gpt-4o")
	if !ok {
		t.Fatal("catalog entry not found")
	}
	if r.InputCentsPerMTok != 100 || r.OutputCentsPerMTok != 200 {
		t.Fatalf("table won over the synced catalog: got %+v, want 100/200", r)
	}
}

// A model the sync does not cover must still be priced, by longest prefix,
// exactly as before.
func TestBasisFallsBackToTableForUncoveredModel(t *testing.T) {
	r, ok := basisRate(map[string]costRate{}, "gpt-4o-2024-08-06")
	if !ok {
		t.Fatal("longest-prefix fallback lost")
	}
	if r.InputCentsPerMTok != 250 {
		t.Fatalf("input = %v, want 250 from the curated table", r.InputCentsPerMTok)
	}
}

// A model neither source knows stays unknown, so the caller keeps counting it in
// UnknownModelTokens instead of booking it at zero cost.
func TestBasisUnknownStaysUnknown(t *testing.T) {
	if _, ok := basisRate(map[string]costRate{}, "no-such-model-anywhere"); ok {
		t.Fatal("an unknown model must not resolve — that books 0 cost and 100% margin")
	}
}

// A ledger row usually carries the unqualified id, so the tail after "/" is
// indexed too.
func TestIndexRatesJoinsOnSlugUpstreamAndTail(t *testing.T) {
	cat := indexRates([]*catalogentry.CatalogEntry{
		mtok("anthropic/claude-opus-5", "claude-opus-5-20260101", "3.00", "15.00"),
	})
	for _, id := range []string{"anthropic/claude-opus-5", "claude-opus-5", "claude-opus-5-20260101"} {
		if _, ok := cat[id]; !ok {
			t.Errorf("id %q did not resolve", id)
		}
	}
}

// Two models sharing a tail with DIFFERENT costs must not silently pick one.
// Dropping the ambiguous key falls through to the curated table, which is at
// least a deliberate number.
func TestIndexRatesDropsAmbiguousTail(t *testing.T) {
	cat := indexRates([]*catalogentry.CatalogEntry{
		mtok("vendor-a/mixtral", "", "1.00", "1.00"),
		mtok("vendor-b/mixtral", "", "9.00", "9.00"),
	})
	if _, ok := cat["mixtral"]; ok {
		t.Fatal("ambiguous tail resolved arbitrarily instead of being dropped")
	}
	// The unambiguous full slugs still work.
	if _, ok := cat["vendor-a/mixtral"]; !ok {
		t.Fatal("full slug lost to the ambiguity guard")
	}
}

// Same tail, same price is not ambiguous — nothing is lost by keeping it.
func TestIndexRatesKeepsAgreeingDuplicateTail(t *testing.T) {
	cat := indexRates([]*catalogentry.CatalogEntry{
		mtok("a/same", "", "2.00", "2.00"),
		mtok("b/same", "", "2.00", "2.00"),
	})
	if _, ok := cat["same"]; !ok {
		t.Fatal("agreeing duplicates should not trip the guard")
	}
}

// An entry that prices no tokens contributes nothing rather than a zero cost.
func TestIndexRatesSkipsEntriesWithoutTokenRates(t *testing.T) {
	e := &catalogentry.CatalogEntry{
		Slug:  "some-vm",
		Rates: []catalogentry.Rate{{Key: "", Unit: catalogentry.UnitMonth, Cost: "40.00"}},
	}
	if len(indexRates([]*catalogentry.CatalogEntry{e})) != 0 {
		t.Fatal("a month-priced row must not be indexed as a token cost")
	}
}
