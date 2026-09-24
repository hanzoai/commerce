package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/meter"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/pricingrule"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/test/ae"
)

// card is a scripted card on file. It records every charge attempt with the
// invoice's AttemptCount as the charger saw it, which is the attempt part of the
// processor idempotency key the production charger builds.
type card struct {
	decline  bool
	unknown  bool
	attempts []int
	amounts  []int64
	// during runs while the charge is at the processor.
	during func()
}

func (c *card) charger() ProviderCharger {
	return func(_ context.Context, _ *datastore.Datastore, inv *billinginvoice.BillingInvoice, amount int64) (string, error) {
		c.attempts = append(c.attempts, inv.AttemptCount)
		c.amounts = append(c.amounts, amount)
		if c.during != nil {
			c.during()
		}
		if c.unknown {
			return "", fmt.Errorf("%w: i/o timeout", ErrChargeUnknown)
		}
		if c.decline {
			return "", errors.New("CARD_DECLINED")
		}
		return "sqpay_ok", nil
	}
}

// settleDB is a fresh org namespace.
func settleDB(t *testing.T, ns string) (*datastore.Datastore, func()) {
	t.Helper()
	c := ae.NewContext()
	db := datastore.New(c)
	db.SetNamespace(ns)
	return db, c.Close
}

// paidThrough stores an Active subscription on p, paid through `through`.
func paidThrough(t *testing.T, db *datastore.Datastore, p plan.Plan, through time.Time) *subscription.Subscription {
	t.Helper()
	sub := subscription.New(db)
	sub.UserId = "settle/alice"
	sub.PlanId = "plan_" + string(p.Interval)
	sub.Plan = p
	sub.Status = subscription.Active
	sub.PeriodEnd = through
	sub.PeriodStart = advancePeriodBack(through, &p)
	if err := sub.Create(); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	return sub
}

// advancePeriodBack is Advance run backwards, for seeding a paid period.
func advancePeriodBack(end time.Time, p *plan.Plan) time.Time {
	if p.Interval == types.Yearly {
		return end.AddDate(-1, 0, 0)
	}
	return end.AddDate(0, -1, 0)
}

var monthly = plan.Plan{Name: "Pro", Price: 2000, Currency: currency.USD, Interval: types.Monthly, IntervalCount: 1}

func storedInvoices(t *testing.T, db *datastore.Datastore, sub *subscription.Subscription) []*billinginvoice.BillingInvoice {
	t.Helper()
	out := make([]*billinginvoice.BillingInvoice, 0)
	q := billinginvoice.Query(db).Filter("SubscriptionId=", sub.Id())
	if _, err := q.GetAll(&out); err != nil {
		t.Fatalf("query invoices: %v", err)
	}
	return out
}

func reload(t *testing.T, db *datastore.Datastore, id string) *subscription.Subscription {
	t.Helper()
	s := subscription.New(db)
	if err := s.GetById(id); err != nil {
		t.Fatalf("reload subscription %s: %v", id, err)
	}
	return s
}

func settleAt(t *testing.T, db *datastore.Datastore, sub *subscription.Subscription, now time.Time, c *card, dryRun bool) *Step {
	t.Helper()
	return settleRun(t, db, sub, Run{Now: now, DryRun: dryRun}, c)
}

// settleRun is settleAt with the whole Run.
func settleRun(t *testing.T, db *datastore.Datastore, sub *subscription.Subscription, run Run, c *card) *Step {
	t.Helper()
	now := run.Now
	step, err := Settle(context.Background(), db, sub, run, CardPayer(c.charger()))
	if err != nil {
		t.Fatalf("settle at %s: %v", now, err)
	}
	return step
}

func TestIsDue_AtPeriodEnd(t *testing.T) {
	end := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := &subscription.Subscription{Status: subscription.Active, PeriodEnd: end}
	if IsDue(sub, end.Add(-time.Nanosecond)) {
		t.Fatal("due before the paid period ended")
	}
	if !IsDue(sub, end) {
		t.Fatal("not due at the end of the paid period")
	}
}

// TestCreatePaidFirstInvoice_KeepsThePaidPeriod: the first charge pays for the
// period the subscription opened on, so the row stays on it and is due at its end.
func TestCreatePaidFirstInvoice_KeepsThePaidPeriod(t *testing.T) {
	db, done := settleDB(t, "settle-first")
	defer done()
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	sub := subscription.New(db)
	sub.UserId = "settle/first"
	sub.Plan = monthly
	sub.Status = subscription.Active
	sub.PeriodStart, sub.PeriodEnd = start, start.AddDate(0, 1, 0)
	if err := sub.Create(); err != nil {
		t.Fatalf("create: %v", err)
	}
	inv, err := CreatePaidFirstInvoice(db, sub, "card", "sqpay_first")
	if err != nil {
		t.Fatalf("first invoice: %v", err)
	}
	if !inv.PeriodStart.Equal(start) || !inv.PeriodEnd.Equal(start.AddDate(0, 1, 0)) || inv.Status != billinginvoice.Paid {
		t.Fatalf("first invoice %s..%s %s, want the opening month paid", inv.PeriodStart, inv.PeriodEnd, inv.Status)
	}
	if !sub.PeriodStart.Equal(start) || !sub.PeriodEnd.Equal(start.AddDate(0, 1, 0)) || sub.CurrentInvoiceId != inv.Id() {
		t.Fatalf("subscription %s..%s invoice %q, want it left on the paid month", sub.PeriodStart, sub.PeriodEnd, sub.CurrentInvoiceId)
	}
}

