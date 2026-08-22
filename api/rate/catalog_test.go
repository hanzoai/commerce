// Copyright © 2026 Hanzo AI. MIT License.

package rate

import (
	"strings"
	"testing"

	"github.com/hanzoai/commerce/models/rate"
)

// THE SEED MUST NOT REPRICE ANYTHING ON THE DAY IT LANDS. Every row here
// replaces a compiled constant that is charging today, so the number has to be
// the same number — in nano-dollars, which is where a unit conversion goes wrong
// silently. $0.08 written as 80_000 instead of 80_000_000 is a thousand-fold
// underprice that no test of "is there a row" would catch.
func TestSeededRatesEqualTheConstantsTheyReplace(t *testing.T) {
	// (slug, nano) taken from the constant each one replaces:
	//   storage   apps/provisioning/dedicated.go  defaultStoragePriceCents = 8   ($0.08/GB-month)
	//   translate apps/translate/engine.go        defaultBulkPriceUUSDPer1kChars = 20 (20 µUSD)
	//   risk      apps/risk/typed.go              defaultScreenUUSD = 100        (100 µUSD)
	want := map[string]int64{
		"storage/block-gb-month": 80_000_000,
		"translate/bulk-chars":   20_000,
		"risk/screen":            100_000,
	}

	got := map[string]int64{}
	for _, r := range Seeded() {
		r.Bind()
		got[r.Slug] = r.Rate
	}
	for slug, nano := range want {
		g, ok := got[slug]
		if !ok {
			t.Errorf("%s is not seeded, so the meter has no row and an operator cannot "+
				"move its price at all", slug)
			continue
		}
		if g != nano {
			t.Errorf("%s seeded at %d nano, want %d — the seed reprices live work on the "+
				"day it lands, which is the one thing it must not do", slug, g, nano)
		}
	}
	if len(got) != len(want) {
		t.Errorf("catalog has %d meters, expected %d — a new one must arrive with the "+
			"constant it replaces, or it prices something at a number nobody chose",
			len(got), len(want))
	}
}

// A row with no unit prices nothing: the rate is "per one of these", so without
// it the number is unreadable and an editor cannot say what it is charging for.
func TestEverySeededRateStatesWhatOneUnitIs(t *testing.T) {
	for _, r := range Seeded() {
		r.Bind()
		if strings.TrimSpace(r.Unit) == "" {
			t.Errorf("%s declares no Unit — its rate is a number with no meaning", r.Slug)
		}
		if strings.TrimSpace(r.Label) == "" {
			t.Errorf("%s declares no Label, so admin.hanzo.ai lists it as a slug", r.Slug)
		}
		if r.Currency == "" {
			t.Errorf("%s declares no Currency", r.Slug)
		}
		if r.Rate <= 0 {
			t.Errorf("%s is seeded at %d — a zero rate makes metered work free and is "+
				"indistinguishable from an unconfigured one", r.Slug, r.Rate)
		}
	}
}

// AI AND TOOLS ARE DELIBERATELY ABSENT. Both already have an authority — a
// ModelRoute row for a model, a marketplace listing for a tool — and both are
// read ahead of anything else. A row here would be a SECOND answer to a question
// that already has one, on the money path, with no stated precedence.
func TestTheCatalogDoesNotDuplicateAnExistingAuthority(t *testing.T) {
	for _, r := range Seeded() {
		switch r.Product {
		case "ai":
			t.Errorf("%s/%s prices a model, which ModelRoute.InputPrice/OutputPrice already "+
				"does — and that row is read first, so this one would be edited and ignored",
				r.Product, r.Meter)
		case "tools":
			t.Errorf("%s/%s prices a tool, which its marketplace listing already does — set "+
				"by the publisher who is paid for it, not by a platform rate card",
				r.Product, r.Meter)
		}
	}
}

// The catalog is a valid seed input: Seed keys on the slug, so a duplicate or an
// unbound part silently reconciles one row twice.
func TestTheCatalogIsAValidSeedInput(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range Seeded() {
		if r.Product == "" || r.Meter == "" {
			t.Errorf("a row is missing a part (product=%q meter=%q); Bind derives the "+
				"identity from both", r.Product, r.Meter)
			continue
		}
		r.Bind()
		if seen[r.Slug] {
			t.Errorf("%s appears twice — the seed would reconcile one row against two "+
				"published values and the last one silently wins", r.Slug)
		}
		seen[r.Slug] = true
	}
}

var _ = rate.Rate{}
