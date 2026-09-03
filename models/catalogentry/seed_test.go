package catalogentry

import (
	"github.com/hanzoai/commerce/util/test/ae"
	"strings"
	"testing"
)

// clientProducts is the exact set of catalog rows that CONSUME the API rather
// than serve one — the CLI, the SDKs, the API reference, the desktop app, the two
// web surfaces, and Edge.
//
// It is the SECOND WITNESS to the seed, and that is its whole job. All of them
// once carried an apiPath, and since none of them serves a route, every one of
// those paths was a dead link the catalog advertised as "enabled". Marking a row
// a client now costs a deliberate edit here too, next to the reason.
//
// Edge belongs here on the fleet's own argument, not a guess: manifest/apps.go
// gives "/v1/edge" to nobody because hanzoai/edge is the on-device inference
// runtime, a binary a customer runs on their own machine. It calls this API; it
// is not reached through it, and /v1/edge 404s at every depth on purpose.
//
// The IDE does NOT belong here, though it was once written down as if it did. The
// code-intelligence engine behind it is served — search, symbols and LSP at
// /v1/code — so calling it a client hid a working API instead of revealing a
// missing one.
var clientProducts = map[string]bool{
	"api":     true,
	"cli":     true,
	"sdks":    true,
	"desktop": true,
	"console": true,
	"studio":  true,
	"edge":    true,
}

// pendingProducts is the exact set of catalog rows that would serve an API and do
// not yet — real products with a console surface that api.hanzo.ai does not front.
// Each was searched for before it was written down here, and the search is the
// point: "missing" usually means MOVED, and eleven of these thirteen sibling rows
// turned out to be exactly that.
//
//   - containers — no /v1/container* anywhere. The container-shaped surfaces the
//     fleet does serve are already other products (Functions, Applications) or a
//     differently-named one (/v1/sandboxes), so pointing here at any of them
//     would dress up a product that is not shipped.
//   - cdn        — no prefix. Purging is a VERB on other products
//     (/v1/projects/:slug/purge, /v1/cloudflare/zones/:zone/purge), never a noun.
//   - hsm        — no prefix. /v1/kms is the key service and it is MPC-rooted,
//     not hardware-rooted; lending it to HSM would sell one as the other.
//   - mpc        — apps/mpc is a CLIENT LIBRARY for the separate luxfi/mpc ring.
//     Nothing in this fleet serves it. MPC custody is REACHED through /v1/wallets,
//     which is the Wallets product, not this one.
//   - attestations — no prefix, no handler, nothing HTTP-addressable.
//
// The three DAO rows are pending and not client, which is the same distinction
// edge turns on read the other way: a client CONSUMES this API and serves none,
// and each of these serves one that is not ours. luxdao/api is the offchain API
// behind Governance and DaoWork — proposals, comments, temperature checks, SIWE —
// so luxdao/app calls THAT, never api.hanzo.ai. Calling them clients would claim
// a dependency on this API that does not exist.
//
//   - safe — the DAO treasury multisig (luxdao/contracts). The fleet DOES serve
//     two paths spelled safe and NEITHER is this product: /v1/captable/safes and
//     /v1/company/fundraise/safe are the fundraising instrument, a simple
//     agreement for future equity, which those operations say in their own prose.
//     One noun, two meanings — pointing here at either would sell a cap-table
//     note as a multisig, which is the dressing-up the address gate refuses.
//   - governance — no /v1 segment resembles it, and none of the 193 the fleet
//     publishes is a vote, proposal or DAO surface. Served at lux.vote.
//   - daowork — the bounty surface of that same app, likewise unfronted here.
var pendingProducts = map[string]bool{
	// vpc and load-balancer were served by cloud's apps/do, the DigitalOcean
	// plane, and that app is gone — so /v1/vpcs and /v1/balancers are addresses
	// the fleet no longer answers. The products are real; the API is not here.
	"vpc":           true,
	"load-balancer": true,
	"containers":    true,
	"cdn":           true,
	"hsm":           true,
	"mpc":           true,
	"attestations":  true,
	"safe":          true,
	"governance":    true,
	"daowork":       true,
}

