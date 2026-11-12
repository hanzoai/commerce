package billing

// The rest of red's probes against commerce#13 (TestRed13_*), adopted: a sale that
// stops part-way, a payment index that is missing, an order sent again under
// another key or after a catalog edit, a key reused for another seat mix, and
// refunds that land together. A stop is a real write failing, never an edited row.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	_ "unsafe"

	"github.com/hanzoai/commerce/datastore"
	dbpkg "github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

//go:linkname idemNow github.com/hanzoai/commerce/models/idempotencykey.nowFn
var idemNow func() time.Time

// restarted moves the guards' clock past StartedTTL for the rest of the test, as a
// process that restarted finds them.
func restarted(t *testing.T) {
	old := idemNow
	idemNow = func() time.Time { return time.Now().Add(10 * time.Minute) }
	t.Cleanup(func() { idemNow = old })
}

// writesFail is the store with the next n writes of one kind failing, as a lost
// connection fails them, and every other read and write as it was.
type writesFail struct {
	dbpkg.DB
	kind string
	n    *atomic.Int32
}

var errWriteLost = errors.New("the write was lost")

func (d writesFail) fails(k dbpkg.Key) bool {
	return k != nil && k.Kind() == d.kind && d.n.Add(-1) >= 0
}

func (d writesFail) Put(ctx context.Context, k dbpkg.Key, src interface{}) (dbpkg.Key, error) {
	if d.fails(k) {
		return nil, errWriteLost
	}
	return d.DB.Put(ctx, k, src)
}

func (d writesFail) Swap(ctx context.Context, k dbpkg.Key, prev string, src any) (bool, error) {
	if d.fails(k) {
		return false, errWriteLost
	}
	return d.DB.(dbpkg.Swapper).Swap(ctx, k, prev, src)
}

// failWrites makes the next n writes of kind fail until the test ends.
func failWrites(t *testing.T, ctx context.Context, kind string, n int32) {
	up := datastore.New(ctx).DB()
	c := new(atomic.Int32)
	c.Store(n)
	datastore.SetDefaultDB(writesFail{DB: up, kind: kind, n: c})
	t.Cleanup(func() { datastore.SetDefaultDB(up) })
}

// orderBody is a 20-seat yearly order of plan with a starting balance.
func orderBody(plan string, balance int64) string {
	return fmt.Sprintf(`{"sourceId":"cnon:ok","planId":%q,"quantity":20,"interval":"year","balanceCents":%d}`, plan, balance)
}

// A sale that stops after the charge — here its paid invoice is not written — is
// finished by the same order sent again once its guard is recoverable: the one
// subscription it opened, its invoice, and the starting balance credited once,
// charging once. Before, the retry was told the subject already pays, and the
// balance was never credited.
func TestRed13_CrashBeforeTheSealLosesTheBalance(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("r13d")
	m := squareMock("cust_r13d", "ccof_r13d", "sqpay_r13d")
	withFakeSquare(t, m)
	orderRow(t, ctx, "r13d-plan", 20, 500)
	body, key := orderBody("r13d-plan", 4000000), map[string]string{"X-Idempotency-Key": "r13d"}
	failWrites(t, ctx, "billing-invoice", 1)
	if r := invokeSubscribeCard(org, ctx, body, key); r.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("the stopped sale answered %d %s, want 503", r.StatusCode, bodyText(r))
	}
	if r := invokeSubscribeCard(org, ctx, body, key); r.StatusCode != http.StatusConflict {
		t.Fatalf("within the guard's life the order answered %d %s, want 409", r.StatusCode, bodyText(r))
	}
	restarted(t)
	r := invokeSubscribeCard(org, ctx, body, key)
	db := datastore.New(org.Namespaced(ctx))
	subs, _ := userSubscriptions(db, "r13d", org.TestMode())
	paid := 0
	for _, s := range subs {
		if s.Metadata["payment"] == "sqpay_r13d" && s.Metadata["bundleParent"] == nil {
			paid++
		}
	}
	if r.StatusCode != http.StatusCreated || balanceOf(t, ctx, org, "r13d") != 4000000 || m.chargeCalls != 1 || paid != 1 {
		t.Fatalf("FINDING: after the stop: %d %s; balance %d, charges %d, subscriptions %d",
			r.StatusCode, bodyText(r), balanceOf(t, ctx, org, "r13d"), m.chargeCalls, paid)
	}
	if sale := receiptOf(t, r); sale.InvoiceID == "" || sale.BalanceID == "" {
		t.Fatalf("the finished receipt names no invoice or credit: %+v", sale)
	}
}

