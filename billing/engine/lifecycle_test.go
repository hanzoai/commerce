package engine

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/test/ae"
)

// declineCharger is a card that declines every charge, counting the attempts
// and the amounts asked for.
func declineCharger(calls *int) ProviderCharger {
	return func(context.Context, *datastore.Datastore, *billinginvoice.BillingInvoice, int64) (string, error) {
		*calls++
		return "", errors.New("CARD_DECLINED")
	}
}

// acceptCharger is a card that accepts every charge.
func acceptCharger(calls *int) ProviderCharger {
	return func(context.Context, *datastore.Datastore, *billinginvoice.BillingInvoice, int64) (string, error) {
		*calls++
		return "sqpay_ok", nil
	}
}

// dueSub stores an Active monthly $20 subscription paid through an hour ago.
func dueSub(t *testing.T, db *datastore.Datastore, user string, now time.Time) *subscription.Subscription {
	t.Helper()
	sub := subscription.New(db)
	sub.UserId = user
	sub.PlanId = "plan_pro"
	sub.Plan = plan.Plan{
		Name:          "Pro",
		Price:         currency.Cents(2000), // $20
		Currency:      currency.USD,
		Interval:      types.Monthly,
		IntervalCount: 1,
	}
	sub.Status = subscription.Active
	sub.PeriodEnd = now.Add(-time.Hour)
	sub.PeriodStart = sub.PeriodEnd.AddDate(0, -1, 0)
	if err := sub.Create(); err != nil {
		t.Fatalf("create sub: %v", err)
	}
	return sub
}

