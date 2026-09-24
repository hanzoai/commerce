package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	squarecore "github.com/square/square-go-sdk/v3/core"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// Adversarial cases of the renewal cycle: cancels racing a charge, a period
// already over, escalation, and a processor that does not answer.

// TestRed2_ImmediateCancelDuringChargeIsResurrected: the customer cancels
// immediately (not at period end) while the renewal charge is at Square. The
// canceled row stays canceled, and the month after is not charged.
func TestRed2_ImmediateCancelDuringChargeIsResurrected(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red2-cancel-now")
	sq := newRedSquare()
	withRedSquare(t, sq)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "red2-cancel-now", d)
	sq.before = func() {
		if _, err := cancelSubscription(ctx, org, sub.Id(), false); err != nil {
			t.Errorf("cancel: %v", err)
		}
	}

	cycleAt(t, ctx, org, d, false)
	charged := sq.captured()
	cycleAt(t, ctx, org, d.AddDate(0, 1, 0), false)
	if got := reloadSub(t, db, sub.Id()); sq.captured() > charged {
		t.Fatalf("a subscription canceled immediately during its renewal charge is %s again and the next month was charged "+
			"(%d cents after the cancel)", got.Status, sq.captured()-charged)
	}
}

// TestRed2_ImmediateCancelDuringDeclineIsRetried: the same cancel while the
// charge is declining. The canceled row stays canceled, its invoice is voided,
// and no retry charges it.
func TestRed2_ImmediateCancelDuringDeclineIsRetried(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red2-cancel-decline")
	sq := newRedSquare("decline", "ok")
	withRedSquare(t, sq)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "red2-cancel-decline", d)
	sq.before = func() {
		if _, err := cancelSubscription(ctx, org, sub.Id(), false); err != nil {
			t.Errorf("cancel: %v", err)
		}
	}

	cycleAt(t, ctx, org, d, false)
	status := reloadSub(t, db, sub.Id()).Status
	cycleAt(t, ctx, org, d.Add(24*time.Hour), false)
	if sq.captured() > 0 {
		t.Fatalf("a subscription canceled during a declined renewal was written back %s and its retry charged %d cents",
			status, sq.captured())
	}
}

// TestRed2_CatchUpChargeThenExpired: a row sits a period ahead of its paid
// first invoice, and the period it holds ended more than RenewalGrace before
// the cycle first runs. Before and after an operator realigns it, it is
// overdue: no run bills a period already over, and none ends the subscriber in
// the month they are using.
func TestRed2_CatchUpChargeThenExpired(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red2-catchup")
	sq := newRedSquare()
	withRedSquare(t, sq)
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now().Truncate(time.Second)
	p1 := now.AddDate(0, -1, 0).Add(-4 * 24 * time.Hour)
	sub := cardSubThrough(t, db, "red2-catchup", p1) // [P0, P1]
	if _, err := engine.CreatePaidFirstInvoice(db, sub, "card", "sqpay_first"); err != nil {
		t.Fatalf("first invoice: %v", err)
	}
	row := reloadSub(t, db, sub.Id())
	row.CurrentInvoiceId = sub.CurrentInvoiceId
	row.PeriodStart, row.PeriodEnd = p1, p1.AddDate(0, 1, 0) // a period ahead of the paid invoice
	if err := row.Update(); err != nil {
		t.Fatalf("shift: %v", err)
	}

	only(t, cycleAt(t, ctx, org, now, false), engine.Overdue) // the first run ever
	if r := Realign(ctx, []*organization.Organization{org}, false); r.Realigned != 1 {
		t.Fatalf("realigned %d rows, want the one", r.Realigned)
	}
	for _, at := range []time.Time{now.Add(time.Hour), now.AddDate(0, 0, 20)} {
		only(t, cycleAt(t, ctx, org, at, false), engine.Overdue)
	}
	if got := reloadSub(t, db, sub.Id()); sq.captured() != 0 || got.Status != subscription.Active {
		t.Fatalf("%d cents charged and the row is %s; want nothing charged for a period already over and the row left as it is",
			sq.captured(), got.Status)
	}
}