// TestSettle_RenewsOnePeriodUpFront: a due subscription is charged once for the
// next period and moved onto it; settling again charges nothing.
func TestSettle_RenewsOnePeriodUpFront(t *testing.T) {
	db, done := settleDB(t, "settle-renew")
	defer done()
	through := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, through)
	c := &card{}

	step := settleAt(t, db, sub, through, c, false)
	if step.Action != Renewed || step.AmountCharged != 2000 {
		t.Fatalf("step = %s %d, want renewed 2000", step.Action, step.AmountCharged)
	}
	inv := step.Invoice
	next := through.AddDate(0, 1, 0)
	if !inv.PeriodStart.Equal(through) || !inv.PeriodEnd.Equal(next) || inv.Status != billinginvoice.Paid ||
		inv.AmountDue != 2000 || !inv.DueDate.Equal(through) {
		t.Fatalf("invoice %s..%s %s due %d at %s, want the next month paid at 2000", inv.PeriodStart, inv.PeriodEnd, inv.Status, inv.AmountDue, inv.DueDate)
	}
	got := reload(t, db, sub.Id())
	if !got.PeriodStart.Equal(through) || !got.PeriodEnd.Equal(next) || got.Status != subscription.Active || got.CurrentInvoiceId != inv.Id() {
		t.Fatalf("subscription %s..%s %s, want active on the next month", got.PeriodStart, got.PeriodEnd, got.Status)
	}

	if again := settleAt(t, db, got, through.Add(time.Hour), c, false); again.Action != "" {
		t.Fatalf("second settle = %s, want nothing due", again.Action)
	}
	if len(c.amounts) != 1 || len(storedInvoices(t, db, sub)) != 1 {
		t.Fatalf("charges=%d invoices=%d, want 1 and 1", len(c.amounts), len(storedInvoices(t, db, sub)))
	}
}

// TestSettle_GraceWindow: inside RenewalGrace the subscription renews one period
// from its PeriodEnd. Past it the row is overdue: nothing is charged, invoiced or
// written, however many periods were missed, and every run finds it the same way
// until the customer acts.
func TestSettle_GraceWindow(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		late time.Duration
		want Action
	}{
		{"71h late renews", 71 * time.Hour, Renewed},
		{"exactly at grace renews", RenewalGrace, Renewed},
		{"73h late is overdue", 73 * time.Hour, Overdue},
		{"three months late is overdue", 92 * 24 * time.Hour, Overdue},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, done := settleDB(t, "settle-grace")
			defer done()
			through := now.Add(-tc.late)
			sub := paidThrough(t, db, monthly, through)
			before := reload(t, db, sub.Id())
			c := &card{}

			step := settleAt(t, db, sub, now, c, false)
			if step.Action != tc.want {
				t.Fatalf("action = %s, want %s", step.Action, tc.want)
			}
			got := reload(t, db, sub.Id())
			switch tc.want {
			case Renewed:
				if len(c.amounts) != 1 || c.amounts[0] != 2000 {
					t.Fatalf("charges %v, want exactly one period (2000)", c.amounts)
				}
				if !got.PeriodStart.Equal(through) || !got.PeriodEnd.Equal(through.AddDate(0, 1, 0)) {
					t.Fatalf("renewed onto %s..%s, want one month from %s", got.PeriodStart, got.PeriodEnd, through)
				}
			case Overdue:
				if len(c.amounts) != 0 || len(storedInvoices(t, db, sub)) != 0 {
					t.Fatalf("charges=%v invoices=%d, want none", c.amounts, len(storedInvoices(t, db, sub)))
				}
				if got.Status != subscription.Active || !got.PeriodEnd.Equal(through) || !got.UpdatedAt.Equal(before.UpdatedAt) {
					t.Fatalf("subscription %s through %s, want it left active through %s and unwritten", got.Status, got.PeriodEnd, through)
				}
				if again := settleAt(t, db, got, now.AddDate(0, 1, 0), c, false); again.Action != Overdue || len(c.amounts) != 0 {
					t.Fatalf("a month later: %s with %d charges, want overdue and nothing charged", again.Action, len(c.amounts))
				}
			}
		})
	}
}

// TestSettle_OverdueRenewsFromNowWhenTheCustomerAsks: the customer renewing an
// overdue row pays one period, the one starting now. The months it was not
// renewed for are never billed, and the row is served the period it paid for.
func TestSettle_OverdueRenewsFromNowWhenTheCustomerAsks(t *testing.T) {
	for _, p := range []plan.Plan{monthly, {Name: "Pro", Price: 20000, Currency: currency.USD, Interval: types.Yearly, IntervalCount: 1}} {
		t.Run(string(p.Interval), func(t *testing.T) {
			db, done := settleDB(t, "settle-asked")
			defer done()
			now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
			through := now.AddDate(0, -3, 0)
			sub := paidThrough(t, db, p, through)
			c := &card{}

			step := settleRun(t, db, sub, Run{Now: now, Asked: true}, c)
			if step.Action != Renewed || len(c.amounts) != 1 || c.amounts[0] != int64(p.Price) {
				t.Fatalf("asked: %s charges %v, want renewed once at %d", step.Action, c.amounts, p.Price)
			}
			if !step.Invoice.PeriodStart.Equal(now) || !step.Invoice.PeriodEnd.Equal(Advance(now, &p)) {
				t.Fatalf("invoice %s..%s, want one period from now", step.Invoice.PeriodStart, step.Invoice.PeriodEnd)
			}
			got := reload(t, db, sub.Id())
			if got.Status != subscription.Active || !got.PeriodStart.Equal(now) || !got.PeriodEnd.Equal(Advance(now, &p)) {
				t.Fatalf("row %s %s..%s, want active on the period from now", got.Status, got.PeriodStart, got.PeriodEnd)
			}
			if n := len(storedInvoices(t, db, sub)); n != 1 {
				t.Fatalf("invoices %d, want only the period from now", n)
			}
		})
	}
}

// TestSettle_OverdueDeclinedWhenAskedRetriesOnSchedule: a customer's renewal of
// an overdue row that declines is retried on the schedule from that decline,
// on the invoice for the period from the ask, and drops to Free after the last.
func TestSettle_OverdueDeclinedWhenAskedRetriesOnSchedule(t *testing.T) {
	db, done := settleDB(t, "settle-asked-decline")
	defer done()
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, now.AddDate(0, -2, 0))
	c := &card{decline: true}

	if step := settleRun(t, db, sub, Run{Now: now, Asked: true}, c); step.Action != RenewalFailed {
		t.Fatalf("asked: %s, want renewal_failed", step.Action)
	}
	for _, st := range []struct {
		at   time.Duration
		want Action
	}{{time.Hour, Skipped}, {24 * time.Hour, RetryFailed}, {72 * time.Hour, RetryFailed}, {168 * time.Hour, Uncollectible}} {
		if step := settleAt(t, db, reload(t, db, sub.Id()), now.Add(st.at), c, false); step.Action != st.want {
			t.Fatalf("at +%s: %s, want %s", st.at, step.Action, st.want)
		}
	}
	if got := reload(t, db, sub.Id()); got.Status != subscription.Canceled || len(c.amounts) != 4 {
		t.Fatalf("row %s after %d charges, want canceled after 4", got.Status, len(c.amounts))
	}
}

