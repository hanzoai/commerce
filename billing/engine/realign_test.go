package engine

import (
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/types"
)

// oldShape stores a subscription the way a sale that moved the period on as it
// recorded the charge left it: a paid invoice for [t0, t1], CurrentInvoiceId
// naming it, and the row already on [t1, t2], one period past what was paid.
func oldShape(t *testing.T, db *datastore.Datastore, p plan.Plan, t0 time.Time) *subscription.Subscription {
	t.Helper()
	sub := subscription.New(db)
	sub.UserId = "settle/old"
	sub.PlanId = "plan_" + string(p.Interval)
	sub.Plan = p
	sub.Status = subscription.Active
	sub.PeriodStart, sub.PeriodEnd = t0, Advance(t0, &p)
	if err := sub.Create(); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := CreatePaidFirstInvoice(db, sub, "card", "sqpay_first"); err != nil {
		t.Fatalf("first invoice: %v", err)
	}
	sub.PeriodStart, sub.PeriodEnd = sub.PeriodEnd, Advance(sub.PeriodEnd, &p)
	if err := sub.Update(); err != nil {
		t.Fatalf("move the row a period on: %v", err)
	}
	return sub
}

var yearly = plan.Plan{Name: "Pro", Price: 20000, Currency: currency.USD, Interval: types.Yearly, IntervalCount: 1}

// TestRealign_MovesARowBackOntoItsPaidInvoice: a row whose PeriodStart is its
// current invoice's PeriodEnd, that invoice issued before the cutover, is moved
// onto that invoice's period. A dry run answers the move and writes nothing; the
// real run writes it and marks the row; a second run finds nothing to do, and
// neither does a run after the row is moved on again without a new invoice.
func TestRealign_MovesARowBackOntoItsPaidInvoice(t *testing.T) {
	t0 := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	for _, p := range []plan.Plan{monthly, yearly} {
		t.Run(string(p.Interval), func(t *testing.T) {
			db, done := settleDB(t, "realign-"+string(p.Interval))
			defer done()
			sub := oldShape(t, db, p, t0)
			paid := Advance(t0, &p)
			before := reload(t, db, sub.Id())
			cutover := time.Now().Add(time.Minute)

			dry, err := Realign(db, reload(t, db, sub.Id()), cutover, true)
			if err != nil || dry == nil {
				t.Fatalf("dry run: %v (err %v), want a move", dry, err)
			}
			if !dry.FromStart.Equal(paid) || !dry.ToStart.Equal(t0) || !dry.ToEnd.Equal(paid) || dry.InvoiceId != before.CurrentInvoiceId {
				t.Fatalf("dry run %+v, want %s..%s onto invoice %s", dry, t0, paid, before.CurrentInvoiceId)
			}
			if after := reload(t, db, sub.Id()); !after.PeriodStart.Equal(before.PeriodStart) || !after.UpdatedAt.Equal(before.UpdatedAt) {
				t.Fatal("a dry run wrote the row")
			}

			done1, err := Realign(db, reload(t, db, sub.Id()), cutover, false)
			if err != nil || done1 == nil {
				t.Fatalf("realign: %v (err %v)", done1, err)
			}
			got := reload(t, db, sub.Id())
			if !got.PeriodStart.Equal(t0) || !got.PeriodEnd.Equal(paid) || got.Status != subscription.Active {
				t.Fatalf("row %s %s..%s, want active on the paid period %s..%s", got.Status, got.PeriodStart, got.PeriodEnd, t0, paid)
			}
			if _, marked := got.Metadata[RealignedKey]; !marked {
				t.Fatal("the realigned row carries no mark")
			}
			if again, err := Realign(db, got, cutover, false); err != nil || again != nil {
				t.Fatalf("a realigned row realigned again: %+v (err %v)", again, err)
			}
			got.PeriodStart, got.PeriodEnd = paid, Advance(paid, &p)
			if err := got.Update(); err != nil {
				t.Fatal(err)
			}
			if again, err := Realign(db, reload(t, db, sub.Id()), cutover, false); err != nil || again != nil {
				t.Fatalf("a realigned row moved on without a new invoice was realigned again: %+v (err %v)", again, err)
			}
		})
	}
}

