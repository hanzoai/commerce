package billing

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// A wallet payment of usd cents (paid in paidCur when it was quoted), settled and
// credited through the real settlement path, against the fake ledger.
type paidWallet struct {
	ctx    context.Context
	org    *organization.Organization
	db     *datastore.Datastore
	led    *fakeLedger
	charge string
}

func settledWallet(ctx context.Context, t *testing.T, id string, usd, paid int64, paidCur string) *paidWallet {
	t.Helper()
	// Org names are lower case, as they are in the books the settlement and the
	// refunds both write to.
	org := organization.New(datastore.New(ctx))
	org.Name = "ref-" + strings.ToLower(id)
	if err := org.Create(); err != nil {
		t.Fatal(err)
	}
	led := newFakeLedger()
	injectLedger(t, led)
	if err := pendingintent.PutQuoted(id, org.Name, "payer", pendingintent.Topup, usd, "usd", paid, paidCur, 1, false); err != nil {
		t.Fatal(err)
	}
	charged, cur := usd, "usd"
	if paidCur != "" {
		charged, cur = paid, paidCur
	}
	useStripe(t, &stripeDouble{tx: &processor.Transaction{ID: id, ProcessorRef: "ch_" + id, Amount: currencyCents(charged), Currency: currency.Type(cur), Status: "succeeded"}})
	w := &paidWallet{ctx: ctx, org: org, db: datastore.New(org.Namespaced(ctx)), led: led, charge: "ch_" + id}
	if err := applySettlementEvent(ctx, w.db, org, succeeded(id, w.charge, charged, cur)); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if got := led.credits(); len(got) != 1 || got[0].AmountCents != usd {
		t.Fatalf("settled credit = %+v; want one of %d", got, usd)
	}
	return w
}

// refund delivers a processor refund of cents in cur against the payment.
func (w *paidWallet) refund(t *testing.T, id string, cents int64, cur string) {
	t.Helper()
	ev := &processor.WebhookEvent{ID: "evt_" + id, Type: "refund.updated", Processor: processor.Stripe,
		Data: map[string]interface{}{"id": id, "payment_id": w.charge, "status": "COMPLETED", "amount": float64(cents), "currency": cur, "stripe_test": false}}
	if err := applyClawbackEvent(w.ctx, w.db, w.org, ev); err != nil {
		t.Fatalf("refund %s: %v", id, err)
	}
}

func (w *paidWallet) dispute(t *testing.T, id, state string, version int64, cents int64, cur string) {
	t.Helper()
	ev := &processor.WebhookEvent{ID: "evt_" + id + state, Type: "dispute.updated", Processor: processor.Stripe,
		Data: map[string]interface{}{"id": id, "payment_id": w.charge, "state": state, "version": float64(version), "amount": float64(cents), "currency": cur, "stripe_test": false}}
	if err := applyClawbackEvent(w.ctx, w.db, w.org, ev); err != nil {
		t.Fatalf("dispute %s %s: %v", id, state, err)
	}
}

func (w *paidWallet) taken() (sum int64) {
	w.led.mu.Lock()
	defer w.led.mu.Unlock()
	for _, c := range w.led.clawbacks {
		sum += c.AmountCents
	}
	return sum
}

func (w *paidWallet) given() (sum int64) {
	w.led.mu.Lock()
	defer w.led.mu.Unlock()
	for _, c := range w.led.restores {
		sum += c.AmountCents
	}
	return sum
}

func (w *paidWallet) balance() int64 {
	b, _ := w.led.Balance(w.ctx, w.org.Name, "payer", "usd", false)
	return b
}

func TestAFullRefundReversesExactlyTheCredit(t *testing.T) {
	ctx := heldStore(t)
	w := settledWallet(ctx, t, "pi_full", 500, 0, "")
	w.refund(t, "re_full", 500, "usd")
	if w.taken() != 500 || w.balance() != 0 {
		t.Fatalf("a full refund took back %d, balance %d; want 500 and 0", w.taken(), w.balance())
	}
}

func TestPartialRefundsReverseTheirShareAndRepeatedOnesNeverExceedTheOriginal(t *testing.T) {
	ctx := heldStore(t)
	w := settledWallet(ctx, t, "pi_parts", 500, 0, "")
	w.refund(t, "re_a", 200, "usd")
	w.refund(t, "re_b", 200, "usd")
	if w.taken() != 400 {
		t.Fatalf("two partial refunds took back %d; want 400", w.taken())
	}
	// A third beyond what remains (100) takes only what remains, and a fourth nothing.
	w.refund(t, "re_c", 200, "usd")
	w.refund(t, "re_d", 200, "usd")
	if w.taken() != 500 || w.balance() != 0 {
		t.Fatalf("repeated refunds took back %d (balance %d); the sum can never exceed the original credit of 500", w.taken(), w.balance())
	}
}

