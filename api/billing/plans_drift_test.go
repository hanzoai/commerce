package billing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"testing"
)

// Step 0 drift guard. The vendored @hanzo/plans JSON (api/billing/plans/*.json)
// is embedded and is the seed source + charge fallback, so an accidental or
// malicious edit to a vendored plan file must fail CI. package.json is gitignored
// (not present in a fresh clone) — the pin is PinnedPlansVersion + the content
// digests of the two EMBEDDED files, recorded PER VERSION below.
//
// versionDigests is keyed by version, so (Red F4) a content change can't land
// without a conscious PinnedPlansVersion bump, and a version bump can't land
// without the matching re-vendored content: bumping the version selects a new map
// entry whose digests must equal the new bytes, and editing the bytes under the
// current version breaks the current entry.
var versionDigests = map[string]struct{ subscription, dns string }{
	"1.8.9": {
		subscription: "cc8481a800b3891d4df6a2eb5670e87d80c89b50a210b3eb4fe615eae55d7f5b",
		dns:          "620485fd5fcda4bb860021167f8f9c91a9b0dfe4dcc498d1b91cf8641bfcacbc",
	},
	"1.8.8": {
		subscription: "1cdca4fcd5606ff7cc5dec939f50ecf0e97830782fe65e441e96378848d2a3fa",
		dns:          "620485fd5fcda4bb860021167f8f9c91a9b0dfe4dcc498d1b91cf8641bfcacbc",
	},
	"1.8.6": {
		subscription: "7fa880b400e466154a41f8020abfa0012138b7c484010c5fa953346ec88f1feb",
		dns:          "620485fd5fcda4bb860021167f8f9c91a9b0dfe4dcc498d1b91cf8641bfcacbc",
	},
	"1.8.5": {
		subscription: "b70c7f759414c7ce2817b460a4c25e5104e22b724013c6d2db40119b481cf957",
		dns:          "620485fd5fcda4bb860021167f8f9c91a9b0dfe4dcc498d1b91cf8641bfcacbc",
	},
	"1.8.4": {
		subscription: "fdddcd25ba382c2963ebadf80a2e0d43066dd841ee57deb25065563a14f2d0ca",
		dns:          "620485fd5fcda4bb860021167f8f9c91a9b0dfe4dcc498d1b91cf8641bfcacbc",
	},
	"1.8.3": {
		subscription: "4e7cf986f1b48435cc79435820828b28a90fce0f9c4f04e9eb333ed4ecff8434",
		dns:          "620485fd5fcda4bb860021167f8f9c91a9b0dfe4dcc498d1b91cf8641bfcacbc",
	},
	"1.6.0": {
		subscription: "883bbdc5261feddc90f2782590a3c806e71c7bc40d017b5fa4ed77ddd2d8c1cb",
		dns:          "620485fd5fcda4bb860021167f8f9c91a9b0dfe4dcc498d1b91cf8641bfcacbc",
	},
	"1.5.0": {
		subscription: "f76c7b381c0a56127b566b060b34a998c077975eddeccd6255081c8d56981e34",
		dns:          "620485fd5fcda4bb860021167f8f9c91a9b0dfe4dcc498d1b91cf8641bfcacbc",
	},
	"1.4.20": {
		subscription: "f89507c4fdebf0df2f5c135281a6f89ae4b6bc1eb4f6aec178b804a795e87339",
		dns:          "620485fd5fcda4bb860021167f8f9c91a9b0dfe4dcc498d1b91cf8641bfcacbc",
	},
	"1.4.4": {
		subscription: "e490185e58b4e83d925eaf2dfd4778e28023655b610d0504b8058670bbdf2f79",
		dns:          "de7da2ab600268bdf5528b9ec1fd037bdbe8f9112f3755d80b5f93a4cbf1cd87",
	},
	"1.4.8": {
		subscription: "7affa8d6d75bf28fed1f014e96bffe25f08b6c0df008cdc24d375a3d3107b38d",
		dns:          "de7da2ab600268bdf5528b9ec1fd037bdbe8f9112f3755d80b5f93a4cbf1cd87",
	},
	"1.4.13": {
		subscription: "c511dfb34552d6adbff33a25aa72cf2ef68eec7bca7fbed41ab17057f58540b1",
		dns:          "de7da2ab600268bdf5528b9ec1fd037bdbe8f9112f3755d80b5f93a4cbf1cd87",
	},
	"1.4.16": {
		subscription: "3ca9e1b0c77abaa2ddcb3d2fb708750b8a3c34db96f475c09eb39764727fb94d",
		dns:          "de7da2ab600268bdf5528b9ec1fd037bdbe8f9112f3755d80b5f93a4cbf1cd87",
	},
	"1.4.18": {
		subscription: "ecb4357f7357f0182a1faa833598f79e45e4d41e9e9c2b7f67dd0ab5ce27b52d",
		dns:          "620485fd5fcda4bb860021167f8f9c91a9b0dfe4dcc498d1b91cf8641bfcacbc",
	},
}