// TestSettle_CancelAtPeriodEndChargesNothing.
func TestSettle_CancelAtPeriodEndChargesNothing(t *testing.T) {
	db, done := settleDB(t, "settle-endcancel")
	defer done()
	through := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, through)
	sub.EndCancel = true
	sub.CanceledAt = through.AddDate(0, 0, -10)
	if err := sub.Update(); err != nil {
		t.Fatalf("update: %v", err)
	}
	c := &card{}

	step := settleAt(t, db, sub, through.Add(time.Minute), c, false)
	got := reload(t, db, sub.Id())
	if step.Action != CanceledAtPeriodEnd || len(c.amounts) != 0 || len(storedInvoices(t, db, sub)) != 0 {
		t.Fatalf("action=%s charges=%v invoices=%d, want canceled_at_period_end with none", step.Action, c.amounts, len(storedInvoices(t, db, sub)))
	}
	if got.Status != subscription.Canceled || !got.Ended.Equal(through) || !got.CanceledAt.Equal(through.AddDate(0, 0, -10)) {
		t.Fatalf("subscription %s ended %s canceledAt %s, want canceled, ended at %s, request time kept", got.Status, got.Ended, got.CanceledAt, through)
	}
}

// TestSettle_RetryScheduleThenUncollectible: a declined renewal is retried at
// exactly +1d, +3d and +7d from the decline — never before, never twice for one
// point — each attempt at a new attempt count, and ends uncollectible after the
// last.
func TestSettle_RetryScheduleThenUncollectible(t *testing.T) {
	db, done := settleDB(t, "settle-retry")
	defer done()
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, d)
	c := &card{decline: true}

	steps := []struct {
		at      time.Duration
		want    Action
		charges int
	}{
		{0, RenewalFailed, 1},
		{24*time.Hour - time.Second, Skipped, 1},
		{24 * time.Hour, RetryFailed, 2},
		{24*time.Hour + time.Minute, Skipped, 2},
		{72*time.Hour - time.Second, Skipped, 2},
		{72 * time.Hour, RetryFailed, 3},
		{100 * time.Hour, Skipped, 3},
		{168*time.Hour - time.Second, Skipped, 3},
		{168 * time.Hour, Uncollectible, 4},
		{169 * time.Hour, "", 4},
	}
	for _, st := range steps {
		cur := reload(t, db, sub.Id())
		step := settleAt(t, db, cur, d.Add(st.at), c, false)
		if step.Action != st.want || len(c.amounts) != st.charges {
			t.Fatalf("at +%s: action=%s charges=%d, want %s and %d", st.at, step.Action, len(c.amounts), st.want, st.charges)
		}
		if st.want == RenewalFailed || st.want == RetryFailed || st.want == Skipped {
			if got := reload(t, db, sub.Id()); got.Status != subscription.PastDue || !got.PeriodEnd.Equal(d) {
				t.Fatalf("at +%s: subscription %s through %s, want past_due still through %s", st.at, got.Status, got.PeriodEnd, d)
			}
		}
	}
	for i, a := range c.attempts {
		if a != i {
			t.Fatalf("attempt counts seen by the charger = %v, want 0,1,2,3 — one fresh processor key per attempt", c.attempts)
		}
	}
	invs := storedInvoices(t, db, sub)
	if len(invs) != 1 || invs[0].Status != billinginvoice.Uncollectible || invs[0].AttemptCount != 4 {
		t.Fatalf("invoices=%d, want one uncollectible after 4 attempts", len(invs))
	}
	got := reload(t, db, sub.Id())
	if got.Status != subscription.Canceled || !got.Ended.Equal(d.Add(168*time.Hour)) || got.Metadata["endReason"] != string(Uncollectible) {
		t.Fatalf("subscription %s ended %s reason %v, want canceled at the last retry", got.Status, got.Ended, got.Metadata["endReason"])
	}
}

// TestSettle_RetrySucceeds: a retry that goes through pays the invoice and moves
// the subscription onto the period it paid for.
func TestSettle_RetrySucceeds(t *testing.T) {
	db, done := settleDB(t, "settle-retry-ok")
	defer done()
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, d)
	c := &card{decline: true}
	settleAt(t, db, sub, d, c, false)

	c.decline = false
	step := settleAt(t, db, reload(t, db, sub.Id()), d.Add(24*time.Hour), c, false)
	if step.Action != Retried || step.AmountCharged != 2000 || step.Invoice.Status != billinginvoice.Paid {
		t.Fatalf("step = %s %d %s, want retried 2000 paid", step.Action, step.AmountCharged, step.Invoice.Status)
	}
	got := reload(t, db, sub.Id())
	if got.Status != subscription.Active || !got.PeriodStart.Equal(d) || !got.PeriodEnd.Equal(d.AddDate(0, 1, 0)) {
		t.Fatalf("subscription %s %s..%s, want active on the paid month", got.Status, got.PeriodStart, got.PeriodEnd)
	}
	if c.attempts[0] == c.attempts[1] {
		t.Fatalf("the retry reused the declined attempt's count %v", c.attempts)
	}
}

// TestSettle_LateCycleRetriesOnce: a cycle that first comes back five days after
// the decline makes one retry, not the +1d and +3d ones back to back.
func TestSettle_LateCycleRetriesOnce(t *testing.T) {
	db, done := settleDB(t, "settle-late")
	defer done()
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, d)
	c := &card{decline: true}
	settleAt(t, db, sub, d, c, false)

	if step := settleAt(t, db, reload(t, db, sub.Id()), d.Add(120*time.Hour), c, false); step.Action != RetryFailed {
		t.Fatalf("late retry = %s, want retry_failed", step.Action)
	}
	if step := settleAt(t, db, reload(t, db, sub.Id()), d.Add(121*time.Hour), c, false); step.Action != Skipped {
		t.Fatalf("an hour later = %s, want skipped until +7d", step.Action)
	}
	if len(c.amounts) != 2 {
		t.Fatalf("charges=%d, want 2", len(c.amounts))
	}
	if inv := storedInvoices(t, db, sub)[0]; !inv.NextAttemptAt.Equal(d.Add(168 * time.Hour)) {
		t.Fatalf("next retry %s, want %s", inv.NextAttemptAt, d.Add(168*time.Hour))
	}
}