// A sale whose credit failed after its receipt was sealed, with the sale lease
// left behind by a process that died: the order sent again waits for the lease's
// life (409), then completes, crediting once and charging once.
func TestRed13_CrashAfterTheSealCompletesAfterTheLockExpires(t *testing.T) {
	fake := &flakyLedger{fakeLedger: newFakeLedger(), failures: 1}
	injectLedger(t, fake)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("r13e")
	m := squareMock("cust_r13e", "ccof_r13e", "sqpay_r13e")
	withFakeSquare(t, m)
	orderRow(t, ctx, "r13e-plan", 20, 500)
	body, key := orderBody("r13e-plan", 4000000), map[string]string{"X-Idempotency-Key": "r13e"}
	if r := invokeSubscribeCard(org, ctx, body, key); r.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
	}
	db := datastore.New(org.Namespaced(ctx))
	if _, held, err := idempotencykey.Hold(db, saleScope+"r13e", saleKey); err != nil || !held {
		t.Fatalf("leave a lease behind: %v %v", held, err)
	}
	if r := invokeSubscribeCard(org, ctx, body, key); r.StatusCode != http.StatusConflict {
		t.Fatalf("within the lease's life: %d %s, want 409", r.StatusCode, bodyText(r))
	}
	restarted(t)
	r := invokeSubscribeCard(org, ctx, body, key)
	held, _ := fake.Balance(ctx, ledgerOrg(org), "r13e", "usd", org.TestMode())
	if r.StatusCode != http.StatusCreated || held != 4000000 || len(fake.credits()) != 1 || m.chargeCalls != 1 {
		t.Fatalf("FINDING: after the lease's life: %d %s; host %d (credits %d), charges %d",
			r.StatusCode, bodyText(r), held, len(fake.credits()), m.chargeCalls)
	}
}

// The payment's record is written before the sale writes anything else, and a
// sale that cannot write it stops with nothing opened and nothing credited.
func TestTheSaleStopsWhenItsPaymentCannotBeRecorded(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("po-down")
	withFakeSquare(t, squareMock("cust_po_down", "ccof_po_down", "sqpay_po_down"))
	orderRow(t, ctx, "po-down-plan", 20, 500)
	failWrites(t, ctx, "payment-org", 1)
	if r := invokeSubscribeCard(org, ctx, orderBody("po-down-plan", 4000000), map[string]string{"X-Idempotency-Key": "po-down"}); r.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("the sale answered %d %s, want 503", r.StatusCode, bodyText(r))
	}
	subs, _ := userSubscriptions(datastore.New(org.Namespaced(ctx)), "po-down", org.TestMode())
	if len(subs) != 0 || balanceOf(t, ctx, org, "po-down") != 0 {
		t.Fatalf("FINDING: a sale that could not record its payment opened %d subscriptions and credited %d", len(subs), balanceOf(t, ctx, org, "po-down"))
	}
}

