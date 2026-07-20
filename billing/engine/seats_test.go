package engine

import (
	"context"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestBuildPeriodInvoice_Seats proves the per-seat money invariant: a per-seat
// plan invoices Price × quantity (floored at 1 seat), while a flat plan's
// invoice is UNCHANGED by quantity.
func TestBuildPeriodInvoice_Seats(t *testing.T) {
	cases := []struct {
		name     string
		perSeat  bool
		price    int64
		quantity int
		want     int64
	}{
		{"per-seat team ×4 seats", true, 2500, 4, 10000},
		{"per-seat floors at 1 seat", true, 2500, 0, 2500},
		{"flat plan ignores quantity", false, 2000, 4, 2000},
		{"flat plan quantity 1", false, 2000, 1, 2000},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := ae.NewContext()
			defer c.Close()

			db := datastore.New(c)
			db.SetNamespace("seats-invoice")

			now := time.Now()
			sub := subscription.New(db)
			sub.UserId = "seats/user"
			sub.PlanId = "plan_seats_" + tc.name
			sub.Plan = plan.Plan{
				Name:          "Seats",
				Price:         currency.Cents(tc.price),
				PerSeat:       tc.perSeat,
				Currency:      currency.USD,
				Interval:      types.Monthly,
				IntervalCount: 1,
			}
			sub.Quantity = tc.quantity
			sub.Status = subscription.Active
			sub.PeriodStart = now.AddDate(0, -1, -i) // distinct periods per case
			sub.PeriodEnd = now.AddDate(0, 0, -1-i)
			if err := sub.Create(); err != nil {
				t.Fatalf("create sub: %v", err)
			}

			inv, _, err := RenewSubscription(context.Background(), db, sub, nil, nil)
			if err != nil {
				t.Fatalf("renew: %v", err)
			}
			if len(inv.LineItems) == 0 {
				t.Fatal("invoice has no line items")
			}
			li := inv.LineItems[0]
			if li.Amount != tc.want {
				t.Fatalf("line amount = %d, want %d (price %d × quantity %d, perSeat=%v)",
					li.Amount, tc.want, tc.price, tc.quantity, tc.perSeat)
			}
			if li.UnitPrice != tc.price {
				t.Fatalf("line unitPrice = %d, want %d", li.UnitPrice, tc.price)
			}
			if inv.Subtotal != tc.want || inv.AmountDue != tc.want {
				t.Fatalf("subtotal=%d amountDue=%d, want %d", inv.Subtotal, inv.AmountDue, tc.want)
			}
		})
	}
}

// TestChangePlan_ProrationSeats proves proration respects quantity: on a
// per-seat plan the credit/charge scale by the subscription's seat count. The
// proration fraction is time.Now-based, so the proof is relational — the
// 3-seat net equals 3 × the 1-seat net within a 3-cent flooring bound.
func TestChangePlan_ProrationSeats(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	db := datastore.New(c)
	db.SetNamespace("seats-proration")

	mkSub := func(qty int) *subscription.Subscription {
		now := time.Now()
		sub := subscription.New(db)
		sub.Plan = plan.Plan{
			Name:     "Team",
			Price:    currency.Cents(2500),
			PerSeat:  true,
			Currency: currency.USD,
			Interval: types.Monthly,
		}
		sub.Quantity = qty
		sub.PeriodStart = now.AddDate(0, 0, -15)
		sub.PeriodEnd = now.AddDate(0, 0, 15)
		return sub
	}
	newPlan := plan.New(db)
	newPlan.Name = "Team Max"
	newPlan.Price = currency.Cents(22500)
	newPlan.PerSeat = true
	newPlan.Currency = currency.USD
	newPlan.Interval = types.Monthly

	item1, err := ChangePlan(mkSub(1), newPlan, true)
	if err != nil {
		t.Fatalf("proration qty=1: %v", err)
	}
	item3, err := ChangePlan(mkSub(3), newPlan, true)
	if err != nil {
		t.Fatalf("proration qty=3: %v", err)
	}
	if item1 == nil || item3 == nil {
		t.Fatal("expected proration line items")
	}
	if item1.Amount <= 0 {
		t.Fatalf("upgrade proration must be a positive net charge, got %d", item1.Amount)
	}
	want := 3 * item1.Amount
	if diff := item3.Amount - want; diff < -3 || diff > 3 {
		t.Fatalf("3-seat proration = %d, want ~3 × 1-seat (%d) within 3c", item3.Amount, want)
	}

	// Flat plans are UNCHANGED by quantity: same prices, PerSeat off.
	flatOld := mkSub(3)
	flatOld.Plan.PerSeat = false
	flatNew := plan.New(db)
	flatNew.Name = newPlan.Name
	flatNew.Price = newPlan.Price
	flatNew.Currency = newPlan.Currency
	flatNew.Interval = newPlan.Interval
	itemFlat, err := ChangePlan(flatOld, flatNew, true)
	if err != nil {
		t.Fatalf("flat proration: %v", err)
	}
	if diff := itemFlat.Amount - item1.Amount; diff < -3 || diff > 3 {
		t.Fatalf("flat-plan proration at qty=3 (%d) must match the 1-seat net (%d)", itemFlat.Amount, item1.Amount)
	}
}
