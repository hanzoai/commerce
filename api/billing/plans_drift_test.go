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
	// 1.4.14 adds the max price ladder (`prices`) and changes nothing else. The
	// ladder is what makes the tier sellable at a chosen level, so it is plan
	// data and belongs in the catalog beside the price it starts from.
	"1.4.14": {
		subscription: "d3d8ce358ecc744b6908d6382c9d61f77a7d41a72ef84dcb0546bf5cc91abea6",
		dns:          "de7da2ab600268bdf5528b9ec1fd037bdbe8f9112f3755d80b5f93a4cbf1cd87",
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
// These are the @hanzo/plans@1.4.8 monthly/annual cents — the published ladder.
// contactSales plans are null-priced → stored as 0 + ContactSales (never a
// chargeable $0).
func TestVendoredPlanPrices(t *testing.T) {
	if got := len(hanzoPlans); got != 9 { // 6 subscription + 3 dns
		t.Fatalf("hanzoPlans = %d, want 9 (6 subscription + 3 dns)", got)
	}
	if got := len(dnsPlans); got != 3 {
		t.Fatalf("dnsPlans = %d, want 3", got)
	}
	bySlug := map[string]staticPlan{}
	for _, p := range hanzoPlans {
		bySlug[p.Slug] = p
	}
	type want struct {
		monthly, annual int64
		contactSales    bool
	}
	cases := map[string]want{
		"go":             {900, 825, false},
		"dev":            {1900, 1650, false},
		"pro":            {4900, 4150, false},
		"max":            {9900, 8325, false},
		"team":           {2500, 2000, false},
		"enterprise":     {0, 0, true}, // null price → 0 + contactSales
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
	for _, p := range hanzoPlans {
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
	for _, p := range hanzoPlans {
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

// A retired slug must STAY retired in the embed. The embed is what a fresh
// database seeds from, so a tier that comes back here comes back on the pricing
// page of every new deployment — which is exactly the resurrection plan.Status
// prevents for rows that already exist, and cannot prevent for rows that do not.
func TestRetiredSlugsAreNotInTheEmbed(t *testing.T) {
	bySlug := map[string]bool{}
	for _, p := range hanzoPlans {
		bySlug[p.Slug] = true
	}
	for _, slug := range []string{
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
		"go":         500,   // includedCreditUsd 5 — the one the spec states in dollars
		"dev":        500,   // includedCloudCredits 5
		"pro":        2500,  // includedCloudCredits 25
		"max":        10000, // includedCloudCredits 100
		"team":       10000, // includedCloudCredits 100
		"enterprise": 0,     // contact-sales: terms are negotiated, so none is published
		"dns-pro":    0,     // dns tiers mint no cloud allotment
	}
	for slug, cents := range want {
		if got := IncludedMonthlyCents(slug); got != cents {
			t.Errorf("IncludedMonthlyCents(%q) = %d, want %d (allotment mint amount drifted)", slug, got, cents)
		}
	}
}