func digest(t *testing.T, fs interface {
	ReadFile(string) ([]byte, error)
}, path string) string {
	t.Helper()
	b, err := fs.ReadFile(path)
	if err != nil {
		t.Fatalf("read embedded %s: %v", path, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// TestVendoredPlansMatchPinnedVersion fails if the embedded plan JSON drifts from
// the pinned @hanzo/plans version — the definitive vendored-dependency tripwire.
func TestVendoredPlansMatchPinnedVersion(t *testing.T) {
	want, ok := versionDigests[PinnedPlansVersion]
	if !ok {
		t.Fatalf("PinnedPlansVersion = %q has no digests in versionDigests; add its subscription+dns digests when re-vendoring", PinnedPlansVersion)
	}
	if got := digest(t, subscriptionJSON, "plans/subscription.json"); got != want.subscription {
		t.Fatalf("subscription.json drifted from @hanzo/plans@%s\n  got  %s\n  want %s\n(re-vendor AND bump PinnedPlansVersion + add a versionDigests entry together)", PinnedPlansVersion, got, want.subscription)
	}
	if got := digest(t, dnsJSON, "plans/dns.json"); got != want.dns {
		t.Fatalf("dns.json drifted from @hanzo/plans@%s\n  got  %s\n  want %s", PinnedPlansVersion, got, want.dns)
	}
}

// TestVendoredPlanPrices is a diagnostic price-canary: if a money-bearing plan's
// cents change, THIS test names which one (the digest test only says "drifted").
// These are the @hanzo/plans@1.8.9 cents — monthly, the annual price tag shown
// per month, and the year a yearly subscription is charged. contactSales plans
// are null-priced → stored as 0 + ContactSales (never a chargeable $0).
func TestVendoredPlanPrices(t *testing.T) {
	if got := len(catalog); got != 11 { // 8 subscription + 3 dns
		t.Fatalf("catalog = %d, want 11 (8 subscription + 3 dns)", got)
	}
	if got := len(dnsPlans); got != 3 {
		t.Fatalf("dnsPlans = %d, want 3", got)
	}
	bySlug := map[string]staticPlan{}
	for _, p := range catalog {
		bySlug[p.Slug] = p
	}
	// none is an annual price the catalog does not state: null on the wire, never $0.
	const none = -1
	type want struct {
		monthly, annual, year int64
		contactSales          bool
	}
	// A personal year is ten months ($200, $1,000, $2,000) and a team seat's is
	// $240 against $25 monthly; the annual tag is that year over twelve, rounded
	// to the cent. The catalog states the total because twelve rounded tags would
	// bill $200.04.
	cases := map[string]want{
		"free":           {0, 0, 0, false},           // a real $0 rung, not a null price
		"dev":            {2000, 1667, 20000, false}, // shown as Pro
		"max-5x":         {10000, 8333, 100000, false},
		"max-20x":        {20000, 16667, 200000, false},
		"team":           {2500, 2000, 24000, false}, // per seat
		"advisory":       {499900, none, 0, false},   // agency, monthly only
		"dedicated":      {999900, none, 0, false},
		"enterprise":     {0, none, 0, true}, // null price → 0 + contactSales
		"dns-free":       {0, 0, 0, false},
		"dns-pro":        {500, 400, 4800, false}, // dns states no total: a year is twelve annual months
		"dns-enterprise": {2500, 2000, 24000, false},
	}
	for slug, w := range cases {
		p, ok := bySlug[slug]
		if !ok {
			t.Fatalf("plan %q missing from embed", slug)
		}
		annual := int64(none)
		if p.PriceAnnual != nil {
			annual = *p.PriceAnnual
		}
		if p.Price != w.monthly || annual != w.annual || p.AnnualTotal != w.year {
			t.Errorf("plan %q price = %d/%d/%d cents, want %d/%d/%d (monthly/annual tag, %d for none/year)",
				slug, p.Price, annual, p.AnnualTotal, w.monthly, w.annual, none, w.year)
		}
		if p.ContactSales != w.contactSales {
			t.Errorf("plan %q contactSales = %v, want %v (free-vs-null distinction)", slug, p.ContactSales, w.contactSales)
		}
	}
}

// TestVendoredPlanRoster is the price canary's twin for what a plan may RUN.
// These are capacities, not allowances of runtime: they decide whether a
// customer may create their next agent, while the hours it then runs meter
// separately at the catalog's hourly rate. A number that drifts here refuses
// paying work or gives away concurrency — and the digest test only ever says
// "drifted", never which figure moved.
//
// It reads through AgentsIncluded/BotsIncluded rather than the raw field so the
// accessor cloud enforces with is the thing under test. A bare struct read
// would pass while the exported answer was wrong.
func TestVendoredPlanRoster(t *testing.T) {
	cases := map[string]struct{ agents, bots int }{
		"free":       {1, 0},  // one personal agent, no bot
		"dev":        {10, 0}, // Pro, the $20 tier
		"max-5x":     {10, 1}, // both Max multiples may run a resident bot
		"max-20x":    {10, 1},
		"max":        {10, 1}, // retired, and answered by max-5x for its holders
		"advisory":   {10, 1},
		"dedicated":  {-1, 1},
		"team":       {10, 0},
		"enterprise": {-1, -1}, // -1 is unlimited, as it is for maxMembers
	}
	for slug, w := range cases {
		agents, ok := AgentsIncluded(slug)
		if !ok {
			t.Errorf("plan %q publishes no agent roster — enforcement has nothing to read and a holder is refused their first agent", slug)
			continue
		}
		if agents != w.agents {
			t.Errorf("plan %q includes %d agents, want %d", slug, agents, w.agents)
		}
		bots, ok := BotsIncluded(slug)
		if !ok {
			t.Errorf("plan %q publishes no bot roster", slug)
			continue
		}
		if bots != w.bots {
			t.Errorf("plan %q includes %d bots, want %d", slug, bots, w.bots)
		}
	}
	// Silence must stay distinguishable from zero, or the accessor's whole
	// contract collapses into "unknown means refuse".
	if n, ok := AgentsIncluded("no-such-plan"); ok || n != 0 {
		t.Errorf("AgentsIncluded(unknown) = (%d, %v), want (0, false)", n, ok)
	}
}

// TestAYearIsStatedOrExact: a yearly subscription charges AnnualTotal, which the
// decoder takes from price_ref.recurring.annual_total_usd and, for a row that
// states none, as twelve of its annual price. That second reading is only honest
// when the annual price was never rounded — a whole-dollar month — so a row whose
// per-month tag carries cents MUST state its year, or the charge becomes twelve
// rounded months: $198.96 for a $199 year.
func TestAYearIsStatedOrExact(t *testing.T) {
	for _, f := range []struct {
		fs   interface{ ReadFile(string) ([]byte, error) }
		path string
	}{{subscriptionJSON, "plans/subscription.json"}, {dnsJSON, "plans/dns.json"}} {
		raw, err := f.fs.ReadFile(f.path)
		if err != nil {
			t.Fatalf("read %s: %v", f.path, err)
		}
		var rows []canonicalPlan
		if err := json.Unmarshal(raw, &rows); err != nil {
			t.Fatalf("decode %s: %v", f.path, err)
		}
		for _, r := range rows {
			if r.PriceAnnual == nil || *r.PriceAnnual <= 0 {
				continue
			}
			stated := r.PriceRef != nil && r.PriceRef.Recurring != nil && r.PriceRef.Recurring.AnnualTotalUSD != nil
			if !stated && *r.PriceAnnual != float64(int64(*r.PriceAnnual)) {
				t.Errorf("%s: %q shows $%.2f a month annually and states no year; twelve rounded months is not a price anyone set",
					f.path, r.ID, *r.PriceAnnual)
			}
		}
	}
}

// A retired slug must STAY retired in the embed. The embed is what a fresh
// database seeds from, so a tier that comes back here comes back on the pricing
// page of every new deployment — which is exactly the resurrection plan.Status
// prevents for rows that already exist, and cannot prevent for rows that do not.
func TestRetiredSlugsAreNotInTheEmbed(t *testing.T) {
	bySlug := map[string]bool{}
	for _, p := range catalog {
		bySlug[p.Slug] = true
	}
	for _, slug := range []string{
		// go ($9), pro ($49) and max ($99) came off the ladder; a renewal still
		// prices through its own snapshot, but the embed must not offer them for
		// sale again. max is answered by max-5x (its "replaces"), never listed.
		"go", "pro", "max",
		"developer", "plus", "team-max", "custom",
		"world-free", "world-pro", "world-team", "world-enterprise",
		"social-free", "social-pro", "social-team", "social-team-max", "social-enterprise",
	} {
		if bySlug[slug] {
			t.Errorf("retired plan %q is back in the embed", slug)
		}
	}
}

// TestVendoredAllotmentAmounts canaries what a plan gives away. No rung mints
// prepaid credit any more — the ledger holds only credit a buyer bought — and
// each paid rung instead COVERS AI usage a period (ai.included_cents, 75% of its
// price). coveredCents is the sum the plan-move gate scores, so a drift in either
// half changes who may move where without paying, and must fail here by name.
func TestVendoredAllotmentAmounts(t *testing.T) {
	for _, slug := range []string{"free", "dev", "max-5x", "max-20x", "max", "team", "advisory", "dedicated", "enterprise", "dns-pro", "go", "pro"} {
		if got := IncludedMonthlyCents(slug); got != 0 {
			t.Errorf("IncludedMonthlyCents(%q) = %d, want 0 — no rung mints credit", slug, got)
		}
	}
	for slug, cents := range map[string]int64{
		"free":       0,
		"dev":        1500,  // $15 of a $20 plan
		"max-5x":     7500,  // $75 of $100
		"max-20x":    15000, // $150 of $200
		"max":        7500,  // retired; covered as max-5x
		"team":       1875,  // per seat, $18.75 of a $25 seat
		"advisory":   15000, // agency retainers carry Max 20x's AI
		"dedicated":  15000,
		"enterprise": math.MaxInt64, // unlimited by contract
		"dns-pro":    0,
		"go":         0,
		"pro":        0,
	} {
		if got := coveredCents(slug); got != cents {
			t.Errorf("coveredCents(%q) = %d, want %d", slug, got, cents)
		}
	}
}

// TestVendoredPriceLadder: no rung is sold at levels. The $99 rung used to carry
// a ladder up to $999; the multiples are separate rungs now (max-5x, max-20x),
// so a ladder on either would be a second way to buy the same usage at a
// different price. A plan with no ladder is sold at exactly Price and every
// level above 0 is refused (LevelPrice), which is the direction that cannot
// overcharge.
func TestVendoredPriceLadder(t *testing.T) {
	for _, p := range catalog {
		if len(p.Prices) != 0 {
			t.Errorf("plan %q publishes a price ladder %v; the ladder is its own rungs now", p.Slug, p.Prices)
		}
	}
}