// A refund of a payment whose index is missing is traced through the books of the
// org whose own account delivered it, and takes the balance back.
func TestRed13_IndexNotWrittenRefundTakesNothing(t *testing.T) {
	const secret = "whsec_r13f"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "r13f", true)
	withFakeSquare(t, squareMock("cust_r13f", "ccof_r13f", "sqpay_r13f"))
	orderRow(t, ctx, "r13f-plan", 20, 500)
	if r := invokeSubscribeCard(org, ctx, orderBody("r13f-plan", 4000000), map[string]string{"X-Idempotency-Key": "r13f"}); r.StatusCode != http.StatusCreated {
		t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
	}
	p, found, err := paymentorg.Get("sqpay_r13f")
	if err != nil || !found {
		t.Fatalf("index: %v %v", found, err)
	}
	if err := p.Delete(); err != nil {
		t.Fatalf("delete index: %v", err)
	}
	b := refundEvent("evt_r13f", "refund.updated", "rf_r13f", "sqpay_r13f", "COMPLETED", 4480000)
	if r := deliverWebhook(ctx, "r13f", secret, b, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("delivery: %d %s", r.StatusCode, bodyText(r))
	}
	db := datastore.New(org.Namespaced(ctx))
	invs := invoicesForSub(t, db, parentSub(t, db, "r13f", "r13f-plan").Id())
	if balanceOf(t, ctx, org, "r13f") != 0 || invs[0].Refunded() != invs[0].AmountPaid {
		t.Fatalf("FINDING: with no index a full refund left balance %d, invoice refunded %d of %d",
			balanceOf(t, ctx, org, "r13f"), invs[0].Refunded(), invs[0].AmountPaid)
	}
}

// After a failed credit, the order sent again under a NEW key, or with none, is
// the same order: it completes the owed credit, once, and charges nothing.
func TestRed13_NewKeyAfterFailedCreditNeverCredits(t *testing.T) {
	fake := &flakyLedger{fakeLedger: newFakeLedger(), failures: 1}
	injectLedger(t, fake)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("r13b")
	m := squareMock("cust_r13b", "ccof_r13b", "sqpay_r13b")
	withFakeSquare(t, m)
	orderRow(t, ctx, "r13b-plan", 20, 500)
	body := orderBody("r13b-plan", 4000000)
	if r := invokeSubscribeCard(org, ctx, body, map[string]string{"X-Idempotency-Key": "r13b-1"}); r.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("first: %d %s", r.StatusCode, bodyText(r))
	}
	r2 := invokeSubscribeCard(org, ctx, body, map[string]string{"X-Idempotency-Key": "r13b-2"})
	r3 := invokeSubscribeCard(org, ctx, body, nil)
	held, _ := fake.Balance(ctx, ledgerOrg(org), "r13b", "usd", org.TestMode())
	if r2.StatusCode != http.StatusCreated || held != 4000000 || len(fake.credits()) != 1 || m.chargeCalls != 1 {
		t.Fatalf("FINDING: new key %d %s; host %d (credits %d), charges %d", r2.StatusCode, bodyText(r2), held, len(fake.credits()), m.chargeCalls)
	}
	if r3.StatusCode == http.StatusCreated || m.chargeCalls != 1 {
		t.Fatalf("the order sent again once completed was sold again: %d %s, charges %d", r3.StatusCode, bodyText(r3), m.chargeCalls)
	}
	// The host ledger's line says what the money is: the order's starting balance.
	if c := fake.credits(); !strings.HasPrefix(c[0].Reason, "starting balance ") {
		t.Fatalf("the starting balance is posted as %q", c[0].Reason)
	}
}

// A staff record of a subject's plan holds the subject's sale lease, as a sale
// does, so it never runs beside one.
func TestARecordWaitsForTheSubjectsSale(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("rec-lease")
	db := datastore.New(org.Namespaced(ctx))
	lease, held, err := idempotencykey.Hold(db, saleScope+"acme", saleKey)
	if err != nil || !held {
		t.Fatalf("hold: %v %v", held, err)
	}
	if _, err := RecordSubscription(ctx, org, recordGo(t, db)); !IsSaleConflict(err) {
		t.Fatalf("FINDING: a record beside a sale of the same subject answered %v, want held", err)
	}
	lease.Release()
	if _, err := RecordSubscription(ctx, org, recordGo(t, db)); err != nil {
		t.Fatalf("the record after the sale: %v", err)
	}
}

