package billing

import (
	"testing"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/subscription"
	types "github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/test/ae"
)

// indexBySlug builds a slug → plan map so tests never depend on the positional
// order of the embedded catalog (which grows as @hanzo/plans adds tiers).
func indexBySlug(plans []staticPlan) map[string]staticPlan {
	m := make(map[string]staticPlan, len(plans))
	for _, p := range plans {
		m[p.Slug] = p
	}
	return m
}

func TestPlansLoaded(t *testing.T) {
	if len(catalog) == 0 {
		t.Fatal("catalog is empty")
	}

	bySlug := indexBySlug(catalog)

	// Core subscription plans that must exist regardless of catalog growth.
	must := []string{"free", "dev", "max", "team", "enterprise"}
	for _, slug := range must {
		p, ok := bySlug[slug]
		if !ok {
			t.Fatalf("required plan %q missing", slug)
		}
		if p.Name == "" {
			t.Errorf("plan %q.Name is empty", slug)
		}
		if p.Currency != "usd" {
			t.Errorf("plan %q.Currency = %q, want usd", slug, p.Currency)
		}
	}

	// `dev` is the entry paid rung; `enterprise` is priced by conversation, so it
	// is the one row with a null price.
	entry := bySlug["dev"]
	if entry.Price != 1900 {
		t.Errorf("Dev price = %d cents, want 1900", entry.Price)
	}
	if entry.Limits == nil {
		t.Fatal("Dev plan should have limits")
	}
	if entry.Limits.RequestsPerMinute == nil || *entry.Limits.RequestsPerMinute != 500 {
		t.Error("Dev requestsPerMinute should be 500")
	}
	if entry.Limits.TokensPerMinute == nil || *entry.Limits.TokensPerMinute != 1000000 {
		t.Error("Dev tokensPerMinute should be 1000000")
	}

	// The ladder climbs. Asserted as an ORDERING rather than four numbers, so it
	// keeps holding after a reprice and only fails on a rung that is out of order.
	prev := int64(-1)
	for _, slug := range []string{"free", "dev", "max"} {
		p := bySlug[slug]
		if p.Price <= prev {
			t.Errorf("ladder is not ascending at %q: %d cents follows %d", slug, p.Price, prev)
		}
		prev = p.Price
	}

	// Annual is one discount, the same on every rung: 18% off the monthly rate,
	// stored as the per-month equivalent. Pinned as the ARITHMETIC rather than as
	// four numbers, so a reprice moves both halves together and a rung that drifts
	// to its own private discount fails here.
	//
	// This supersedes an earlier rule that the stored per-month had to multiply to
	// a round dollar year (which forced it to a multiple of 25 cents). A fixed
	// percentage cannot also land on that grid — 19.00 x 0.82 is 15.58, and no
	// rounding of it is both 18% and a quarter — so the ladder now prices the
	// DISCOUNT exactly and lets the yearly total fall where it falls.
	for _, slug := range []string{"dev", "max", "team"} {
		p := bySlug[slug]
		if p.PriceAnnual <= 0 || p.PriceAnnual >= p.Price {
			t.Errorf("plan %q annual = %d cents/mo, monthly = %d; annual must be a discount", slug, p.PriceAnnual, p.Price)
		}
		if want := (p.Price * 82) / 100; p.PriceAnnual != want {
			t.Errorf("plan %q annual = %d cents/mo, want %d (18%% off %d)", slug, p.PriceAnnual, want, p.Price)
		}
	}
}

func TestDNSPlansLoaded(t *testing.T) {
	if len(dnsPlans) == 0 {
		t.Fatal("dnsPlans is empty")
	}

	if got := len(dnsPlans); got != 3 {
		t.Fatalf("expected 3 DNS plans, got %d", got)
	}

	bySlug := indexBySlug(dnsPlans)
	for _, slug := range []string{"dns-free", "dns-pro", "dns-enterprise"} {
		p, ok := bySlug[slug]
		if !ok {
			t.Fatalf("required DNS plan %q missing", slug)
		}
		if p.Category != "dns" {
			t.Errorf("DNS plan %q.Category = %q, want dns", slug, p.Category)
		}
		if p.Currency != "usd" {
			t.Errorf("DNS plan %q.Currency = %q, want usd", slug, p.Currency)
		}
		if len(p.Features) == 0 {
			t.Errorf("DNS plan %q.Features is empty", slug)
		}
		if p.Limits == nil {
			t.Fatalf("DNS plan %q should have limits", slug)
		}
	}

	if c := bySlug["dns-free"].Price; c != 0 {
		t.Errorf("DNS Free price = %d cents, want 0", c)
	}
	if !bySlug["dns-pro"].Popular {
		t.Error("DNS Pro plan should be popular")
	}
}