// TestRealign_LeavesEveryOtherRowAlone: the rule holds only for a live row
// sitting exactly one paid invoice's end ahead, on its own paid invoice.
func TestRealign_LeavesEveryOtherRowAlone(t *testing.T) {
	t0 := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		cutover time.Time
		shape   func(t *testing.T, db *datastore.Datastore) *subscription.Subscription
	}{
		{"on its paid period", time.Time{}, func(t *testing.T, db *datastore.Datastore) *subscription.Subscription {
			sub := paidThrough(t, db, monthly, t0.AddDate(0, 1, 0))
			if _, err := CreatePaidFirstInvoice(db, sub, "card", "sqpay"); err != nil {
				t.Fatal(err)
			}
			if err := sub.Update(); err != nil {
				t.Fatal(err)
			}
			return sub
		}},
		{"renewed onto its paid period", time.Time{}, func(t *testing.T, db *datastore.Datastore) *subscription.Subscription {
			sub := oldShape(t, db, monthly, t0)
			inv := issueFor(t, db, sub, sub.PeriodStart, sub.PeriodEnd)
			if err := inv.MarkPaid("card", "sqpay_renewal"); err != nil {
				t.Fatal(err)
			}
			if err := inv.Update(); err != nil {
				t.Fatal(err)
			}
			sub.CurrentInvoiceId = inv.Id()
			if err := sub.Update(); err != nil {
				t.Fatal(err)
			}
			return sub
		}},
		{"its invoice is not paid", time.Time{}, func(t *testing.T, db *datastore.Datastore) *subscription.Subscription {
			sub := oldShape(t, db, monthly, t0)
			inv, err := loadInvoice(db, sub.CurrentInvoiceId)
			if err != nil {
				t.Fatal(err)
			}
			inv.Status = billinginvoice.Open
			if err := inv.Update(); err != nil {
				t.Fatal(err)
			}
			return sub
		}},
		{"its invoice bills no period", time.Time{}, func(t *testing.T, db *datastore.Datastore) *subscription.Subscription {
			sub := oldShape(t, db, monthly, t0)
			inv, err := loadInvoice(db, sub.CurrentInvoiceId)
			if err != nil {
				t.Fatal(err)
			}
			inv.PeriodStart = inv.PeriodEnd
			if err := inv.Update(); err != nil {
				t.Fatal(err)
			}
			return sub
		}},
		{"its invoice is another subscription's", time.Time{}, func(t *testing.T, db *datastore.Datastore) *subscription.Subscription {
			sub := oldShape(t, db, monthly, t0)
			inv, err := loadInvoice(db, sub.CurrentInvoiceId)
			if err != nil {
				t.Fatal(err)
			}
			inv.SubscriptionId = "sub_other"
			if err := inv.Update(); err != nil {
				t.Fatal(err)
			}
			return sub
		}},
		{"it names no invoice", time.Time{}, func(t *testing.T, db *datastore.Datastore) *subscription.Subscription {
			sub := oldShape(t, db, monthly, t0)
			sub.CurrentInvoiceId = ""
			if err := sub.Update(); err != nil {
				t.Fatal(err)
			}
			return sub
		}},
		{"its invoice was issued after the cutover", time.Now().Add(-time.Minute), func(t *testing.T, db *datastore.Datastore) *subscription.Subscription {
			return oldShape(t, db, monthly, t0)
		}},
		{"it has ended", time.Time{}, func(t *testing.T, db *datastore.Datastore) *subscription.Subscription {
			sub := oldShape(t, db, monthly, t0)
			End(sub, t0.AddDate(0, 1, 0), t0.AddDate(0, 1, 0), CanceledAtPeriodEnd)
			if err := sub.Update(); err != nil {
				t.Fatal(err)
			}
			return sub
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, done := settleDB(t, "realign-alone")
			defer done()
			sub := tc.shape(t, db)
			before := reload(t, db, sub.Id())
			if r, err := Realign(db, before, tc.cutover, false); err != nil || r != nil {
				t.Fatalf("realigned %+v (err %v), want the row left alone", r, err)
			}
			if after := reload(t, db, sub.Id()); !after.PeriodStart.Equal(before.PeriodStart) || !after.PeriodEnd.Equal(before.PeriodEnd) {
				t.Fatalf("row moved from %s..%s to %s..%s", before.PeriodStart, before.PeriodEnd, after.PeriodStart, after.PeriodEnd)
			}
		})
	}
}

// TestRealign_NeverRunsInsideTheCycle: the cycle settles a row by the dates it
// holds. A row a period ahead of its paid invoice is not due at the end of the
// period it paid for until Realign, run on its own, moves it; then it renews
// that period once.
func TestRealign_NeverRunsInsideTheCycle(t *testing.T) {
	db, done := settleDB(t, "realign-not-in-cycle")
	defer done()
	t0 := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	sub := oldShape(t, db, monthly, t0)
	paid := Advance(t0, &monthly)
	c := &card{}

	if step := settleAt(t, db, reload(t, db, sub.Id()), paid.Add(time.Hour), c, false); step.Action != "" || len(c.amounts) != 0 {
		t.Fatalf("the cycle acted on the unrealigned row: %s charges %v", step.Action, c.amounts)
	}
	if got := reload(t, db, sub.Id()); !got.PeriodStart.Equal(paid) {
		t.Fatalf("the cycle moved the row to %s", got.PeriodStart)
	}

	if _, err := Realign(db, reload(t, db, sub.Id()), time.Time{}, false); err != nil {
		t.Fatalf("realign: %v", err)
	}
	step := settleAt(t, db, reload(t, db, sub.Id()), paid.Add(time.Hour), c, false)
	if step.Action != Renewed || len(c.amounts) != 1 || c.amounts[0] != 2000 {
		t.Fatalf("after realign: %s charges %v, want the next month renewed once", step.Action, c.amounts)
	}
	if got := reload(t, db, sub.Id()); !got.PeriodStart.Equal(paid) || !got.PeriodEnd.Equal(Advance(paid, &monthly)) {
		t.Fatalf("row %s..%s, want the month after the paid one", got.PeriodStart, got.PeriodEnd)
	}
}