// TestSettle_AnnualRenewsAYear: a yearly plan renews at its annual price for a
// year.
func TestSettle_AnnualRenewsAYear(t *testing.T) {
	db, done := settleDB(t, "settle-annual")
	defer done()
	yearly := plan.Plan{Name: "Pro", Price: 20000, Currency: currency.USD, Interval: types.Yearly, IntervalCount: 1}
	through := time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, yearly, through)
	c := &card{}

	step := settleAt(t, db, sub, through, c, false)
	if step.Action != Renewed || len(c.amounts) != 1 || c.amounts[0] != 20000 {
		t.Fatalf("step=%s charges=%v, want renewed once at 20000", step.Action, c.amounts)
	}
	got := reload(t, db, sub.Id())
	if !got.PeriodStart.Equal(through) || !got.PeriodEnd.Equal(through.AddDate(1, 0, 0)) {
		t.Fatalf("renewed onto %s..%s, want a year", got.PeriodStart, got.PeriodEnd)
	}
	if step.Invoice.PeriodEnd != through.AddDate(1, 0, 0) {
		t.Fatalf("invoice covers %s..%s, want a year", step.Invoice.PeriodStart, step.Invoice.PeriodEnd)
	}
}

// TestSettle_PaidInvoiceMovesTheSubscriptionWithoutACharge: a renewal invoice the
// customer paid themselves (or whose charge landed while the subscription write
// did not) moves the subscription on and charges nothing.
func TestSettle_PaidInvoiceMovesTheSubscriptionWithoutACharge(t *testing.T) {
	db, done := settleDB(t, "settle-paid")
	defer done()
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, d)
	c := &card{decline: true}
	first := settleAt(t, db, sub, d, c, false)

	inv := first.Invoice
	if err := inv.MarkPaid("card", "sqpay_manual"); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if err := inv.Update(); err != nil {
		t.Fatalf("update: %v", err)
	}
	step := settleAt(t, db, reload(t, db, sub.Id()), d.Add(time.Hour), c, false)
	got := reload(t, db, sub.Id())
	if step.Action != Renewed || step.AmountCharged != 0 || len(c.amounts) != 1 {
		t.Fatalf("step=%s charged=%d charges=%d, want renewed with no new charge", step.Action, step.AmountCharged, len(c.amounts))
	}
	if got.Status != subscription.Active || !got.PeriodEnd.Equal(d.AddDate(0, 1, 0)) {
		t.Fatalf("subscription %s through %s, want active on the paid month", got.Status, got.PeriodEnd)
	}
}

// TestSettle_DryRunWritesNothing: a dry run reports what a real run does and
// changes nothing in the store.
func TestSettle_DryRunWritesNothing(t *testing.T) {
	db, done := settleDB(t, "settle-dry")
	defer done()
	now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	due := paidThrough(t, db, monthly, now.Add(-time.Hour))
	old := paidThrough(t, db, monthly, now.AddDate(0, -3, 0))
	c := &card{}

	for _, tc := range []struct {
		sub  *subscription.Subscription
		want Action
		cost int64
	}{{due, Renewed, 2000}, {old, Overdue, 0}} {
		before := reload(t, db, tc.sub.Id())
		charges := len(c.amounts)
		dry := settleAt(t, db, reload(t, db, tc.sub.Id()), now, c, true)
		after := reload(t, db, tc.sub.Id())
		if dry.Action != tc.want || dry.AmountCharged != tc.cost {
			t.Fatalf("dry run = %s %d, want %s %d", dry.Action, dry.AmountCharged, tc.want, tc.cost)
		}
		if after.Status != before.Status || !after.PeriodEnd.Equal(before.PeriodEnd) || !after.UpdatedAt.Equal(before.UpdatedAt) {
			t.Fatalf("dry run wrote the subscription: %s %s -> %s %s", before.Status, before.PeriodEnd, after.Status, after.PeriodEnd)
		}
		if len(c.amounts) != charges || len(storedInvoices(t, db, tc.sub)) != 0 {
			t.Fatalf("dry run charged %d times and stored %d invoices", len(c.amounts)-charges, len(storedInvoices(t, db, tc.sub)))
		}
		real := settleAt(t, db, after, now, c, false)
		if real.Action != dry.Action || real.AmountCharged != dry.AmountCharged {
			t.Fatalf("real run = %s %d, dry run said %s %d", real.Action, real.AmountCharged, dry.Action, dry.AmountCharged)
		}
	}
}

// TestSettle_UsageIsBilledForThePeriodThatEnded: the renewal invoice carries the
// plan fee for the period ahead and the metered usage of the period just paid
// for, and none from before it.
func TestSettle_UsageIsBilledForThePeriodThatEnded(t *testing.T) {
	db, done := settleDB(t, "settle-usage")
	defer done()
	through := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, through)

	m := meter.New(db)
	m.Name, m.EventName, m.AggregationType, m.Currency = "Queries", "queries", meter.AggSum, currency.USD
	if err := m.Create(); err != nil {
		t.Fatalf("meter: %v", err)
	}
	rule := pricingrule.New(db)
	rule.MeterId, rule.PricingType, rule.UnitPrice, rule.Currency = m.Id(), pricingrule.PerUnit, 3, currency.USD
	if err := rule.Create(); err != nil {
		t.Fatalf("rule: %v", err)
	}
	for _, e := range []struct {
		at    time.Time
		value int64
	}{
		{through.AddDate(0, 0, -10), 10}, // inside the paid month: billed
		{through.AddDate(0, -2, 0), 100}, // before it: already billed
	} {
		ev := meter.NewEvent(db)
		ev.MeterId, ev.UserId, ev.Value, ev.Timestamp = m.Id(), sub.UserId, e.value, e.at
		if err := ev.Create(); err != nil {
			t.Fatalf("event: %v", err)
		}
	}

	c := &card{}
	step := settleAt(t, db, sub, through, c, false)
	if step.Action != Renewed || step.Invoice.AmountDue != 2030 || c.amounts[0] != 2030 {
		t.Fatalf("step=%s due=%d charged=%v, want 2000 plan + 30 usage", step.Action, step.Invoice.AmountDue, c.amounts)
	}
}