func TestAReplayedRefundPostsNothingMore(t *testing.T) {
	ctx := heldStore(t)
	w := settledWallet(ctx, t, "pi_replay", 500, 0, "")
	w.refund(t, "re_once", 300, "usd")
	// The same refund object under a new event id, and the same event again.
	for i := 0; i < 3; i++ {
		w.refund(t, "re_once", 300, "usd")
	}
	if w.taken() != 300 || len(w.led.clawbacks) != 1 {
		t.Fatalf("a replayed refund took back %d in %d postings; want 300 in one", w.taken(), len(w.led.clawbacks))
	}
}

func TestARepeatedQuotedRefundNeverExceedsTheCredit(t *testing.T) {
	ctx := heldStore(t)
	w := settledWallet(ctx, t, "pi_qrep", 500, 462, "eur")
	w.refund(t, "re_q1", 462, "eur")
	w.refund(t, "re_q2", 100, "eur")
	if w.taken() != 500 {
		t.Fatalf("a full euro refund and a further 100 took back %d; want exactly 500", w.taken())
	}
}

func TestAPartialQuotedRefundIsItsShareOfTheDollarsCredited(t *testing.T) {
	ctx := heldStore(t)
	w := settledWallet(ctx, t, "pi_qpart", 500, 462, "eur")
	w.refund(t, "re_h1", 231, "eur")
	w.refund(t, "re_h2", 231, "eur")
	if w.taken() != 500 {
		t.Fatalf("two euro halves took back %d; want 500 between them", w.taken())
	}
}

// Units already spent are reversed in full against the ledger (which may go below zero,
// the money being gone either way) and the part the wallet could not cover is recorded
// by itself, once, as an unrecovered refund: never netted away, never dropped.
func TestARefundOfSpentUnitsRecordsTheShortfallExplicitly(t *testing.T) {
	ctx := heldStore(t)
	w := settledWallet(ctx, t, "pi_spent", 500, 0, "")
	if _, _, err := w.led.Debit(ctx, creditledger.DebitInput{Org: w.org.Name, Subject: "payer", Currency: "usd", Reason: "usage", Ref: "use-1", AmountCents: 400}); err != nil {
		t.Fatal(err)
	}
	w.refund(t, "re_spent", 500, "usd")
	w.refund(t, "re_spent", 500, "usd") // the replay records nothing more

	if w.taken() != 500 {
		t.Fatalf("the reversal posted %d; want the full 500", w.taken())
	}
	rows := make([]*billingevent.BillingEvent, 0)
	if _, err := billingevent.Query(w.db).Filter("Type=", "refund.unrecovered").GetAll(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want exactly one refund.unrecovered record, got %d", len(rows))
	}
	r := rows[0]
	if r.ObjectId != "refund:re_spent" || fmt.Sprint(r.Data["shortfallCents"]) != "400" || fmt.Sprint(r.Data["reversedCents"]) != "500" || fmt.Sprint(r.Data["payment"]) != w.charge || fmt.Sprint(r.Data["subject"]) != "payer" {
		t.Fatalf("the record does not state the payment, subject, reversal and shortfall: %+v", r.Data)
	}
}

func TestARefundTheWalletCoversRecordsNoShortfall(t *testing.T) {
	ctx := heldStore(t)
	w := settledWallet(ctx, t, "pi_covered", 500, 0, "")
	w.refund(t, "re_covered", 500, "usd")
	rows := make([]*billingevent.BillingEvent, 0)
	if _, err := billingevent.Query(w.db).Filter("Type=", "refund.unrecovered").GetAll(&rows); err != nil || len(rows) != 0 {
		t.Fatalf("a covered refund recorded a shortfall: %d rows (%v)", len(rows), err)
	}
}

// A dispute is taken and given back by the same rule: what its loss could take is
// capped by what earlier refunds left, and a win restores exactly that.
func TestADisputeWonRestoresExactlyWhatItsLossReversed(t *testing.T) {
	ctx := heldStore(t)
	w := settledWallet(ctx, t, "pi_disp", 500, 0, "")
	w.refund(t, "re_first", 300, "usd")
	w.dispute(t, "dp_1", "LOST", 1, 500, "usd")
	if w.taken() != 500 {
		t.Fatalf("a refund of 300 then a lost dispute of 500 took back %d; want the credit's 500 and no more", w.taken())
	}
	w.dispute(t, "dp_1", "WON", 2, 500, "usd")
	if w.given() != 200 {
		t.Fatalf("the win gave back %d; want exactly the 200 its loss reversed", w.given())
	}
	w.dispute(t, "dp_1", "WON", 2, 500, "usd")
	if w.given() != 200 {
		t.Fatalf("a replayed win gave back %d; want 200 once", w.given())
	}
}
