package catalogentry

import (
	"strings"
	"testing"
)

// clientProducts is the exact set of catalog rows that CONSUME the API rather
// than serve one — the CLI, the SDKs, the API reference, the IDE, the desktop
// app and the two web surfaces.
//
// It is the SECOND WITNESS to the seed, and that is its whole job. All seven
// once carried an apiPath, and since none of them serves a route, all seven of
// those paths were dead links the catalog advertised as "enabled". Marking a row
// a client now costs a deliberate edit here too, next to the reason.
var clientProducts = map[string]bool{
	"api":     true,
	"cli":     true,
	"sdks":    true,
	"ide":     true,
	"desktop": true,
	"console": true,
	"studio":  true,
}

func TestSeedGivesEveryClientNoApiPath(t *testing.T) {
	rows, err := HanzoSeedRows()
	if err != nil {
		t.Fatalf("HanzoSeedRows: %v", err)
	}
	seen := map[string]bool{}
	for _, r := range rows {
		switch r.Kind {
		case KindClient:
			seen[r.Slug] = true
			if !clientProducts[r.Slug] {
				t.Errorf("%s is seeded a client but is not in the client table — one side moved alone", r.Slug)
			}
			if r.ApiPath != "" || r.ApiRoute != "" {
				t.Errorf("%s is a client yet advertises apiPath %q / apiRoute %q — it consumes the API and has no route of its own, so any path on it is a dead link",
					r.Slug, r.ApiPath, r.ApiRoute)
			}
		case KindService, "":
			if clientProducts[r.Slug] {
				t.Errorf("%s is a client but is seeded a service — it will be judged by an apiPath it can never have", r.Slug)
			}
			if !strings.HasPrefix(r.ApiPath, "/v1/") {
				t.Errorf("%s: apiPath %q is not /v1-prefixed — a service is reached at a real route", r.Slug, r.ApiPath)
			}
		default:
			t.Errorf("%s: kind %q is neither %q nor %q", r.Slug, r.Kind, KindService, KindClient)
		}
	}
	for slug := range clientProducts {
		if !seen[slug] {
			t.Errorf("%s is a client but no seed row says so — it stays counted against an apiPath", slug)
		}
	}
}

// An apiPath and its host-qualified twin are two spellings of one route. Left to
// drift they disagree, and a reader has no way to tell which one lies.
func TestSeedApiRouteAgreesWithApiPath(t *testing.T) {
	rows, err := HanzoSeedRows()
	if err != nil {
		t.Fatalf("HanzoSeedRows: %v", err)
	}
	for _, r := range rows {
		want := ""
		if r.ApiPath != "" {
			want = "api.hanzo.ai" + r.ApiPath
		}
		if r.ApiRoute != want {
			t.Errorf("%s: apiRoute %q contradicts apiPath %q (want %q)", r.Slug, r.ApiRoute, r.ApiPath, want)
		}
	}
}

// billedRetail is what the enso service CHARGES, per MTok, transcribed from the
// deployed billing catalog (hanzoai/zen catalog-enso.yaml, mounted through
// universe charts/app/values/enso/enso.yaml). Production has billed these since
// the 2026-07-22/23 reprice; they reconcile exactly against 1,296 rows of
// hanzo.cloud_usage.
//
// This table is the SECOND WITNESS to the seed, and that is its whole job. The
// bug it exists to stop is not a typo — it is someone changing the published
// price on its own, which is exactly how the estate came to advertise 20/60
// while charging 4/20, and to advertise 3/12 BELOW what it charged. Editing the
// seed now costs a deliberate edit here too, next to the reason.
//
// If enso reprices, BOTH sides move together, in one commit, and the numbers
// come from the billing catalog — never from marketing copy, never from a docs
// page, never from the pre-reprice figures (20/60, 2/6, 40/120) which are stale
// and must not be restored.
var billedRetail = map[string]struct{ in, out string }{
	"enso":        {"4.00", "20.00"},
	"enso-flash":  {"2.00", "4.00"},
	"enso-ultra":  {"5.00", "25.00"},
	"enso-vl":     {"2.28", "9.60"},
	"enso-vl-pro": {"9.00", "45.00"},
}

func TestEnsoSeedPublishesTheBilledPrice(t *testing.T) {
	rows, err := EnsoSeedRows()
	if err != nil {
		t.Fatalf("EnsoSeedRows: %v", err)
	}
	if len(rows) != len(billedRetail) {
		t.Fatalf("seed has %d SKUs, billed table has %d — a SKU was added or dropped on one side only",
			len(rows), len(billedRetail))
	}
	for _, r := range rows {
		want, ok := billedRetail[r.Slug]
		if !ok {
			t.Errorf("%s: seeded but not in the billed table — every seeded SKU must state what it charges", r.Slug)
			continue
		}
		got := map[string]string{}
		for _, rate := range r.Rates {
			if rate.Unit != UnitMTok {
				t.Errorf("%s: rate %q has unit %q, want %q — a token price is per MTok", r.Slug, rate.Key, rate.Unit, UnitMTok)
			}
			// MaxContext 0 means "all contexts". Enso has no rung ladder, and
			// TokenCostCents only reads the 0 rung, so a non-zero here would make
			// the price invisible to every reader.
			if rate.MaxContext != 0 {
				t.Errorf("%s: rate %q has MaxContext %d, want 0 — enso bills one rung per SKU",
					r.Slug, rate.Key, rate.MaxContext)
			}
			got[rate.Key] = rate.Price
		}
		if got[RateIn] != want.in {
			t.Errorf("%s input: seed publishes %q, enso bills %q — published price must EQUAL billed price",
				r.Slug, got[RateIn], want.in)
		}
		if got[RateOut] != want.out {
			t.Errorf("%s output: seed publishes %q, enso bills %q — published price must EQUAL billed price",
				r.Slug, got[RateOut], want.out)
		}
	}
}

// A listed model with no price is a billing hole: it is selectable and there is
// no number to charge. Published rows must carry both components, priced.
func TestEnsoSeedNeverPublishesAnUnpricedModel(t *testing.T) {
	rows, err := EnsoSeedRows()
	if err != nil {
		t.Fatalf("EnsoSeedRows: %v", err)
	}
	for _, r := range rows {
		if !r.Published {
			continue
		}
		for _, key := range []string{RateIn, RateOut} {
			found := false
			for _, rate := range r.Rates {
				if rate.Key == key && rate.Price != "" {
					found = true
				}
			}
			if !found {
				t.Errorf("%s is published with no %s price — a selectable model with no rate is a billing hole", r.Slug, key)
			}
		}
	}
}

// The vision engines are reached only through the family's vision_fallback and
// are marked "NEVER listed SKUs" in the billing catalog. Seeding them published
// would put two internal engines on the public price list.
func TestEnsoSeedKeepsTheVisionEnginesUnlisted(t *testing.T) {
	rows, err := EnsoSeedRows()
	if err != nil {
		t.Fatalf("EnsoSeedRows: %v", err)
	}
	internal := map[string]bool{"enso-vl": true, "enso-vl-pro": true}
	for _, r := range rows {
		if internal[r.Slug] && r.Published {
			t.Errorf("%s is seeded published — it is an internal vision engine, never a listed SKU", r.Slug)
		}
		if !internal[r.Slug] && !r.Published {
			t.Errorf("%s is seeded unpublished — the three public SKUs must be listed for discovery", r.Slug)
		}
	}
}