func TestSeedGivesEveryClientNoApiPath(t *testing.T) {
	rows, err := HanzoSeedRows()
	if err != nil {
		t.Fatalf("HanzoSeedRows: %v", err)
	}
	seen := map[string]bool{}
	for _, r := range rows {
		switch r.Kind {
		case KindClient, KindPending:
			seen[r.Slug] = true
			table, other := clientProducts, pendingProducts
			if r.Kind == KindPending {
				table, other = pendingProducts, clientProducts
			}
			if !table[r.Slug] {
				t.Errorf("%s is seeded %s but is not in the %s table — one side moved alone", r.Slug, r.Kind, r.Kind)
			}
			if other[r.Slug] {
				t.Errorf("%s is seeded %s while the other table also claims it — a row has one kind", r.Slug, r.Kind)
			}
			if r.ApiPath != "" || r.ApiRoute != "" {
				t.Errorf("%s is %s yet advertises apiPath %q / apiRoute %q — it has no route of its own, so any path on it is a dead link",
					r.Slug, r.Kind, r.ApiPath, r.ApiRoute)
			}
		case KindService, "":
			if clientProducts[r.Slug] || pendingProducts[r.Slug] {
				t.Errorf("%s serves no API but is seeded a service — it will be judged by an apiPath it does not have", r.Slug)
			}
			if !strings.HasPrefix(r.ApiPath, "/v1/") {
				t.Errorf("%s: apiPath %q is not /v1-prefixed — a service is reached at a real route", r.Slug, r.ApiPath)
			}
		default:
			t.Errorf("%s: kind %q is none of %q, %q, %q", r.Slug, r.Kind, KindService, KindClient, KindPending)
		}
	}
	for _, table := range []map[string]bool{clientProducts, pendingProducts} {
		for slug := range table {
			if !seen[slug] {
				t.Errorf("%s serves no API but no seed row says so — it stays counted against an apiPath", slug)
			}
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

// TestEnsoSeedIsNotBlockedByAForeignRow is the production defect, pinned.
//
// A deployment carried one enso-category row this seed does not contain
// (enso-free, created by another path). The old guard counted CATEGORY rows, saw
// one, and skipped — so enso, enso-flash and enso-ultra were never created and
// every paid tier answered as an unknown model while the catalog served a lone
// free tier. Nothing went red: the guard was doing exactly what it said.
func TestEnsoSeedIsNotBlockedByAForeignRow(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	// A row in the enso category that the seed does not contain.
	foreign := New(db)
	foreign.Slug = "enso-free"
	foreign.Name = "Enso Free"
	foreign.Category = CategoryEnso
	if err := foreign.Put(); err != nil {
		t.Fatalf("seeding the foreign row: %v", err)
	}

	created, err := SeedEnsoModelsIfEmpty(db)
	if err != nil {
		t.Fatalf("SeedEnsoModelsIfEmpty: %v", err)
	}
	if created == 0 {
		t.Fatal("a foreign enso row blocked the seed — the paid tiers would never exist")
	}
	for _, slug := range []string{"enso", "enso-flash", "enso-ultra"} {
		e := New(db)
		ok, qerr := e.Query().Filter("Slug=", slug).Get()
		if qerr != nil {
			t.Fatalf("query %s: %v", slug, qerr)
		}
		if !ok {
			t.Errorf("%s was not seeded", slug)
		}
	}
}

// TestEnsoSeedRespectsCuration is the half the guard exists for: once the seed
// HAS run, a deletion is authoritative and re-running must not resurrect it.
func TestEnsoSeedRespectsCuration(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := sysDB(c)

	if _, err := SeedEnsoModelsIfEmpty(db); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	// An admin retires one SKU.
	e := New(db)
	ok, err := e.Query().Filter("Slug=", "enso-ultra").Get()
	if err != nil || !ok {
		t.Fatalf("enso-ultra missing after the seed: ok=%v err=%v", ok, err)
	}
	if err := e.Delete(); err != nil {
		t.Fatalf("delete: %v", err)
	}

	created, err := SeedEnsoModelsIfEmpty(db)
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if created != 0 {
		t.Errorf("re-seeded %d rows after the seed had run — a deletion must stay deleted", created)
	}
}