// TestSettle_UnknownOutcomeRepeatsTheSameAttempt: a charge whose outcome the
// processor did not report is not a decline. It is repeated at the same attempt
// count (the same processor key) on the next run, however late, and the grace
// window never voids its invoice.
func TestSettle_UnknownOutcomeRepeatsTheSameAttempt(t *testing.T) {
	db, done := settleDB(t, "settle-unknown")
	defer done()
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, d)
	c := &card{unknown: true}

	if step := settleAt(t, db, sub, d, c, false); step.Action != Skipped {
		t.Fatalf("unknown outcome = %s, want skipped", step.Action)
	}
	if got := reload(t, db, sub.Id()); got.Status != subscription.PastDue {
		t.Fatalf("subscription %s, want past_due while the charge is unresolved", got.Status)
	}
	inv := storedInvoices(t, db, sub)[0]
	if inv.AttemptCount != 0 || inv.LastAttemptAt.IsZero() || !inv.NextAttemptAt.Equal(d) {
		t.Fatalf("invoice attempts=%d last=%s next=%s, want none counted and a repeat due now", inv.AttemptCount, inv.LastAttemptAt, inv.NextAttemptAt)
	}

	c.unknown = false
	step := settleAt(t, db, reload(t, db, sub.Id()), d.Add(100*time.Hour), c, false)
	if step.Action != Retried || len(c.attempts) != 2 || c.attempts[0] != 0 || c.attempts[1] != 0 {
		t.Fatalf("repeat = %s with attempt counts %v, want retried at the same count 0", step.Action, c.attempts)
	}
}

// TestSettle_FindsAnInvoiceWrittenBackWithoutItsParent: a path that reads an
// invoice by id and writes it back stores it without the synckey ancestor. The
// cycle still finds it as the period's invoice, so the period is never
// invoiced or charged a second time.
func TestSettle_FindsAnInvoiceWrittenBackWithoutItsParent(t *testing.T) {
	db, done := settleDB(t, "settle-orphan")
	defer done()
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, d)
	c := &card{decline: true}
	settleAt(t, db, sub, d, c, false)

	id := storedInvoices(t, db, sub)[0].Id()
	w := billinginvoice.New(db)
	if err := w.GetById(id); err != nil {
		t.Fatalf("read invoice: %v", err)
	}
	w.CustomerEmail = "alice@example.test"
	if err := w.Update(); err != nil {
		t.Fatalf("write invoice: %v", err)
	}

	c.decline = false
	step := settleAt(t, db, reload(t, db, sub.Id()), d.Add(24*time.Hour), c, false)
	all := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := billinginvoice.Query(db).Filter("SubscriptionId=", sub.Id()).GetAll(&all); err != nil {
		t.Fatalf("query: %v", err)
	}
	if step.Action != Retried || step.Invoice.Id() != id || len(all) != 1 || len(c.amounts) != 2 {
		t.Fatalf("step=%s invoice=%s invoices=%d charges=%d, want the same invoice retried once", step.Action, step.Invoice.Id(), len(all), len(c.amounts))
	}
}

// TestSettle_CompedMovesOnWithoutACharge: a comped subscription is never
// charged or ended; once its period ends it moves on to the period holding now,
// however late.
func TestSettle_CompedMovesOnWithoutACharge(t *testing.T) {
	db, done := settleDB(t, "settle-comped")
	defer done()
	t0 := time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, t0.AddDate(0, 1, 0))
	if _, err := CreatePaidFirstInvoice(db, sub, "credit", "credit_burn"); err != nil {
		t.Fatalf("first invoice: %v", err)
	}
	now := t0.AddDate(0, 5, 3)
	comped := Collection{Comped: true, Reason: "a gift"}

	for i := 0; i < 2; i++ {
		step, err := Settle(context.Background(), db, reload(t, db, sub.Id()), Run{Now: now}, comped)
		if err != nil {
			t.Fatalf("settle: %v", err)
		}
		want := Comped
		if i == 1 {
			want = ""
		}
		if step.Action != want {
			t.Fatalf("run %d: %q, want %q", i, step.Action, want)
		}
	}
	got := reload(t, db, sub.Id())
	if got.Status != subscription.Active || now.Before(got.PeriodStart) || !now.Before(got.PeriodEnd) {
		t.Fatalf("row %s %s..%s, want active on the period holding %s", got.Status, got.PeriodStart, got.PeriodEnd, now)
	}
	if n := len(storedInvoices(t, db, sub)); n != 1 {
		t.Fatalf("invoices = %d, want only the first", n)
	}
}

// TestSettle_CancelAtPeriodEndEndsACompedRow.
func TestSettle_CancelAtPeriodEndEndsACompedRow(t *testing.T) {
	db, done := settleDB(t, "settle-comped-cancel")
	defer done()
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, d)
	sub.EndCancel = true
	if err := sub.Update(); err != nil {
		t.Fatalf("update: %v", err)
	}
	step, err := Settle(context.Background(), db, sub, Run{Now: d}, Collection{Comped: true})
	if err != nil || step.Action != CanceledAtPeriodEnd {
		t.Fatalf("step %v (err %v), want canceled_at_period_end", step, err)
	}
}

// TestSettle_NothingToPayEndsAtPeriodEnd: with nothing on file to pay it, a due
// subscription ends at the end of its paid period and nothing is invoiced.
func TestSettle_NothingToPayEndsAtPeriodEnd(t *testing.T) {
	db, done := settleDB(t, "settle-nopay")
	defer done()
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, d)

	step, err := Settle(context.Background(), db, sub, Run{Now: d.Add(time.Minute)}, Collection{Reason: "paid outside Hanzo, no card"})
	if err != nil || step.Action != EndedNoCard {
		t.Fatalf("step %v (err %v), want ended_no_card", step, err)
	}
	got := reload(t, db, sub.Id())
	if got.Status != subscription.Canceled || !got.Ended.Equal(d) || len(storedInvoices(t, db, sub)) != 0 {
		t.Fatalf("row %s ended %s with %d invoices, want canceled at %s with none", got.Status, got.Ended, len(storedInvoices(t, db, sub)), d)
	}
}

