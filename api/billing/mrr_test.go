package billing

import (
	"testing"

	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/types"
)

// MonthlyNormalizedCents is the one definition of interval normalization in
// commerce. api/metrics and the hanzoai/cloud admin each carried a copy; this
// is where the behaviour is now pinned for all of them.
func TestMonthlyNormalizedCents(t *testing.T) {
	for _, c := range []struct {
		price    int64
		interval string
		want     int64
	}{
		{2000, "month", 2000},
		{2000, "monthly", 2000},
		{2000, "Monthly", 2000}, // case-insensitive
		{2000, " month ", 2000}, // and space-tolerant
		{12000, "year", 1000},
		{12000, "annual", 1000},
		{12000, "annually", 1000},
		{12000, "yearly", 1000},
		{700, "week", 3033}, // 36400/12 = 3033.33
		// 255500/12 = 21291.67. This used to read 21291: the division
		// truncated, so every normalized price lost its fraction. It rounds
		// to the nearer cent now, in one place, for every interval.
		{700, "day", 21292},
		{2000, "", 2000}, // unknown → monthly, never zero
		{2000, "fortnight", 2000},
	} {
		// A count of 1 is the plain "every interval" plan; the multiplier has
		// its own test below.
		if got := MonthlyNormalizedCents(c.price, c.interval, 1); got != c.want {
			t.Errorf("MonthlyNormalizedCents(%d, %q, 1) = %d, want %d", c.price, c.interval, got, c.want)
		}
	}
}

// TestMonthlyNormalizedCentsHonorsIntervalCount pins the multiplier the
// billing engine already applies. advancePeriod (billing/engine/lifecycle.go)
// advances a period by AddDate(0, IntervalCount, 0), and the invoice charges
// the full Plan.Price ONCE per that period — so Interval×IntervalCount is the
// period, and a $50 plan on month/3 bills $50 every three months.
//
// The normalizer could not see the field: its signature took a price and an
// interval, which made the quarterly case structurally impossible to get
// right. It reported $50/mo for a plan worth $16.67/mo — a 3× overstatement of
// recurring revenue, amplified 12× again by ARR = MRR × 12.
func TestMonthlyNormalizedCentsHonorsIntervalCount(t *testing.T) {
	for _, c := range []struct {
		price    int64
		interval string
		count    int
		want     int64
	}{
		// The defect, stated: $50 every 3 months is $16.67/mo, not $50/mo.
		{5000, "month", 3, 1667},
		{5000, "month", 1, 5000},
		{2400, "month", 6, 400},    // semi-annual
		{12000, "month", 12, 1000}, // annual expressed as 12 months
		{24000, "year", 2, 1000},   // biennial
		{12000, "year", 1, 1000},
		// The engine reads a count of 0 or less as 1; so must this, or a
		// legacy row with an unset count would report infinite MRR.
		{5000, "month", 0, 5000},
		{5000, "month", -1, 5000},
		// Rounding is to the nearer cent, not toward zero: truncating biases
		// every quarterly and annual plan downward, forever.
		{10000, "month", 3, 3333}, // 3333.33
		{20000, "month", 3, 6667}, // 6666.67
	} {
		if got := MonthlyNormalizedCents(c.price, c.interval, c.count); got != c.want {
			t.Errorf("MonthlyNormalizedCents(%d, %q, %d) = %d, want %d",
				c.price, c.interval, c.count, got, c.want)
		}
	}
}

func sub(price int64, interval types.Interval, qty int) *subscription.Subscription {
	s := &subscription.Subscription{Quantity: qty}
	s.Plan.Price = currency.Cents(price)
	s.Plan.Interval = interval
	return s
}

// quarterly is sub() with the period multiplier the wire actually carries.
func quarterly(price int64, qty int) *subscription.Subscription {
	s := sub(price, types.Monthly, qty)
	s.Plan.IntervalCount = 3
	return s
}

// TestSubscriptionMRRCentsHonorsIntervalCount is the same defect seen through
// the function every reader actually calls — the subscriptions wire, the
// mrr_cents event, and the metrics rollup all come through here.
func TestSubscriptionMRRCentsHonorsIntervalCount(t *testing.T) {
	for _, c := range []struct {
		name string
		s    *subscription.Subscription
		want int64
	}{
		{"$50 every 3 months is $16.67/mo", quarterly(5000, 1), 1667},
		{"seats still multiply", quarterly(5000, 4), 6668},
	} {
		if got := SubscriptionMRRCents(c.s); got != c.want {
			t.Errorf("%s: SubscriptionMRRCents = %d, want %d", c.name, got, c.want)
		}
	}
}

// TestCountsTowardMRRExcludesTrialing pins the ONE definition of "is this
// revenue". A trialing subscription is not: nobody has been charged yet.
func TestCountsTowardMRRExcludesTrialing(t *testing.T) {
	for _, c := range []struct {
		status subscription.Status
		want   bool
	}{
		{subscription.Active, true},
		{subscription.Trialing, false},
		{subscription.PastDue, false},
		{subscription.Unpaid, false},
		{subscription.Canceled, false},
		{"ACTIVE", true},    // wire noise: cloud reads this off JSON
		{" active ", true},  //
		{"trialing", false}, //
	} {
		if got := c.status.CountsTowardMRR(); got != c.want {
			t.Errorf("Status(%q).CountsTowardMRR() = %v, want %v", c.status, got, c.want)
		}
	}
}

// SubscriptionMRRCents must include seats. commerce invoices charge
// Price × quantity (floored at 1), so an MRR that reports the unit price
// understates a multi-seat account by exactly the seat count — the cloud admin
// did this, reporting $20 for a 10-seat $20/seat plan worth $200.
func TestSubscriptionMRRCentsIncludesSeats(t *testing.T) {
	for _, c := range []struct {
		name string
		s    *subscription.Subscription
		want int64
	}{
		{"single seat monthly", sub(2000, types.Monthly, 1), 2000},
		{"ten seats monthly", sub(2000, types.Monthly, 10), 20000},
		{"ten seats annual", sub(12000, types.Yearly, 10), 10000},
		// A subscription exists, so it bills for at least one seat. Legacy rows
		// written before Quantity existed carry 0 and must not be free.
		{"quantity zero counts as one", sub(2000, types.Monthly, 0), 2000},
		{"negative quantity counts as one", sub(2000, types.Monthly, -3), 2000},
	} {
		if got := SubscriptionMRRCents(c.s); got != c.want {
			t.Errorf("%s: SubscriptionMRRCents = %d, want %d", c.name, got, c.want)
		}
	}
}

// The two are related by exactly the seat count, so no caller needs to know
// which one it wants beyond "a price" versus "a subscription".
func TestSubscriptionMRRIsNormalizedPriceTimesSeats(t *testing.T) {
	s := sub(9900, types.Yearly, 7)
	want := MonthlyNormalizedCents(9900, string(types.Yearly), 1) * 7
	if got := SubscriptionMRRCents(s); got != want {
		t.Errorf("SubscriptionMRRCents = %d, want MonthlyNormalizedCents×7 = %d", got, want)
	}
}
