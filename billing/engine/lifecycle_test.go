package engine

import (
	"context"
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

	// Period advanced on success; a second renewal is a NEW period → invoice #2.
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