// TestSettle_DryRunReportsAPreviewedDecline: a dry run asks the Collection's
// Choose and reports the decline it answers, writing nothing.
func TestSettle_DryRunReportsAPreviewedDecline(t *testing.T) {
	db, done := settleDB(t, "settle-preview")
	defer done()
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, d)
	c := &card{}
	short := CardPayer(c.charger())
	short.Choose = func(context.Context, *datastore.Datastore, *billinginvoice.BillingInvoice, int64) (string, error) {
		return "", errors.New("prepaid balance is short")
	}
	step, err := Settle(context.Background(), db, sub, Run{Now: d, DryRun: true}, short)
	if err != nil || step.Action != RenewalFailed || step.Reason != "prepaid balance is short" {
		t.Fatalf("dry run %v (err %v), want renewal_failed with the preview's reason", step, err)
	}
	if len(c.amounts) != 0 || len(storedInvoices(t, db, sub)) != 0 {
		t.Fatal("a dry run moved money or stored an invoice")
	}
}

// TestPay_RecordsTheAttemptBeforePaying: the attempt — instrument, amount, key —
// is on the stored invoice before any money is asked for, and a repeat after
// an unknown outcome asks for exactly the same thing.
func TestPay_RecordsTheAttemptBeforePaying(t *testing.T) {
	db, done := settleDB(t, "pay-record")
	defer done()
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, d)
	inv, err := draftPeriodInvoice(db, sub, d, d.AddDate(0, 1, 0), time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if err := issuePeriodInvoice(db, inv); err != nil {
		t.Fatalf("issue: %v", err)
	}

	var seen []string
	c := Collection{
		Choose: func(context.Context, *datastore.Datastore, *billinginvoice.BillingInvoice, int64) (string, error) {
			return "card", nil
		},
		Pay: func(_ context.Context, db *datastore.Datastore, got *billinginvoice.BillingInvoice, method string, amount int64) (string, error) {
			stored, err := loadInvoice(db, got.Id())
			if err != nil || stored.PendingKey == "" || stored.PendingMethod != method || stored.PendingAmount != amount {
				t.Fatalf("paying before the attempt was stored: %+v (err %v)", stored, err)
			}
			seen = append(seen, stored.PendingKey)
			return "", fmt.Errorf("%w: 502", ErrChargeUnknown)
		},
	}
	for i := 0; i < 2; i++ {
		res, err := Pay(context.Background(), db, inv, c, d)
		if err != nil || !res.Unknown {
			t.Fatalf("pay %d: %+v (err %v), want unknown", i, res, err)
		}
	}
	if len(seen) != 2 || seen[0] != seen[1] || inv.AttemptCount != 0 {
		t.Fatalf("keys %v, attempts %d: an unknown outcome must repeat under its key and not count", seen, inv.AttemptCount)
	}
}

// TestCollectInvoice_UnknownCardLegIsNotCounted: the waterfall's card leg with
// an unknown outcome leaves the attempt uncounted, so its next call carries the
// same key.
func TestCollectInvoice_UnknownCardLegIsNotCounted(t *testing.T) {
	inv := &billinginvoice.BillingInvoice{}
	inv.Status = billinginvoice.Open
	inv.AmountDue = 1000
	unknown := ProviderCharger(func(context.Context, *datastore.Datastore, *billinginvoice.BillingInvoice, int64) (string, error) {
		return "", fmt.Errorf("%w: timeout", ErrChargeUnknown)
	})
	res, err := CollectInvoice(nil, nil, inv, nil, unknown)
	if err != nil || !res.Unknown || res.Success || inv.AttemptCount != 0 {
		t.Fatalf("result %+v attempts %d (err %v), want an uncounted unknown", res, inv.AttemptCount, err)
	}
}

// issueFor stores an open invoice for sub over [start, end].
func issueFor(t *testing.T, db *datastore.Datastore, sub *subscription.Subscription, start, end time.Time) *billinginvoice.BillingInvoice {
	t.Helper()
	inv, err := draftPeriodInvoice(db, sub, start, end, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if err := issuePeriodInvoice(db, inv); err != nil {
		t.Fatalf("issue: %v", err)
	}
	return inv
}

// TestFirstInvoice_SkipsInvoicesWithNoPeriod: a one-off invoice filed against
// the subscription with no period is not how it was bought, however early its
// zero period start sorts.
func TestFirstInvoice_SkipsInvoicesWithNoPeriod(t *testing.T) {
	db, done := settleDB(t, "settle-first-no-period")
	defer done()
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, start.AddDate(0, 1, 0))
	bought, err := CreatePaidFirstInvoice(db, sub, "card", "sqpay_first")
	if err != nil {
		t.Fatalf("first invoice: %v", err)
	}
	oneOff := billinginvoice.New(db)
	oneOff.SubscriptionId, oneOff.UserId = sub.Id(), sub.UserId
	oneOff.AmountDue, oneOff.AmountPaid, oneOff.Status = 500, 500, billinginvoice.Paid
	if err := oneOff.Create(); err != nil {
		t.Fatalf("one-off invoice: %v", err)
	}
	if n := len(storedInvoices(t, db, sub)); n != 2 {
		t.Fatalf("%d invoices stored, want the purchase and the one-off", n)
	}
	first, err := FirstInvoice(db, sub)
	if err != nil || first == nil || first.Id() != bought.Id() {
		t.Fatalf("first invoice %v (err %v), want the purchase %s", first, err, bought.Id())
	}
}

// TestSettle_PrefersThePaidInvoiceForAPeriod: with a paid and an open invoice
// for the same period, the paid one moves the subscription on and nothing is
// charged.
func TestSettle_PrefersThePaidInvoiceForAPeriod(t *testing.T) {
	db, done := settleDB(t, "settle-prefer-paid")
	defer done()
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, d)
	issueFor(t, db, sub, d, d.AddDate(0, 1, 0))
	paid := issueFor(t, db, sub, d, d.AddDate(0, 1, 0))
	if err := paid.MarkPaid("card", "sqpay_x"); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if err := paid.Update(); err != nil {
		t.Fatalf("update: %v", err)
	}
	c := &card{}
	step := settleAt(t, db, sub, d, c, false)
	if step.Action != Renewed || step.Invoice.Id() != paid.Id() || len(c.amounts) != 0 {
		t.Fatalf("step %s on %s with %d charges, want renewed on the paid invoice with none", step.Action, step.Invoice.Id(), len(c.amounts))
	}
}

