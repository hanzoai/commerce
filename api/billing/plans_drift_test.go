package billing

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
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
// These are the @hanzo/plans@1.4.16 monthly/annual cents — the published ladder.
// contactSales plans are null-priced → stored as 0 + ContactSales (never a
// chargeable $0).
func TestVendoredPlanPrices(t *testing.T) {
	if got := len(catalog); got != 8 { // 5 subscription + 3 dns
		t.Fatalf("catalog = %d, want 8 (5 subscription + 3 dns)", got)
	}
	if got := len(dnsPlans); got != 3 {
		t.Fatalf("dnsPlans = %d, want 3", got)
	}
	bySlug := map[string]staticPlan{}
	for _, p := range catalog {
		bySlug[p.Slug] = p
	}
	type want struct {
		monthly, annual int64
		contactSales    bool
	}
	// Annual is monthly less 18%, so these are not free-floating numbers: 1900
	// → 1558, 9900 → 8118, 2400 → 1968. A rung whose annual stops being 0.82 of
	// its monthly is selling a discount nobody agreed to.
	cases := map[string]want{
		"free":           {0, 0, false}, // a real $0 rung, not a null price
		"dev":            {1900, 1558, false},
		"max":            {9900, 8118, false},
		"team":           {2400, 1968, false}, // the individual plan plus $5 a seat
		"enterprise":     {0, 0, true},        // null price → 0 + contactSales
		"dns-free":       {0, 0, false},
		"dns-pro":        {500, 400, false},
		"dns-enterprise": {2500, 2000, false},
	}
	for slug, w := range cases {
		p, ok := bySlug[slug]
		if !ok {
			t.Fatalf("plan %q missing from embed", slug)
		}
		if p.Price != w.monthly || p.PriceAnnual != w.annual {
			t.Errorf("plan %q price = %d/%d cents, want %d/%d", slug, p.Price, p.PriceAnnual, w.monthly, w.annual)
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
		"dev":        {10, 0}, // the $19 tier
		"max":        {10, 1}, // the $99 tier may run a resident bot
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
		// go ($9) and pro ($49) came off the ladder; a renewal still prices
		// through resolveSubscriptionPlan, which is ungated by design, but the
		// embed must not offer them for sale again.
		"go", "pro",
		"developer", "plus", "team-max", "custom",
		"world-free", "world-pro", "world-team", "world-enterprise",
		"social-free", "social-pro", "social-team", "social-team-max", "social-enterprise",
	} {
		if bySlug[slug] {
			t.Errorf("retired plan %q is back in the embed", slug)
		}
	}
}

// TestVendoredAllotmentAmounts canaries the money-adjacent entitlement that
// actually MOVES money: limits.includedCloudCredits (→ IncludedMonthlyCents) is
// the allotment a subscription MINTS, and it's the value the C1-a PATCH gate
// (Red F1) compares. A drift here changes what a plan mints, so it must fail
// loudly + named — the price canary above does NOT cover it (Red F4).
func TestVendoredAllotmentAmounts(t *testing.T) {
	want := map[string]int64{ // slug -> allotment mint amount (cents)
		"free":       0,     // a personal rung sells USAGE, bounded by its windows —
		"dev":        0,     // never the refund shape a rung granting its own price
		"max":        0,     // back would be.
		"team":       10000, // includedCloudCreditsPerUser 100 — the one rung that grants
		"enterprise": 0,     // contact-sales: terms are negotiated, so none is published
		"dns-pro":    0,     // dns tiers mint no cloud allotment
		"go":         0,     // archived: an unpublished slug mints nothing at all
		"pro":        0,
	}
	for slug, cents := range want {
		if got := IncludedMonthlyCents(slug); got != cents {
			t.Errorf("IncludedMonthlyCents(%q) = %d, want %d (allotment mint amount drifted)", slug, got, cents)
		}
	}
}

// TestVendoredPriceLadder canaries the OTHER money-bearing number in the embed:
// the prices a plan is sold at above its base. The price canary above reads
// Price alone, so a re-vendored catalog could drop or reprice the whole max
// ladder without a single existing assertion noticing — and dropping it makes
// every level above the base refuse (LevelPrice), which is the safe direction but
// silently unsells the tier.
//
// It also pins the invariant the wire depends on: Prices[0] == Price. A client
// renders one control over Prices and sends back the index it lands on, and level
// 0 is answered from Price, so the two must name the same money or the first
// position on that control quotes one number and charges another.
func TestVendoredPriceLadder(t *testing.T) {
	bySlug := map[string]staticPlan{}
	for _, p := range catalog {
		bySlug[p.Slug] = p
	}

	max, ok := bySlug["max"]
	if !ok {
		t.Fatalf("plan %q missing from embed", "max")
	}
	want := []int64{9900, 19900, 29900, 39900, 49900, 59900, 69900, 79900, 89900, 99900}
	if !slices.Equal(max.Prices, want) {
		t.Fatalf("max ladder = %v cents, want %v", max.Prices, want)
	}

	// Every plan that publishes a ladder: it starts at the plan's own price and
	// it only ever goes up. Ascending is what lets a client render it as one
	// control without sorting it first, and a ladder that stepped backwards would
	// put a cheaper price at a higher level.
	for _, p := range catalog {
		if len(p.Prices) == 0 {
			continue
		}
		if p.Prices[0] != p.Price {
			t.Errorf("plan %q ladder starts at %d, want its price %d", p.Slug, p.Prices[0], p.Price)
		}
		for i := 1; i < len(p.Prices); i++ {
			if p.Prices[i] <= p.Prices[i-1] {
				t.Errorf("plan %q ladder is not ascending at %d: %d then %d", p.Slug, i, p.Prices[i-1], p.Prices[i])
			}
		}
	}
}