// TestSettle_DeclineIsIdempotentPerPeriod proves the money-path invariant:
// settling the SAME (subscription, period) again generates no second invoice and
// no second charge. A declined renewal leaves the subscription past_due on the
// same paid-through period, so every cycle until the next retry sees it due;
// without the per-period lookup each one would mint and charge a duplicate.
func TestSettle_DeclineIsIdempotentPerPeriod(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	db := datastore.New(c)
	db.SetNamespace("renew-idem")

	now := time.Now()
	sub := dueSub(t, db, "renew-idem/alice", now)
	calls := 0

	step1, err := Settle(context.Background(), db, sub, Run{Now: now}, CardPayer(declineCharger(&calls)))
	if err != nil {
		t.Fatalf("first settle: %v", err)
	}
	if step1.Action != RenewalFailed || step1.Invoice == nil {
		t.Fatalf("first settle = %+v, want a renewal_failed step on an invoice", step1)
	}
	if step1.Invoice.Number == 0 || step1.Invoice.NumberStr == "" {
		t.Fatalf("first invoice is not numbered: Number=%d NumberStr=%q", step1.Invoice.Number, step1.Invoice.NumberStr)
	}
	if sub.Status != subscription.PastDue {
		t.Fatalf("sub status = %s, want past_due (period must NOT advance on a decline)", sub.Status)
	}

	// Settle the SAME period again: it waits for the retry, reusing the invoice.
	step2, err := Settle(context.Background(), db, sub, Run{Now: now}, CardPayer(declineCharger(&calls)))
	if err != nil {
		t.Fatalf("second settle: %v", err)
	}
	if step2.Action != Skipped || step2.Invoice == nil || step2.Invoice.Id() != step1.Invoice.Id() {
		t.Fatalf("second settle = %+v, want a skip on the SAME invoice %s", step2, step1.Invoice.Id())
	}
	if calls != 1 {
		t.Fatalf("card charged %d times, want 1", calls)
	}

	all := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := billinginvoice.Query(db).GetAll(&all); err != nil {
		t.Fatalf("query invoices: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("invoice count = %d, want 1 — no duplicate invoice per period", len(all))
	}
	if all[0].Number == 0 || all[0].NumberStr == "" {
		t.Fatalf("persisted invoice not numbered: Number=%d NumberStr=%q", all[0].Number, all[0].NumberStr)
	}
}

// TestSettle_NumbersDistinctPeriods proves numbering increments across periods:
// a paid renewal moves the subscription onto the next period, and renewing that
// one produces invoice #2.
func TestSettle_NumbersDistinctPeriods(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	db := datastore.New(c)
	db.SetNamespace("renew-number")

	now := time.Now()
	sub := dueSub(t, db, "renew-number/dave", now)
	calls := 0

	step1, err := Settle(context.Background(), db, sub, Run{Now: now}, CardPayer(acceptCharger(&calls)))
	if err != nil {
		t.Fatalf("first settle: %v", err)
	}
	if step1.Action != Renewed || step1.Invoice.Number != 1 {
		t.Fatalf("first settle = %s invoice #%d, want renewed invoice #1", step1.Action, step1.Invoice.Number)
	}

	step2, err := Settle(context.Background(), db, sub, Run{Now: sub.PeriodEnd}, CardPayer(acceptCharger(&calls)))
	if err != nil {
		t.Fatalf("second settle: %v", err)
	}
	if step2.Action != Renewed || step2.Invoice.Number != 2 {
		t.Fatalf("second settle = %s invoice #%d, want renewed invoice #2 (per-org sequential)", step2.Action, step2.Invoice.Number)
	}
	if step2.Invoice.Id() == step1.Invoice.Id() {
		t.Fatalf("distinct periods must yield distinct invoices")
	}
	if calls != 2 {
		t.Fatalf("card charged %d times, want 2", calls)
	}
}

// TestProrationRounds pins the proration arithmetic. `old` is what the truncating spelling
// this replaced returned, and every row where it differs from `want` is a cent that fell
// off a plan change.
//
// Truncating here is not merely biased, it is biased in BOTH directions at once, because
// proration computes two amounts and subtracts them. The credit for the old plan truncates
// down, which is a cent the customer does not get back; the charge for the new plan
// truncates down, which is a cent we do not bill. Neither is a policy anyone chose. Half
// away from zero is the same rule the discount path already uses, so a plan change and a
// coupon cannot disagree about what a fraction of a cent is.
func TestProrationRounds(t *testing.T) {
	for _, tc := range []struct {
		amount   currency.Cents
		fraction float64
		want     int64
		old      int64
	}{
		{1999, 0.5, 1000, 999},   // 999.5
		{995, 0.5, 498, 497},     // 497.5
		{2999, 0.1, 300, 299},    // 299.9
		{1000, 0.333, 333, 333},  // 333 exactly
		{4999, 0.25, 1250, 1249}, // 1249.75
		{100, 0.075, 8, 7},       // 7.5
		{29, 0.33, 10, 9},        // 9.57
		{0, 0.5, 0, 0},
		{1999, 0, 0, 0},
		{1999, 1, 1999, 1999},
		// A downgrade credit is the same amount reversed, so it must round the same
		// distance from zero — floor would make a refund disagree with the charge.
		{-1999, 0.5, -1000, -999},
	} {
		got, err := proration(tc.amount, tc.fraction)
		if err != nil {
			t.Fatalf("proration(%d, %v): %v", tc.amount, tc.fraction, err)
		}
		if got != tc.want {
			t.Errorf("proration(%d, %v) = %d, want %d (truncating gave %d)",
				tc.amount, tc.fraction, got, tc.want, tc.old)
		}
	}
}

// TestProrationRejectsANonFraction: a period whose start and end are the same instant makes
// remaining/total a NaN, and NaN converted to an integer is undefined in Go. A proration
// line of an undefined number of cents is worse than no proration line.
func TestProrationRejectsANonFraction(t *testing.T) {
	if _, err := proration(1999, math.NaN()); err == nil {
		t.Error("proration with a NaN fraction: want an error, got none")
	}
	if _, err := proration(1999, math.Inf(1)); err == nil {
		t.Error("proration with an infinite fraction: want an error, got none")
	}
}

// TestIsDue_ExternalIsNeverDue: a plan collected outside Hanzo is paid to the
// processor directly, so no elapsed period makes it due; its next period is
// recorded when that payment arrives. A checkout row with the same status and
// period is due, which is what makes the difference the type.
func TestIsDue_ExternalIsNeverDue(t *testing.T) {
	end := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	later := end.AddDate(1, 0, 0)
	for _, status := range []subscription.Status{subscription.Active, subscription.PastDue} {
		checkout := &subscription.Subscription{Status: status, PeriodEnd: end}
		if !IsDue(checkout, later) {
			t.Fatalf("%s checkout row past its period: want due", status)
		}
		external := &subscription.Subscription{Status: status, PeriodEnd: end, Type: subscription.External}
		if IsDue(external, later) {
			t.Fatalf("%s external row answered due", status)
		}
	}
}

// TestVoidOpen_LeavesAnInvoiceBeingPaid: an ended row's open invoices are
// voided, except one whose lock a payment holds and one whose last attempt has
// no known outcome; what a voided one collected goes back to the balance.
func TestVoidOpen_LeavesAnInvoiceBeingPaid(t *testing.T) {
	db, done := settleDB(t, "void-open")
	defer done()
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, d)

	paying := issueFor(t, db, sub, d, d.AddDate(0, 1, 0))
	release, err := LockInvoice(db, paying.Id())
	if err != nil {
		t.Fatalf("hold the payment lock: %v", err)
	}
	defer release()
	pending := issueFor(t, db, sub, d.AddDate(0, 1, 0), d.AddDate(0, 2, 0))
	pending.PendingMethod, pending.PendingAmount, pending.PendingKey = PaidByCard, pending.AmountDue, "collect:pending"
	if err := pending.Update(); err != nil {
		t.Fatalf("record pending attempt: %v", err)
	}
	idle := issueFor(t, db, sub, d.AddDate(0, 2, 0), d.AddDate(0, 3, 0))
	idle.AmountPaid = 700
	if err := idle.Update(); err != nil {
		t.Fatalf("part-pay: %v", err)
	}

	p := &purse{}
	if err := VoidOpen(context.Background(), db, sub, p, d); err != nil {
		t.Fatalf("void: %v", err)
	}
	for _, tc := range []struct {
		inv  *billinginvoice.BillingInvoice
		want billinginvoice.Status
	}{{paying, billinginvoice.Open}, {pending, billinginvoice.Open}, {idle, billinginvoice.Void}} {
		got, err := loadInvoice(db, tc.inv.Id())
		if err != nil || got.Status != tc.want {
			t.Fatalf("invoice %s is %v (err %v), want %s", tc.inv.Id(), got, err, tc.want)
		}
	}
	if p.returned["invoice-return:"+idle.Id()] != 700 || len(p.returned) != 1 {
		t.Fatalf("returned %v, want the voided invoice's 700 cents back once", p.returned)
	}
}
