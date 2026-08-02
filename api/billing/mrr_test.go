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
		{700, "week", 700 * 52 / 12},
		{700, "day", 700 * 365 / 12},
		{2000, "", 2000}, // unknown → monthly, never zero
		{2000, "fortnight", 2000},
	} {
		if got := MonthlyNormalizedCents(c.price, c.interval); got != c.want {
			t.Errorf("MonthlyNormalizedCents(%d, %q) = %d, want %d", c.price, c.interval, got, c.want)
		}
	}
}

func sub(price int64, interval types.Interval, qty int) *subscription.Subscription {
	s := &subscription.Subscription{Quantity: qty}
	s.Plan.Price = currency.Cents(price)
	s.Plan.Interval = interval
	return s
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
	want := MonthlyNormalizedCents(9900, string(types.Yearly)) * 7
	if got := SubscriptionMRRCents(s); got != want {
		t.Errorf("SubscriptionMRRCents = %d, want MonthlyNormalizedCents×7 = %d", got, want)
	}
}