// TestSettle_IntervalChangeVoidsTheStaleInvoiceOrFollowsTheTriedOne: once the
// plan's interval changed, a renewal invoice issued for the old period and never
// attempted is voided and the new period billed; one a payment was attempted on
// is followed to its own period rather than billed again.
func TestSettle_IntervalChangeVoidsTheStaleInvoiceOrFollowsTheTriedOne(t *testing.T) {
	yearly := plan.Plan{Name: "Pro", Price: 20000, Currency: currency.USD, Interval: types.Yearly, IntervalCount: 1}
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	t.Run("never attempted", func(t *testing.T) {
		db, done := settleDB(t, "settle-interval-stale")
		defer done()
		sub := paidThrough(t, db, monthly, d)
		old := issueFor(t, db, sub, d, d.AddDate(0, 1, 0))
		sub.Plan = yearly
		if err := sub.Update(); err != nil {
			t.Fatalf("change plan: %v", err)
		}
		c := &card{}
		step := settleAt(t, db, reload(t, db, sub.Id()), d, c, false)
		if step.Action != Renewed || len(c.amounts) != 1 || c.amounts[0] != 20000 {
			t.Fatalf("step %s charges %v, want the year renewed at 20000", step.Action, c.amounts)
		}
		if got, _ := loadInvoice(db, old.Id()); got.Status != billinginvoice.Void {
			t.Fatalf("stale invoice %s, want void", got.Status)
		}
		if got := reload(t, db, sub.Id()); !got.PeriodEnd.Equal(d.AddDate(1, 0, 0)) {
			t.Fatalf("row through %s, want a year", got.PeriodEnd)
		}
	})

	t.Run("attempted", func(t *testing.T) {
		db, done := settleDB(t, "settle-interval-tried")
		defer done()
		sub := paidThrough(t, db, monthly, d)
		c := &card{decline: true}
		settleAt(t, db, sub, d, c, false) // declined: an attempt on the month's invoice
		row := reload(t, db, sub.Id())
		row.Plan = yearly
		if err := row.Update(); err != nil {
			t.Fatalf("change plan: %v", err)
		}
		c.decline = false
		step := settleAt(t, db, reload(t, db, sub.Id()), d.Add(24*time.Hour), c, false)
		if step.Action != Retried || c.amounts[len(c.amounts)-1] != 2000 || len(storedInvoices(t, db, sub)) != 1 {
			t.Fatalf("step %s charges %v invoices %d, want the month's invoice retried at 2000 and nothing new", step.Action, c.amounts, len(storedInvoices(t, db, sub)))
		}
		if got := reload(t, db, sub.Id()); !got.PeriodEnd.Equal(d.AddDate(0, 1, 0)) {
			t.Fatalf("row through %s, want the month the invoice paid for", got.PeriodEnd)
		}
	})
}

// TestSettle_NeverBillsAPeriodThatIsOver: a declined renewal whose retry
// comes only after the period it would pay for has ended is voided and the
// subscription ended, with nothing charged.
func TestSettle_NeverBillsAPeriodThatIsOver(t *testing.T) {
	db, done := settleDB(t, "settle-over")
	defer done()
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, d)
	c := &card{decline: true}
	settleAt(t, db, sub, d, c, false)

	c.decline = false
	step := settleAt(t, db, reload(t, db, sub.Id()), d.AddDate(0, 1, 1), c, false)
	if step.Action != ExpiredOverdue || len(c.amounts) != 1 {
		t.Fatalf("step %s with %d charges, want expired_overdue and only the declined attempt", step.Action, len(c.amounts))
	}
	if inv, _ := loadInvoice(db, storedInvoices(t, db, sub)[0].Id()); inv.Status != billinginvoice.Void {
		t.Fatalf("invoice %s, want void", inv.Status)
	}
}

// An immediate cancel that lands while the renewal is at the processor keeps the
// row canceled whatever the charge answers: a payment is kept on its invoice and
// reported for a refund, a decline voids the invoice so nothing retries it, and
// an unknown outcome keeps its attempt for reconciliation.
func TestSettle_ImmediateCancelDuringTheChargeStaysCanceled(t *testing.T) {
	through := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		card    card
		action  Action
		invoice billinginvoice.Status
		pending bool
	}{
		{"paid", card{}, ChargedAfterCancel, billinginvoice.Paid, false},
		{"declined", card{decline: true}, Skipped, billinginvoice.Void, false},
		{"unknown", card{unknown: true}, Skipped, billinginvoice.Open, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, done := settleDB(t, "settle-cancel-during-"+tc.name)
			defer done()
			sub := paidThrough(t, db, monthly, through)
			canceledAt := through.Add(time.Minute)
			c := tc.card
			c.during = func() {
				row := reload(t, db, sub.Id())
				row.Status, row.Canceled, row.CanceledAt, row.Ended = subscription.Canceled, true, canceledAt, canceledAt
				if err := row.Update(); err != nil {
					t.Fatalf("cancel: %v", err)
				}
			}
			step := settleAt(t, db, sub, through.Add(time.Hour), &c, false)
			if step.Action != tc.action || len(c.amounts) != 1 {
				t.Fatalf("action=%s charges=%v, want %s after one charge", step.Action, c.amounts, tc.action)
			}
			if tc.action == ChargedAfterCancel && step.AmountCharged != int64(monthly.Price) {
				t.Fatalf("charged %d, want %d reported for a refund", step.AmountCharged, monthly.Price)
			}
			got := reload(t, db, sub.Id())
			if got.Status != subscription.Canceled || !got.Ended.Equal(canceledAt) || !got.PeriodEnd.Equal(through) {
				t.Fatalf("row %s ended %s through %s, want canceled at %s through %s", got.Status, got.Ended, got.PeriodEnd, canceledAt, through)
			}
			if sub.Status != subscription.Canceled {
				t.Fatalf("row in hand %s, want canceled", sub.Status)
			}
			invs := storedInvoices(t, db, sub)
			if len(invs) != 1 || invs[0].Status != tc.invoice || (invs[0].PendingKey != "") != tc.pending {
				t.Fatalf("invoices %+v, want one %s (pending attempt kept: %v)", invs, tc.invoice, tc.pending)
			}
			if again := settleAt(t, db, got, through.Add(25*time.Hour), &c, false); again.Action != "" || len(c.amounts) != 1 {
				t.Fatalf("next run %s charges=%v, want a canceled row left alone", again.Action, c.amounts)
			}
		})
	}
}

