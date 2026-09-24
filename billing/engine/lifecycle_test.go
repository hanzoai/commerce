package engine

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestRenewSubscription_IdempotentPerPeriod proves the money-path invariant:
// re-running a billing cycle for the SAME (subscription, period) generates
// EXACTLY ONE invoice, and that invoice carries a sequential number. This is
// the exact case a PastDue subscription hits on every cycle — its period never
// advances while collection fails, so without the idempotency guard each cycle
// would mint a duplicate, unnumbered invoice.
func TestRenewSubscription_IdempotentPerPeriod(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	db := datastore.New(c)
	db.SetNamespace("renew-idem")

	now := time.Now()
	sub := subscription.New(db)
	sub.UserId = "renew-idem/alice"
	sub.PlanId = "plan_pro"
	sub.Plan = plan.Plan{
		Name:          "Pro",
		Price:         currency.Cents(2000), // $20
		Currency:      currency.USD,
		Interval:      types.Monthly,
		IntervalCount: 1,
	}
	sub.Status = subscription.Active
	sub.PeriodStart = now.AddDate(0, -1, 0)
	sub.PeriodEnd = now.AddDate(0, 0, -1) // period already elapsed
	if err := sub.Create(); err != nil {
		t.Fatalf("create sub: %v", err)
	}

	// No balance and no credits (burnCredits=nil) → collection fails → the sub
	// goes PastDue and the period stays fixed.
	inv1, res1, err := RenewSubscription(context.Background(), db, sub, nil, nil)
	if err != nil {
		t.Fatalf("first renew: %v", err)
	}
	if res1 == nil || res1.Success {
		t.Fatalf("expected a non-nil FAILED collection (no funds), got %+v", res1)
	}
	if inv1.Number == 0 || inv1.NumberStr == "" {
		t.Fatalf("first invoice is not numbered: Number=%d NumberStr=%q", inv1.Number, inv1.NumberStr)
	}
	if sub.Status != subscription.PastDue {
		t.Fatalf("sub status = %s, want past_due (period must NOT advance on failed collection)", sub.Status)
	}

	// Re-run the SAME period. Must reuse inv1 — never mint a second invoice.
	inv2, res2, err := RenewSubscription(context.Background(), db, sub, nil, nil)
	if err != nil {
		t.Fatalf("second renew: %v", err)
	}
	if res2 == nil {
		t.Fatalf("second renew returned a nil result (callers dereference result.Success)")
	}
	if inv2.Id() != inv1.Id() {
		t.Fatalf("second renew returned a DIFFERENT invoice: %s vs %s (duplicate!)", inv2.Id(), inv1.Id())
	}

	// The datastore must hold EXACTLY ONE billing invoice for this org.
	rootKey := db.NewKey("synckey", "", 1, nil)
	all := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := billinginvoice.Query(db).Ancestor(rootKey).GetAll(&all); err != nil {
		t.Fatalf("query invoices: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("invoice count = %d, want 1 — no duplicate invoice per period", len(all))
	}
	if all[0].Number == 0 || all[0].NumberStr == "" {
		t.Fatalf("persisted invoice not numbered: Number=%d NumberStr=%q", all[0].Number, all[0].NumberStr)
	}
	t.Logf("PROOF: 2 renews of the same period -> 1 invoice %s (#%d), sub=%s",
		all[0].NumberStr, all[0].Number, sub.Status)
}

// TestRenewSubscription_NumbersDistinctPeriods proves numbering increments
// across distinct periods: a successful first period (which advances the sub)
// followed by a second renewal produces two invoices numbered 1 then 2.
func TestRenewSubscription_NumbersDistinctPeriods(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	db := datastore.New(c)
	db.SetNamespace("renew-number")

	now := time.Now()
	sub := subscription.New(db)
	sub.UserId = "renew-number/dave"
	sub.PlanId = "plan_free"
	sub.Plan = plan.Plan{
		Name:          "Zero",
		Price:         currency.Cents(0), // $0 plan → AmountDue 0 → collection trivially succeeds
		Currency:      currency.USD,
		Interval:      types.Monthly,
		IntervalCount: 1,
	}
	sub.Status = subscription.Active
	sub.PeriodStart = now.AddDate(0, -2, 0)
	sub.PeriodEnd = now.AddDate(0, -1, 0)
	if err := sub.Create(); err != nil {
		t.Fatalf("create sub: %v", err)
	}

	inv1, res1, err := RenewSubscription(context.Background(), db, sub, nil, nil)
	if err != nil {
		t.Fatalf("first renew: %v", err)
	}
	if !res1.Success {
		t.Fatalf("zero-amount invoice should collect successfully, got %+v", res1)
	}
	if inv1.Number != 1 {
		t.Fatalf("first invoice number = %d, want 1", inv1.Number)
	}

	// The row moved onto the period it paid for. Once that one is over, a second
	// renewal bills a NEW period → invoice #2.
	sub.PeriodStart, sub.PeriodEnd = sub.PeriodStart.AddDate(0, -1, 0), time.Now().Add(-time.Minute)
	inv2, _, err := RenewSubscription(context.Background(), db, sub, nil, nil)
	if err != nil {
		t.Fatalf("second renew: %v", err)
	}
	if inv2.Number != 2 {
		t.Fatalf("second invoice number = %d, want 2 (per-org sequential)", inv2.Number)
	}
	if inv2.Id() == inv1.Id() {
		t.Fatalf("distinct periods must yield distinct invoices")
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
// processor directly, so no elapsed period makes it due. A checkout row with the
// same status and period is due, which is what makes the difference the type.
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

// TestRenewSubscription_TheExistingInvoiceDecidesTheRow: a row whose owed period
// already has an invoice is never charged again here, but it is not left active
// on nothing either. A paid invoice moves it onto that period; an unpaid one
// leaves it past due.
func TestRenewSubscription_TheExistingInvoiceDecidesTheRow(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(c)
	db.SetNamespace("renew-settle")

	seed := func(user string, paid bool) (*subscription.Subscription, *billinginvoice.BillingInvoice) {
		now := time.Now()
		sub := subscription.New(db)
		sub.UserId = user
		sub.PlanId = "plan_pro"
		sub.Plan = plan.Plan{Name: "Pro", Price: currency.Cents(2500), Currency: currency.USD, Interval: types.Monthly, IntervalCount: 1}
		sub.Status = subscription.Active
		sub.PeriodStart, sub.PeriodEnd = now.AddDate(0, -1, 0), now.Add(-time.Hour)
		if err := sub.Create(); err != nil {
			t.Fatalf("create sub: %v", err)
		}
		inv, err := buildPeriodInvoice(db, sub, owed(sub, time.Now()), period{})
		if err != nil {
			t.Fatalf("invoice: %v", err)
		}
		if paid {
			if err := inv.MarkPaid("balance", "led_1"); err != nil {
				t.Fatalf("mark paid: %v", err)
			}
			if err := inv.Update(); err != nil {
				t.Fatalf("save invoice: %v", err)
			}
		}
		return sub, inv
	}
	pre := &purse{balance: 100000}

	// The owed period is paid, but the row was never moved onto it: it moves now,
	// and nothing is drawn.
	sub, inv := seed("renew-settle/paid", true)
	got, res, err := RenewSubscription(context.Background(), db, sub, pre, nil)
	if err != nil || got.Id() != inv.Id() || !res.Success {
		t.Fatalf("renew: inv=%v res=%+v err=%v, want the paid invoice back", got, res, err)
	}
	if sub.Status != subscription.Active || sub.PeriodStart.Unix() != inv.PeriodStart.Unix() || sub.CurrentInvoiceId != inv.Id() {
		t.Fatalf("row %s on %s (invoice %q), want active on the paid period %s", sub.Status, sub.PeriodStart, sub.CurrentInvoiceId, inv.PeriodStart)
	}

	// Unpaid: the row is past due, and nothing is drawn.
	sub, inv = seed("renew-settle/open", false)
	got, res, err = RenewSubscription(context.Background(), db, sub, pre, nil)
	if err != nil || got.Id() != inv.Id() || res.Success {
		t.Fatalf("renew: inv=%v res=%+v err=%v, want the open invoice back, unpaid", got, res, err)
	}
	if sub.Status != subscription.PastDue {
		t.Fatalf("status = %s, want past_due: an ended period with an unpaid invoice is not active", sub.Status)
	}
	if len(pre.draws) != 0 {
		t.Fatalf("drew %v; an existing invoice is never charged again here", pre.draws)
	}
}

// TestRenewSubscription_BillsTheNextPeriodInAdvance: the row holds the period it
// paid for. When that period ends the next one is invoiced — its whole fee, before
// it is served — and the row moves onto it only once it is paid. A plan canceled
// at the end of its period is therefore never served a period it did not pay for.
func TestRenewSubscription_BillsTheNextPeriodInAdvance(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(c)
	db.SetNamespace("renew-advance")

	start := time.Now().AddDate(0, 0, -3)
	sub := subscription.New(db)
	sub.UserId = "renew-advance/alice"
	sub.PlanId = "plan_pro"
	sub.Plan = plan.Plan{Name: "Pro", Price: currency.Cents(2500), Currency: currency.USD, Interval: types.Monthly, IntervalCount: 1}
	sub.Status = subscription.Active
	sub.PeriodStart, sub.PeriodEnd = start, Advance(start, &sub.Plan)
	if err := sub.Create(); err != nil {
		t.Fatalf("create sub: %v", err)
	}
	first, err := CreatePaidFirstInvoice(db, sub, "balance", "led_1")
	if err != nil {
		t.Fatalf("first invoice: %v", err)
	}
	if sub.PeriodStart.Unix() != start.Unix() || first.PeriodStart.Unix() != start.Unix() {
		t.Fatalf("after paying, row on %s and invoice for %s; both want the paid period %s", sub.PeriodStart, first.PeriodStart, start)
	}

	// Mid-period nothing is owed.
	pre := &purse{balance: 10000}
	if inv, _, _ := RenewSubscription(context.Background(), db, sub, pre, nil); inv != nil || len(pre.draws) != 0 {
		t.Fatalf("a renewal inside the paid period billed %v, drew %v", inv != nil, pre.draws)
	}

	// At its end, the next period is billed in full and the row moves onto it.
	end := sub.PeriodEnd
	sub.PeriodStart, sub.PeriodEnd = sub.PeriodStart.AddDate(0, -1, 0), time.Now().Add(-time.Minute)
	end = sub.PeriodEnd
	inv, res, err := RenewSubscription(context.Background(), db, sub, pre, nil)
	if err != nil || !res.Success {
		t.Fatalf("renew: %+v, %v", res, err)
	}
	if inv.PeriodStart.Unix() != end.Unix() || inv.PeriodEnd.Unix() != Advance(end, &sub.Plan).Unix() || inv.AmountPaid != 2500 {
		t.Fatalf("invoice %s..%s paid %d, want the period after %s paid 2500", inv.PeriodStart, inv.PeriodEnd, inv.AmountPaid, end)
	}
	if sub.PeriodStart.Unix() != end.Unix() || sub.CurrentInvoiceId != inv.Id() {
		t.Fatalf("row on %s (invoice %q), want the period it just paid for", sub.PeriodStart, sub.CurrentInvoiceId)
	}
}

// TestRenewSubscription_AMissedRowPaysOnlyThePeriodRunningNow: an active row whose
// renewal was missed for whole periods is billed the one running now, once; the
// periods that ended in between are not billed after the fact.
func TestRenewSubscription_AMissedRowPaysOnlyThePeriodRunningNow(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(c)
	db.SetNamespace("renew-missed")

	sub := subscription.New(db)
	sub.UserId = "renew-missed/alice"
	sub.PlanId = "plan_pro"
	sub.Plan = plan.Plan{Name: "Pro", Price: currency.Cents(2500), Currency: currency.USD, Interval: types.Monthly, IntervalCount: 1}
	sub.Status = subscription.Active
	sub.PeriodStart, sub.PeriodEnd = time.Now().AddDate(0, -4, 0), time.Now().AddDate(0, -3, 0)
	if err := sub.Create(); err != nil {
		t.Fatalf("create sub: %v", err)
	}
	pre := &purse{balance: 100000}
	inv, res, err := RenewSubscription(context.Background(), db, sub, pre, nil)
	if err != nil || !res.Success {
		t.Fatalf("renew: %+v, %v", res, err)
	}
	now := time.Now()
	if len(pre.draws) != 1 || pre.draws[0] != 2500 || inv.PeriodStart.After(now) || !inv.PeriodEnd.After(now) {
		t.Fatalf("drew %v for %s..%s; want one period, the one running now", pre.draws, inv.PeriodStart, inv.PeriodEnd)
	}
	if again, _, _ := RenewSubscription(context.Background(), db, sub, pre, nil); again != nil || len(pre.draws) != 1 {
		t.Fatalf("a second renewal billed again: %v, %v", again != nil, pre.draws)
	}
}

// TestVoidOpen_LeavesAnInvoiceBeingPaid: an invoice whose payment is in flight is
// not voided over it; one nobody is paying, with nothing paid toward it, is.
func TestVoidOpen_LeavesAnInvoiceBeingPaid(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := datastore.New(c)
	db.SetNamespace("void-open")

	sub := subscription.New(db)
	sub.UserId = "void-open/alice"
	sub.PlanId = "plan_pro"
	sub.Plan = plan.Plan{Name: "Pro", Price: currency.Cents(2500), Currency: currency.USD, Interval: types.Monthly, IntervalCount: 1}
	sub.Status = subscription.PastDue
	sub.PeriodStart, sub.PeriodEnd = time.Now().AddDate(0, -1, 0), time.Now().Add(-time.Hour)
	if err := sub.Create(); err != nil {
		t.Fatalf("create sub: %v", err)
	}
	paying, err := buildPeriodInvoice(db, sub, owed(sub, time.Now()), period{})
	if err != nil {
		t.Fatalf("invoice: %v", err)
	}
	if _, replay, err := idempotencykey.Begin(db, "billing-pay", "invoice:"+paying.Id()); err != nil || replay {
		t.Fatalf("hold the payment guard: %v %v", replay, err)
	}
	idle, err := buildPeriodInvoice(db, sub, period{sub.PeriodStart, sub.PeriodEnd}, period{})
	if err != nil {
		t.Fatalf("invoice: %v", err)
	}

	if err := VoidOpen(db, sub); err != nil {
		t.Fatalf("void: %v", err)
	}
	for _, tc := range []struct {
		inv  *billinginvoice.BillingInvoice
		want billinginvoice.Status
	}{{paying, billinginvoice.Open}, {idle, billinginvoice.Void}} {
		got := billinginvoice.New(db)
		if err := got.GetById(tc.inv.Id()); err != nil || got.Status != tc.want {
			t.Fatalf("invoice %s is %s (err %v), want %s", tc.inv.Id(), got.Status, err, tc.want)
		}
	}
}
