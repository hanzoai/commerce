package billing

import "testing"

// The three paid monthly tiers are priced $20/$100/$200 and carry the
// limited-time 50%-off promo, surfaced through the ONE plan catalog (/v1/plans)
// — the pricing UI reads PromoPercent/PromoUntil, no second source of truth.
func TestStaticPlans_MonthlyTiersPricedAndOnPromo(t *testing.T) {
	want := map[string]int64{"pro": 2000, "team": 10000, "max": 20000} // cents/month
	byslug := map[string]Plan{}
	for _, p := range StaticPlans() {
		byslug[p.Slug] = p
	}
	for slug, cents := range want {
		p, ok := byslug[slug]
		if !ok {
			t.Fatalf("plan %q missing from the catalog", slug)
		}
		if p.PriceMonth != cents {
			t.Errorf("%s PriceMonth = %d, want %d ($%.0f/mo)", slug, p.PriceMonth, cents, float64(cents)/100)
		}
		if p.PromoPercent != 50 {
			t.Errorf("%s PromoPercent = %v, want 50 (the limited-time promo)", slug, p.PromoPercent)
		}
		if p.PromoUntil == "" {
			t.Errorf("%s has a promo percent but no PromoUntil window — a promo must expire", slug)
		}
	}
}