// hangSquare is a processor whose charge never answers until the caller's
// context ends (or a long fallback, so the test does not leak forever).
type hangSquare struct{ *MockSquareProcessor }

func (h hangSquare) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(8 * time.Second):
		return &processor.PaymentResult{Success: true, ProcessorRef: "sqpay_late", TransactionID: "sqpay_late", Status: "COMPLETED"}, nil
	}
}

// TestRed2_HungProcessorStallsTheCycle: the cycle runs on a context no request
// cancels, and the Square SDK's client has no deadline of its own. A charge that
// never answers is given up after chargeTimeout, so it cannot hold the org's
// cycle lock, and every org after it in run-all, indefinitely.
func TestRed2_HungProcessorStallsTheCycle(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red2-hang")
	h := hangSquare{newMockSquare(nil, "", nil)}
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(h)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
	oldTimeout := chargeTimeout
	chargeTimeout = time.Second
	t.Cleanup(func() { chargeTimeout = oldTimeout })
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	cardSubThrough(t, db, "red2-hang", d)

	done := make(chan struct{})
	go func() {
		defer close(done)
		cycleOrg(context.WithoutCancel(ctx), org, db, d, false, nil, chargeProviderForOrg(org), "", newCycleReport(d, false))
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		<-done // let the fallback answer before the context closes
		t.Fatal("the cycle waited on a processor that did not answer with no deadline of its own; a hung connection stalls the org, and every org after it in run-all")
	}
}

// darkSquare takes the first charge under a key and then answers 502 to every
// request, the first one included, while dark is set; once it clears, a repeat
// of a key it took answers with that payment.
type darkSquare struct {
	*MockSquareProcessor
	dark   bool
	landed map[string]int64
}

func (d *darkSquare) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	if _, ok := d.landed[req.IdempotencyKey]; !ok {
		d.landed[req.IdempotencyKey] = int64(req.Amount)
	}
	if d.dark {
		err := squarecore.NewAPIError(502, nil, errors.New("bad gateway"))
		return &processor.PaymentResult{Success: false, Error: err, ErrorMessage: err.Error()}, err
	}
	return &processor.PaymentResult{Success: true, ProcessorRef: "sqpay_" + req.IdempotencyKey, TransactionID: "sqpay_" + req.IdempotencyKey, Status: "COMPLETED"}, nil
}

// TestRed2_EscalatedThenPaidStaysUnpaid: a renewal charge lands and Square
// answers 5xx for longer than the retry schedule, so the cycle escalates the
// subscription to unpaid. When the answer is finally had — the customer pays the
// invoice, which repeats the pending attempt under its key and gets the payment
// back — the next run moves the row onto the period it paid for, active again.
func TestRed2_EscalatedThenPaidStaysUnpaid(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red2-escalated")
	sq := &darkSquare{MockSquareProcessor: newMockSquare(nil, "", nil), dark: true, landed: map[string]int64{}}
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(sq)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "red2-escalated", d)

	for _, at := range []time.Duration{0, 24 * time.Hour, 72 * time.Hour, 169 * time.Hour} {
		cycleAt(t, ctx, org, d.Add(at), false)
	}
	if got := reloadSub(t, db, sub.Id()); got.Status != subscription.Unpaid {
		t.Fatalf("after the retry schedule with no answer the row is %s, want escalated to unpaid", got.Status)
	}
	sq.dark = false
	inv := invoicesForSub(t, db, sub.Id())[0]
	_ = invokePay(org, ctx, inv.Id())
	cycleAt(t, ctx, org, d.Add(170*time.Hour), false)

	paid := invoicesForSub(t, db, sub.Id())[0].Status
	if got := reloadSub(t, db, sub.Id()); paid == billinginvoice.Paid && got.Status != subscription.Active {
		t.Fatalf("the renewal invoice is paid (%d cents taken) and the subscription is still %s", sq.landed[inv.PendingKey], got.Status)
	}
}