// An owed balance is completed before the catalog is read: switching the plan to
// sales-only after the sale does not strand the money it took.
func TestRed13_OwedBalanceStrandedByACatalogEdit(t *testing.T) {
	fake := &flakyLedger{fakeLedger: newFakeLedger(), failures: 1}
	injectLedger(t, fake)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("r13p")
	withFakeSquare(t, squareMock("cust_r13p", "ccof_r13p", "sqpay_r13p"))
	row := orderRow(t, ctx, "r13p-plan", 20, 500)
	body, key := orderBody("r13p-plan", 4000000), map[string]string{"X-Idempotency-Key": "r13p"}
	if r := invokeSubscribeCard(org, ctx, body, key); r.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("first: %d %s", r.StatusCode, bodyText(r))
	}
	row.ContactSales = true
	if err := row.Update(); err != nil {
		t.Fatal(err)
	}
	r := invokeSubscribeCard(org, ctx, body, key)
	held, _ := fake.Balance(ctx, ledgerOrg(org), "r13p", "usd", org.TestMode())
	if r.StatusCode != http.StatusCreated || held != 4000000 {
		t.Fatalf("FINDING: a catalog edit strands the owed balance: %d %s, host %d", r.StatusCode, bodyText(r), held)
	}
}

// A key names one order, seat mix included: the same key sent for another mix of
// seat types with the same seat count is 409, with nothing charged again.
func TestRed13_KeyReusedForAnotherSeatMix(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("r13j")
	m := squareMock("cust_r13j", "ccof_r13j", "sqpay_r13j")
	withFakeSquare(t, m)
	key := map[string]string{"X-Idempotency-Key": "r13j"}
	if a := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"team","quantity":2,"lines":[{"planId":"team-premium","quantity":1}]}`, key); a.StatusCode != http.StatusCreated {
		t.Fatalf("first: %d %s", a.StatusCode, bodyText(a))
	}
	b := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"team","quantity":1,"lines":[{"planId":"team-premium","quantity":2}]}`, key)
	if b.StatusCode != http.StatusConflict || m.chargeCalls != 1 {
		t.Fatalf("FINDING: a key reused for another seat mix answered %d %s with %d charges", b.StatusCode, bodyText(b), m.chargeCalls)
	}
	if c := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"team","quantity":2,"lines":[{"planId":"team-premium","quantity":1}]}`, key); c.StatusCode != http.StatusOK {
		t.Fatalf("the same order under its key should replay: %d %s", c.StatusCode, bodyText(c))
	}
}

// Refunds of one order that land together are applied one after the other: eight
// refunds of the whole payment between them take the balance and the seats, in
// every order, every time.
func TestRed13_ConcurrentRefundsOfOneOrder(t *testing.T) {
	const secret = "whsec_r13g"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	for trial := range 6 {
		name := fmt.Sprintf("r13g%d", trial)
		ref := "sqpay_" + name
		org := webhookOrg(t, ctx, name, true)
		withFakeSquare(t, squareMock("cust_"+name, "ccof_"+name, ref))
		orderRow(t, ctx, name+"-plan", 20, 500)
		if r := invokeSubscribeCard(org, ctx, orderBody(name+"-plan", 4000000), map[string]string{"X-Idempotency-Key": name}); r.StatusCode != http.StatusCreated {
			t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
		}
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				b := refundEvent(fmt.Sprintf("evt_%s_%d", name, i), "refund.updated", fmt.Sprintf("rf_%s_%d", name, i), ref, "COMPLETED", 560000)
				if r := deliverWebhook(ctx, "", secret, b, ""); r.StatusCode != http.StatusOK {
					t.Errorf("%s delivery %d: %d", name, i, r.StatusCode)
				}
			}()
		}
		wg.Wait()
		db := datastore.New(org.Namespaced(ctx))
		invs := invoicesForSub(t, db, parentSub(t, db, name, name+"-plan").Id())
		if bal := balanceOf(t, ctx, org, name); bal != 0 || invs[0].Refunded() != invs[0].AmountPaid {
			t.Fatalf("FINDING: %s: balance %d, invoice refunded %d of %d", name, bal, invs[0].Refunded(), invs[0].AmountPaid)
		}
	}
}

// Two refunds of half the payment each, delivered at once, take the whole of it.
func TestRed13_TwoConcurrentHalfRefunds(t *testing.T) {
	const secret = "whsec_r13m2"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	for trial := range 10 {
		name := fmt.Sprintf("r13h%d", trial)
		ref := "sqpay_" + name
		org := webhookOrg(t, ctx, name, true)
		withFakeSquare(t, squareMock("cust_"+name, "ccof_"+name, ref))
		orderRow(t, ctx, name+"-plan", 20, 500)
		if r := invokeSubscribeCard(org, ctx, orderBody(name+"-plan", 4000000), map[string]string{"X-Idempotency-Key": name}); r.StatusCode != http.StatusCreated {
			t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
		}
		var wg sync.WaitGroup
		for i := range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				b := refundEvent(fmt.Sprintf("evt_%s_%d", name, i), "refund.updated", fmt.Sprintf("rf_%s_%d", name, i), ref, "COMPLETED", 2240000)
				_ = deliverWebhook(ctx, "", secret, b, "")
			}()
		}
		wg.Wait()
		db := datastore.New(org.Namespaced(ctx))
		invs := invoicesForSub(t, db, parentSub(t, db, name, name+"-plan").Id())
		if bal := balanceOf(t, ctx, org, name); bal != 0 || invs[0].Refunded() != invs[0].AmountPaid {
			t.Fatalf("FINDING: %s: two half refunds left balance %d, invoice refunded %d of %d", name, bal, invs[0].Refunded(), invs[0].AmountPaid)
		}
	}
}

// A balance spent before its payment is refunded is still taken back: the wallet
// goes below zero by what was spent, and the seats end.
func TestRed13_RefundAfterTheBalanceIsSpent(t *testing.T) {
	const secret = "whsec_r13s"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "r13s", true)
	withFakeSquare(t, squareMock("cust_r13s", "ccof_r13s", "sqpay_r13s"))
	orderRow(t, ctx, "r13s-plan", 20, 500)
	if r := invokeSubscribeCard(org, ctx, orderBody("r13s-plan", 40000), map[string]string{"X-Idempotency-Key": "r13s"}); r.StatusCode != http.StatusCreated {
		t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
	}
	if _, err := prepaidFor(ctx, org).Draw(ctx, "r13s", currency.USD, 40000, "spend:r13s"); err != nil {
		t.Fatalf("spend: %v", err)
	}
	deliver(t, ctx, secret, refundEvent("evt_r13s", "refund.updated", "rf_r13s", "sqpay_r13s", "COMPLETED", 480000+40000))
	db := datastore.New(org.Namespaced(ctx))
	invs := invoicesForSub(t, db, parentSub(t, db, "r13s", "r13s-plan").Id())
	if balanceOf(t, ctx, org, "r13s") != -40000 || invs[0].Refunded() != invs[0].AmountPaid {
		t.Fatalf("spent then refunded in full: balance %d (want -40000), invoice refunded %d of %d",
			balanceOf(t, ctx, org, "r13s"), invs[0].Refunded(), invs[0].AmountPaid)
	}
}

// A refund of the seats' price is taken from the balance first: the seats stay paid.
func TestRed13_ARefundOfTheSeatsPriceLeavesTheSeats(t *testing.T) {
	const secret = "whsec_r13i"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "r13i", true)
	withFakeSquare(t, squareMock("cust_r13i", "ccof_r13i", "sqpay_r13i"))
	orderRow(t, ctx, "r13i-plan", 20, 500)
	if r := invokeSubscribeCard(org, ctx, orderBody("r13i-plan", 4000000), map[string]string{"X-Idempotency-Key": "r13i"}); r.StatusCode != http.StatusCreated {
		t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
	}
	deliver(t, ctx, secret, refundEvent("evt_r13i", "refund.updated", "rf_r13i", "sqpay_r13i", "COMPLETED", 480000))
	db := datastore.New(org.Namespaced(ctx))
	invs := invoicesForSub(t, db, parentSub(t, db, "r13i", "r13i-plan").Id())
	if balanceOf(t, ctx, org, "r13i") != 3520000 || invs[0].Refunded() != 0 {
		t.Fatalf("refund of 480000: balance %d (want 3520000), invoice refunded %d (want 0)", balanceOf(t, ctx, org, "r13i"), invs[0].Refunded())
	}
}

// A dispute lost, won, and reported lost again: its loss is taken once and given
// back once. A dispute is one take, by its id, as the host ledger posts it.
func TestRed13_DisputeLostWonLost(t *testing.T) {
	const secret = "whsec_r13k"
	registerSquare(t, secret)
	fake := newFakeLedger()
	injectLedger(t, fake)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "r13k", true)
	withFakeSquare(t, squareMock("cust_r13k", "ccof_r13k", "sqpay_r13k"))
	orderRow(t, ctx, "r13k-plan", 20, 500)
	if r := invokeSubscribeCard(org, ctx, orderBody("r13k-plan", 4000000), map[string]string{"X-Idempotency-Key": "r13k"}); r.StatusCode != http.StatusCreated {
		t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
	}
	held := func() int64 { b, _ := fake.Balance(ctx, ledgerOrg(org), "r13k", "usd", org.TestMode()); return b }
	deliver(t, ctx, secret, disputeEvent("evt_k1", "dispute.created", "dp_k", "sqpay_r13k", "LOST", 4480000))
	a := held()
	deliver(t, ctx, secret, disputeEvent("evt_k2", "dispute.state.updated", "dp_k", "sqpay_r13k", "WON", 4480000))
	b := held()
	deliver(t, ctx, secret, disputeEvent("evt_k3", "dispute.state.updated", "dp_k", "sqpay_r13k", "LOST", 4480000))
	if a != 0 || b != 4000000 || held() != 4000000 || balanceOf(t, ctx, org, "r13k") != 4000000 {
		t.Fatalf("host: lost %d, won %d, lost %d; commerce %d", a, b, held(), balanceOf(t, ctx, org, "r13k"))
	}
}

// An unknown or malformed plan id is a miss (404), never "billing could not be
// reached" (503).
func TestRed13_UnknownPlanIsNotUnavailable(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("r13o")
	withFakeSquare(t, squareMock("cust_r13o", "ccof_r13o", "sqpay_r13o"))
	for _, id := range []string{"nope", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "a/b", "%00", "team-premiumx", "ZZZZZZZZZZZ", "1", "-1", "é"} {
		b, _ := json.Marshal(map[string]any{"sourceId": "cnon:ok", "planId": id})
		if r := invokeSubscribeCard(org, ctx, string(b), nil); r.StatusCode == http.StatusServiceUnavailable {
			t.Errorf("FINDING: plan id %q answers 503: %s", id, bodyText(r))
		}
	}
}

// Whether a payment is already deposited is asked of the store; a store that
// cannot answer is not "no deposit", so nothing is deposited a second time.
func TestFundFailsClosedWhenItsDepositsCannotBeRead(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("fund-closed")
	db := datastore.New(org.Namespaced(ctx))
	if _, _, err := fund(ctx, nil, org, db, "fund-closed", 40000, currency.USD, processor.Square, "sqpay_fund_closed"); err != nil {
		t.Fatal(err)
	}
	up := datastore.New(ctx).DB()
	datastore.SetDefaultDB(depositsUnread{up})
	_, _, err := fund(ctx, nil, org, datastore.New(org.Namespaced(ctx)), "fund-closed", 40000, currency.USD, processor.Square, "sqpay_fund_closed")
	datastore.SetDefaultDB(up)
	if err == nil || depositsOf(t, db, "fund-closed", "sqpay_fund_closed") != 1 {
		t.Fatalf("FINDING: with its deposits unreadable fund answered %v and wrote %d deposits", err, depositsOf(t, db, "fund-closed", "sqpay_fund_closed"))
	}
}

// depositsUnread is the store with its transactions unreadable by query.
type depositsUnread struct{ dbpkg.DB }

func (d depositsUnread) Query(kind string) dbpkg.Query {
	if kind == "transaction" {
		return downQuery{d.DB.Query(kind)}
	}
	return d.DB.Query(kind)
}

func (d depositsUnread) Swap(ctx context.Context, k dbpkg.Key, prev string, src any) (bool, error) {
	return d.DB.(dbpkg.Swapper).Swap(ctx, k, prev, src)
}