// escalated runs an unknown charge past the retry schedule on a row paid
// through d, so it ends unpaid with the attempt recorded on its invoice.
func escalated(t *testing.T, db *datastore.Datastore, d time.Time, c *card) *subscription.Subscription {
	t.Helper()
	sub := paidThrough(t, db, monthly, d)
	c.unknown = true
	for _, at := range []time.Duration{0, 24 * time.Hour, 72 * time.Hour} {
		settleAt(t, db, reload(t, db, sub.Id()), d.Add(at), c, false)
	}
	if step := settleAt(t, db, reload(t, db, sub.Id()), d.Add(169*time.Hour), c, false); step.Action != Escalated {
		t.Fatalf("after the schedule: %s, want escalated", step.Action)
	}
	if got := reload(t, db, sub.Id()); got.Status != subscription.Unpaid {
		t.Fatalf("row %s, want unpaid", got.Status)
	}
	return sub
}

// An escalated row stays in the cycle: while the attempt has no known outcome
// each run repeats it under its key and reports it skipped, and once the
// processor answers the payment moves the row onto the paid period and back to
// active.
func TestSettle_EscalatedRowResolvesWhenTheProcessorAnswers(t *testing.T) {
	db, done := settleDB(t, "settle-escalated-paid")
	defer done()
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	c := &card{}
	sub := escalated(t, db, d, c)

	if step := settleAt(t, db, reload(t, db, sub.Id()), d.Add(200*time.Hour), c, false); step.Action != Skipped {
		t.Fatalf("still unknown: %s, want skipped with no second escalation", step.Action)
	}
	c.unknown = false
	step := settleAt(t, db, reload(t, db, sub.Id()), d.Add(250*time.Hour), c, false)
	if step.Action != Retried || step.AmountCharged != int64(monthly.Price) {
		t.Fatalf("answered: %s charged %d, want retried for the plan", step.Action, step.AmountCharged)
	}
	for i, n := range c.attempts {
		if n != 0 {
			t.Fatalf("attempt %d at count %d, want every repeat under the first attempt's key", i, n)
		}
	}
	got := reload(t, db, sub.Id())
	if got.Status != subscription.Active || !got.PeriodStart.Equal(d) || !got.PeriodEnd.Equal(d.AddDate(0, 1, 0)) {
		t.Fatalf("row %s [%s, %s], want active on the paid period", got.Status, got.PeriodStart, got.PeriodEnd)
	}
}

// An escalated row whose invoice was paid elsewhere (the customer paid it) or
// voided (an operator reconciled it) is settled by the next run: onto the paid
// period, or ended.
func TestSettle_EscalatedRowFollowsItsInvoice(t *testing.T) {
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		mark   func(*billinginvoice.BillingInvoice) error
		action Action
		status subscription.Status
	}{
		{"paid", func(inv *billinginvoice.BillingInvoice) error {
			inv.AmountPaid = inv.AmountDue
			return inv.MarkPaid("card", "sqpay_customer")
		}, Renewed, subscription.Active},
		{"voided", func(inv *billinginvoice.BillingInvoice) error { return inv.MarkVoid() }, EndedInvoiceVoided, subscription.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, done := settleDB(t, "settle-escalated-"+tc.name)
			defer done()
			c := &card{}
			sub := escalated(t, db, d, c)
			inv, err := loadInvoice(db, storedInvoices(t, db, sub)[0].Id())
			if err != nil {
				t.Fatal(err)
			}
			inv.PendingMethod, inv.PendingAmount, inv.PendingKey, inv.PendingRef = "", 0, "", ""
			if err := tc.mark(inv); err != nil {
				t.Fatal(err)
			}
			if err := inv.Update(); err != nil {
				t.Fatal(err)
			}
			calls := len(c.attempts)
			step := settleAt(t, db, reload(t, db, sub.Id()), d.Add(200*time.Hour), c, false)
			if got := reload(t, db, sub.Id()); step.Action != tc.action || got.Status != tc.status || len(c.attempts) != calls {
				t.Fatalf("%s: row %s, charges %d, want %s leaving it %s with no charge", step.Action, got.Status, len(c.attempts)-calls, tc.action, tc.status)
			}
		})
	}
}

// A cancel at period end, or a Collection that no longer pays, never voids an
// invoice whose payment attempt has no known outcome: the attempt is repeated
// first when the Collection can, and escalated when it cannot.
func TestSettle_APendingAttemptIsResolvedBeforeAnEnd(t *testing.T) {
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	t.Run("cancel at period end", func(t *testing.T) {
		db, done := settleDB(t, "settle-pending-endcancel")
		defer done()
		sub := paidThrough(t, db, monthly, d)
		c := &card{unknown: true}
		settleAt(t, db, sub, d, c, false)
		row := reload(t, db, sub.Id())
		row.EndCancel = true
		if err := row.Update(); err != nil {
			t.Fatal(err)
		}
		c.unknown = false
		step := settleAt(t, db, reload(t, db, sub.Id()), d.Add(time.Hour), c, false)
		got := reload(t, db, sub.Id())
		if step.Action != Retried || got.Status != subscription.Active || !got.PeriodEnd.Equal(d.AddDate(0, 1, 0)) || !got.EndCancel {
			t.Fatalf("%s: row %s through %s (end-cancel %v), want the landed payment kept and the row served through it, ending then",
				step.Action, got.Status, got.PeriodEnd, got.EndCancel)
		}
		if step := settleAt(t, db, got, d.AddDate(0, 1, 0), c, false); step.Action != CanceledAtPeriodEnd {
			t.Fatalf("at the paid period's end: %s, want canceled_at_period_end", step.Action)
		}
	})
	t.Run("nothing to repeat it with", func(t *testing.T) {
		db, done := settleDB(t, "settle-pending-nopay")
		defer done()
		sub := paidThrough(t, db, monthly, d)
		settleAt(t, db, sub, d, &card{unknown: true}, false)
		step, err := Settle(context.Background(), db, reload(t, db, sub.Id()), Run{Now: d.Add(time.Hour)},
			Collection{End: EndedNoCard, Reason: "no card on file"})
		if err != nil {
			t.Fatal(err)
		}
		inv := storedInvoices(t, db, sub)[0]
		if got := reload(t, db, sub.Id()); step.Action != Escalated || got.Status != subscription.Unpaid ||
			inv.Status != billinginvoice.Open || inv.PendingKey == "" {
			t.Fatalf("%s: row %s, invoice %s pending %q, want escalated with the attempt kept on an open invoice",
				step.Action, got.Status, inv.Status, inv.PendingKey)
		}
	})
}