func TestLookupPlan(t *testing.T) {
	p := lookupPlan("dev")
	if p == nil {
		t.Fatal("lookupPlan(dev) returned nil")
	}
	if p.Slug != "dev" {
		t.Errorf("lookupPlan(dev).Slug = %q", p.Slug)
	}

	p = lookupPlan("dns-pro")
	if p == nil {
		t.Fatal("lookupPlan(dns-pro) returned nil")
	}
	if p.Slug != "dns-pro" {
		t.Errorf("lookupPlan(dns-pro).Slug = %q", p.Slug)
	}

	p = lookupPlan("nonexistent-plan")
	if p != nil {
		t.Errorf("lookupPlan(nonexistent-plan) should return nil, got %v", p.Slug)
	}
}

func TestIncludedMonthlyCents(t *testing.T) {
	// Unknown plan -> 0 (no allotment).
	if c := IncludedMonthlyCents("nonexistent-plan"); c != 0 {
		t.Errorf("IncludedMonthlyCents(unknown) = %d, want 0", c)
	}

	// Contract MIRRORS the function's real precedence: the monthly allotment is
	// the plan's declared cloud-credit allowance, sourced (in order) from
	// limits.includedCloudCredits (flat), then includedCloudCreditsPerUser (per
	// seat), then the legacy includedCreditUsd alias — dollars*100. Whichever the
	// plan declares > 0, IncludedMonthlyCents returns it; otherwise 0. Asserting
	// the RELATIONSHIP (not a hard-coded number) keeps this stable across
	// @hanzo/plans catalog versions fetched at build time. (The prior test checked
	// only includedCreditUsd, so plans that declare includedCloudCredits — e.g.
	// max=$100, team-max=$100/seat — failed with "10000 want 0".)
	for _, p := range catalog {
		got := IncludedMonthlyCents(p.Slug)
		var want int64
		if p.Limits != nil {
			if usd := firstNonNilPositive(p.Limits.IncludedCloudCredits, p.Limits.IncludedCloudCreditsPerUser, p.Limits.IncludedCreditUsd); usd != nil {
				want = int64(*usd) * 100
			}
		}
		if got != want {
			t.Errorf("IncludedMonthlyCents(%q) = %d, want %d (cloudCredits→cloudCreditsPerUser→creditUsd)", p.Slug, got, want)
		}
	}
}

// firstNonNilPositive returns the first argument that is non-nil AND > 0, or nil.
// Mirrors IncludedMonthlyCents's field-precedence selection so the test and the
// implementation cannot silently diverge.
func firstNonNilPositive(vals ...*int) *int {
	for _, v := range vals {
		if v != nil && *v > 0 {
			return v
		}
	}
	return nil
}

// TestCatalogIntervalIsTheConstant: the interval the catalog projects is the one
// the rest of billing reads, spelled the one way it is declared.
//
// Every consumer of Plan.Interval matches on types.Monthly ("month"), and the two
// that bill reach their monthly answer by a DIFFERENT route when handed anything
// else: advancePeriod through the default arm of its switch, MonthlyNormalizedCents
// through the default arm of its own. Both arms exist to be generous about what a
// stored row might hold, and neither is a place for the catalog's own value to
// land — an answer reached through a fallback is one nothing asserts, and the next
// case added above the default takes it silently.
//
// PeriodsRemaining is the reading of the same field with no default to catch it:
// it answers the YEAR difference for anything that is not types.Monthly.
func TestCatalogIntervalIsTheConstant(t *testing.T) {
	for _, sp := range catalog {
		if types.Interval(sp.Interval) != types.Monthly {
			t.Errorf("catalog plan %q carries interval %q, want %q",
				sp.Slug, sp.Interval, types.Monthly)
		}
	}
}

// TestCatalogMonthlyPlanGetsAMonth reads the projected interval the way billing
// does — through the engine that sets the period and the normalizer that reports
// the revenue — so what is pinned is the period a subscriber actually gets rather
// than the spelling of a string.
func TestCatalogMonthlyPlanGetsAMonth(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	for _, slug := range []string{"dev", "max", "team"} {
		// The stored authority row: StartSubscription records the plan's id, so the
		// period under test is the one a real purchase gets.
		row, err := resolveSubscriptionPlan(datastore.New(ctx), slug)
		if err != nil {
			t.Fatalf("resolve %s: %v", slug, err)
		}

		sub := &subscription.Subscription{}
		engine.StartSubscription(sub, row)
		if want := sub.PeriodStart.AddDate(0, 1, 0); !sub.PeriodEnd.Equal(want) {
			t.Errorf("%s bills every %s..%s, want a month ending %s",
				slug, sub.PeriodStart, sub.PeriodEnd, want)
		}

		if got := MonthlyNormalizedCents(int64(row.Price), string(row.Interval), row.IntervalCount); got != int64(row.Price) {
			t.Errorf("%s normalizes to %d, want its own monthly price %d", slug, got, row.Price)
		}
	}
}
