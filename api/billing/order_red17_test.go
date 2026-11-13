package billing

// Red's probes against commerce#17 (TestRed17_*), adopted: a dispute won around
// the credit is given back once, through the ledger that took it; a refund after
// it takes what the wallet holds; a sale's refund never takes the wallet path; the
// lease yields to the next holder.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A dispute won while the credit is undecided, then the credit, then a second WON
// report: giveBack credits what the credit already included.
func TestRed17_WonBeforeTheCreditThenWonAgainCreditsTwice(t *testing.T) {
	const secret = "whsec_r17a"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org, fake, key, body := owe(t, ctx, "r17a", "sqpay_r17a", 4000000, true)
	deliver(t, ctx, secret, disputeEvent("evt_r17a_1", "dispute.created", "dp_r17a", "sqpay_r17a", "EVIDENCE_REQUIRED", 4480000))
	deliver(t, ctx, secret, disputeEvent("evt_r17a_2", "dispute.state.updated", "dp_r17a", "sqpay_r17a", "WON", 4480000))
	r := invokeSubscribeCard(org, ctx, body, key)
	held := func() int64 { b, _ := fake.Balance(ctx, ledgerOrg(org), "r17a", "usd", org.TestMode()); return b }
	t.Logf("retry: %d %s; host %d commerce %d", r.StatusCode, bodyText(r), held(), balanceOf(t, ctx, org, "r17a"))
	deliver(t, ctx, secret, disputeEvent("evt_r17a_3", "dispute.updated", "dp_r17a", "sqpay_r17a", "WON", 4480000))
	t.Logf("after a second WON report: host %d commerce %d", held(), balanceOf(t, ctx, org, "r17a"))
	if held() != 4000000 || balanceOf(t, ctx, org, "r17a") != 4000000 {
		t.Errorf("FINDING: a won dispute is given back twice: host %d, commerce %d (want 4000000)", held(), balanceOf(t, ctx, org, "r17a"))
	}
}

// The same with the WON report redelivered under its own event id and no credit
// between: control.
func TestRed17_WonAfterTheCreditOnce(t *testing.T) {
	const secret = "whsec_r17b"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org, fake, key, body := owe(t, ctx, "r17b", "sqpay_r17b", 4000000, true)
	deliver(t, ctx, secret, disputeEvent("evt_r17b_1", "dispute.created", "dp_r17b", "sqpay_r17b", "EVIDENCE_REQUIRED", 4480000))
	r := invokeSubscribeCard(org, ctx, body, key)
	held := func() int64 { b, _ := fake.Balance(ctx, ledgerOrg(org), "r17b", "usd", org.TestMode()); return b }
	t.Logf("retry while withheld: %d %s; host %d", r.StatusCode, bodyText(r), held())
	deliver(t, ctx, secret, disputeEvent("evt_r17b_2", "dispute.state.updated", "dp_r17b", "sqpay_r17b", "WON", 4480000))
	deliver(t, ctx, secret, disputeEvent("evt_r17b_3", "dispute.updated", "dp_r17b", "sqpay_r17b", "WON", 4480000))
	t.Logf("won twice: host %d commerce %d", held(), balanceOf(t, ctx, org, "r17b"))
	if held() != 4000000 || balanceOf(t, ctx, org, "r17b") != 4000000 {
		t.Errorf("FINDING: host %d, commerce %d (want 4000000)", held(), balanceOf(t, ctx, org, "r17b"))
	}
}

// Lost, won, lost again on a decided-and-landed credit; and a refund after a won
// dispute that was taken before the credit.
func TestRed17_RefundAfterAWonDisputeGivenBack(t *testing.T) {
	const secret = "whsec_r17c"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org, fake, key, body := owe(t, ctx, "r17c", "sqpay_r17c", 4000000, true)
	deliver(t, ctx, secret, disputeEvent("evt_r17c_1", "dispute.created", "dp_r17c", "sqpay_r17c", "EVIDENCE_REQUIRED", 4480000))
	_ = invokeSubscribeCard(org, ctx, body, key)
	deliver(t, ctx, secret, disputeEvent("evt_r17c_2", "dispute.state.updated", "dp_r17c", "sqpay_r17c", "WON", 4480000))
	held := func() int64 { b, _ := fake.Balance(ctx, ledgerOrg(org), "r17c", "usd", org.TestMode()); return b }
	t.Logf("won: host %d commerce %d", held(), balanceOf(t, ctx, org, "r17c"))
	deliver(t, ctx, secret, refundEvent("evt_r17c_3", "refund.updated", "rf_r17c", "sqpay_r17c", "COMPLETED", 4480000))
	po, _, _ := paymentorg.Get("sqpay_r17c")
	t.Logf("then refunded in full: host %d commerce %d; record %+v", held(), balanceOf(t, ctx, org, "r17c"), po.Takes)
	if held() != 0 || balanceOf(t, ctx, org, "r17c") != 0 {
		t.Errorf("FINDING: a payment refunded in full after a won dispute leaves host %d, commerce %d", held(), balanceOf(t, ctx, org, "r17c"))
	}
}

// The 503 path under another key or no key.
func TestRed17_OwedUnderAnotherKey(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, fake, _, body := owe(t, ctx, "r17d", "sqpay_r17d", 4000000, false)
	a := invokeSubscribeCard(org, ctx, body, map[string]string{"X-Idempotency-Key": "r17d-other"})
	b := invokeSubscribeCard(org, ctx, body, nil)
	held, _ := fake.Balance(ctx, ledgerOrg(org), "r17d", "usd", org.TestMode())
	t.Logf("other key: %d %s; no key: %d %s; host %d", a.StatusCode, bodyText(a), b.StatusCode, bodyText(b), held)
	if held != 4000000 {
		t.Errorf("FINDING (liveness): only the first key completes the owed balance: host %d", held)
	}
}

// A planted lease nobody renews (a holder that crashed) blocks refunds for
// StartedTTL, then yields; a released-over lease is not deleted by its old holder.
func TestRed17_LeaseCrashAndTakeover(t *testing.T) {
	const secret = "whsec_r17e"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "r17e", true)
	withFakeSquare(t, squareMock("cust_r17e", "ccof_r17e", "sqpay_r17e"))
	orderRow(t, ctx, "r17e-plan", 20, 500)
	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"r17e-plan","quantity":20,"interval":"year","balanceCents":4000000}`,
		map[string]string{"X-Idempotency-Key": "r17e"}); r.StatusCode != http.StatusCreated {
		t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
	}
	db := datastore.New(org.Namespaced(ctx))
	if _, replay, err := idempotencykey.Begin(db, paymentScope, "sqpay_r17e"); err != nil || replay {
		t.Fatalf("plant: %v %v", replay, err)
	}
	start := time.Now()
	_, held, err := holdPayment(db, "sqpay_r17e")
	if held || err != nil || time.Since(start) < paymentWait {
		t.Fatalf("a refund under a crashed holder's lease: held=%v err=%v after %v", held, err, time.Since(start))
	}
	old := idemNow
	idemNow = func() time.Time { return time.Now().Add(10 * time.Minute) }
	l2, held2, err := holdPayment(db, "sqpay_r17e")
	idemNow = old
	if !held2 || err != nil {
		t.Fatalf("after StartedTTL: held=%v err=%v", held2, err)
	}
	l2.Release()

	a, ok, err := idempotencykey.Hold(db, "red-lease:", "x")
	if err != nil || !ok {
		t.Fatalf("hold a: %v %v", ok, err)
	}
	idemNow = func() time.Time { return time.Now().Add(10 * time.Minute) }
	b, ok, err := idempotencykey.Hold(db, "red-lease:", "x")
	idemNow = old
	t.Logf("takeover after expiry: %v %v", ok, err)
	a.Release()
	_, ok2, _ := idempotencykey.Hold(db, "red-lease:", "x")
	t.Logf("after the old holder released: a third holder gets it: %v (want false)", ok2)
	if ok2 {
		t.Errorf("FINDING: the old holder's Release removed the new holder's lease")
	}
	b.Release()
}

// Concurrent half refunds (F3) and 8-way refunds, now under the lease.
func TestRed17_ConcurrentRefunds(t *testing.T) {
	const secret = "whsec_r17f"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	bad := 0
	for trial := 0; trial < 8; trial++ {
		name := fmt.Sprintf("r17f%d", trial)
		ref := "sqpay_" + name
		org := webhookOrg(t, ctx, name, true)
		withFakeSquare(t, squareMock("cust_"+name, "ccof_"+name, ref))
		orderRow(t, ctx, name+"-plan", 20, 500)
		if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"`+name+`-plan","quantity":20,"interval":"year","balanceCents":4000000}`,
			map[string]string{"X-Idempotency-Key": name}); r.StatusCode != http.StatusCreated {
			t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
		}
		n := 2 + trial%2*6
		each := int64(4480000) / int64(n)
		var wg sync.WaitGroup
		codes := make([]int, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				b := refundEvent(fmt.Sprintf("evt_%s_%d", name, i), "refund.updated", fmt.Sprintf("rf_%s_%d", name, i), ref, "COMPLETED", each)
				codes[i] = deliverWebhook(ctx, "", secret, b, "").StatusCode
			}(i)
		}
		wg.Wait()
		// Redeliver whatever was refused, as the processor would.
		for i := 0; i < n; i++ {
			if codes[i] != http.StatusOK {
				b := refundEvent(fmt.Sprintf("evt_%s_%d", name, i), "refund.updated", fmt.Sprintf("rf_%s_%d", name, i), ref, "COMPLETED", each)
				codes[i] = deliverWebhook(ctx, "", secret, b, "").StatusCode
			}
		}
		db := datastore.New(org.Namespaced(ctx))
		invs := invoicesForSub(t, db, parentSub(t, db, name, name+"-plan").Id())
		bal := balanceOf(t, ctx, org, name)
		if bal != 0 || invs[0].Refunded() != invs[0].AmountPaid {
			bad++
			t.Logf("trial %d (%d refunds): codes %v balance %d, invoice refunded %d of %d", trial, n, codes, bal, invs[0].Refunded(), invs[0].AmountPaid)
		}
	}
	if bad > 0 {
		t.Errorf("FINDING: %d of 8 orders refunded in full concurrently left the wallet or the seats wrong", bad)
	}
}

// A refund after the balance is spent (standalone).
func TestRed17_RefundAfterSpend(t *testing.T) {
	const secret = "whsec_r17g"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "r17g", true)
	withFakeSquare(t, squareMock("cust_r17g", "ccof_r17g", "sqpay_r17g"))
	orderRow(t, ctx, "r17g-plan", 20, 500)
	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"r17g-plan","quantity":20,"interval":"year","balanceCents":40000}`,
		map[string]string{"X-Idempotency-Key": "r17g"}); r.StatusCode != http.StatusCreated {
		t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
	}
	if _, err := prepaidFor(ctx, org).Draw(ctx, "r17g", "usd", 30000, "spend:r17g"); err != nil {
		t.Fatalf("spend: %v", err)
	}
	deliver(t, ctx, secret, refundEvent("evt_r17g_1", "refund.updated", "rf_r17g_1", "sqpay_r17g", "COMPLETED", 20000))
	deliver(t, ctx, secret, refundEvent("evt_r17g_2", "refund.updated", "rf_r17g_2", "sqpay_r17g", "COMPLETED", 500000))
	db := datastore.New(org.Namespaced(ctx))
	invs := invoicesForSub(t, db, parentSub(t, db, "r17g", "r17g-plan").Id())
	if balanceOf(t, ctx, org, "r17g") != -30000 || invs[0].Refunded() != invs[0].AmountPaid {
		t.Fatalf("spent 30000 of 40000, refunded 20000 then 500000: balance %d (want -30000), invoice refunded %d of %d",
			balanceOf(t, ctx, org, "r17g"), invs[0].Refunded(), invs[0].AmountPaid)
	}
}

// A non-Enterprise sale: the record #17 now writes when the first invoice was not
// recorded (Invoice "", Balance 0) sends its refund down the wallet path, which
// the host ledger refuses: the delivery 503s for as long as Square redelivers.
func TestRed17_TeamSaleWithoutAnInvoiceRefundLoops(t *testing.T) {
	const secret = "whsec_r17h"
	registerSquare(t, secret)
	fake := newFakeLedger()
	injectLedger(t, fake)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "r17h", true)
	withFakeSquare(t, squareMock("cust_r17h", "ccof_r17h", "sqpay_r17h"))
	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"team","quantity":3}`, map[string]string{"X-Idempotency-Key": "r17h"}); r.StatusCode != http.StatusCreated {
		t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
	}
	// The state subscribe_card.go:1093 writes when CreatePaidFirstInvoice failed.
	if err := paymentorg.Put("sqpay_r17h", org.Name, "r17h", "", 0, org.TestMode()); err != nil {
		t.Fatal(err)
	}
	r := deliverWebhook(ctx, "", secret, refundEvent("evt_r17h", "refund.updated", "rf_r17h", "sqpay_r17h", "COMPLETED", 7500), "")
	t.Logf("refund of a Team sale whose invoice was not recorded: %d %s", r.StatusCode, bodyText(r))
	if r.StatusCode != http.StatusOK {
		t.Errorf("FINDING: the refund answers %d and is redelivered until Square gives up", r.StatusCode)
	}
}

// Same key with another seat mix (F7).
func TestRed17_KeyReusedForAnotherSeatMix(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("r17i")
	m := squareMock("cust_r17i", "ccof_r17i", "sqpay_r17i")
	withFakeSquare(t, m)
	key := map[string]string{"X-Idempotency-Key": "r17i"}
	_ = invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"team","quantity":2,"lines":[{"planId":"team-premium","quantity":1}]}`, key)
	b := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"team","quantity":1,"lines":[{"planId":"team-premium","quantity":2}]}`, key)
	t.Logf("same key, other mix: %d", b.StatusCode)
	if b.StatusCode < 300 {
		t.Errorf("FINDING: a key reused for another seat mix answered %d with the first receipt", b.StatusCode)
	}
}

// F4: the process dies after the subscription and the payment's record are
// written, before the receipt is sealed.
func TestRed17b_CrashBeforeTheSeal(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("r17j")
	m := squareMock("cust_r17j", "ccof_r17j", "sqpay_r17j")
	withFakeSquare(t, m)
	orderRow(t, ctx, "r17j-plan", 20, 500)
	body := `{"sourceId":"cnon:ok","planId":"r17j-plan","quantity":20,"interval":"year","balanceCents":4000000}`
	key := map[string]string{"X-Idempotency-Key": "r17j"}
	db := datastore.New(org.Namespaced(ctx))
	// Hold the payment so the sale cannot decide the credit (the state a crash
	// before conclude leaves: record written, credit undecided, nothing landed).
	l, ok, err := idempotencykey.Hold(db, paymentScope, "sqpay_r17j")
	if err != nil || !ok {
		t.Fatal(err)
	}
	_ = invokeSubscribeCard(org, ctx, body, key)
	l.Release()
	g := idempotencykey.New(db)
	if err := g.Get(db.NewKey(g.Kind(), idempotencykey.DeterministicID("billing-subscribe:r17j:", "r17j"), 0, nil)); err != nil {
		t.Fatalf("guard: %v", err)
	}
	g.Status, g.Response = idempotencykey.StatusStarted, ""
	_ = g.Update()
	old := idemNow
	idemNow = func() time.Time { return time.Now().Add(10 * time.Minute) }
	defer func() { idemNow = old }()
	r := invokeSubscribeCard(org, ctx, body, key)
	po, _, _ := paymentorg.Get("sqpay_r17j")
	t.Logf("retry after TTL: %d %s; balance %d; record credited=%v", r.StatusCode, bodyText(r), balanceOf(t, ctx, org, "r17j"), po.Credited)
	if balanceOf(t, ctx, org, "r17j") != 4000000 {
		t.Errorf("FINDING: a crash before the seal still strands the paid balance")
	}
}

// F6: an owed balance after the row is switched to contact-sales.
func TestRed17b_OwedStrandedByContactSales(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, fake, key, body := owe(t, ctx, "r17k", "sqpay_r17k", 4000000, false)
	p := plan.New(plan.AuthorityDB(ctx))
	if ok, err := p.Query().Filter("Slug=", "r17k-plan").Get(); err != nil || !ok {
		t.Fatalf("row: %v %v", ok, err)
	}
	p.ContactSales = true
	if err := p.Update(); err != nil {
		t.Fatal(err)
	}
	r := invokeSubscribeCard(org, ctx, body, key)
	held, _ := fake.Balance(ctx, ledgerOrg(org), "r17k", "usd", org.TestMode())
	t.Logf("same key after contactSales: %d %s; host %d", r.StatusCode, bodyText(r), held)
	if held != 4000000 {
		t.Errorf("FINDING: the kill switch strands the owed balance")
	}
}

// A refund delivered while the sale itself is landing the credit: the sale
// answers 409 and the receipt (as first sealed) owes; after it, sale.BalanceCents
// in the receipt is the netted credit and same() still matches the order.
func TestRed17b_ReceiptAfterNettedCreditStillReplays(t *testing.T) {
	const secret = "whsec_r17l"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org, fake, key, body := owe(t, ctx, "r17l", "sqpay_r17l", 4000000, true)
	deliver(t, ctx, secret, refundEvent("evt_r17l", "refund.updated", "rf_r17l", "sqpay_r17l", "COMPLETED", 1000000))
	a := invokeSubscribeCard(org, ctx, body, key)
	b := invokeSubscribeCard(org, ctx, body, key)
	var s Sale
	_ = json.Unmarshal([]byte(bodyText(b)), &s)
	held, _ := fake.Balance(ctx, ledgerOrg(org), "r17l", "usd", org.TestMode())
	t.Logf("complete: %d; replay: %d balanceCents %d; host %d", a.StatusCode, b.StatusCode, s.BalanceCents, held)
	if b.StatusCode != http.StatusOK || held != 3000000 {
		t.Errorf("FINDING: replay %d host %d", b.StatusCode, held)
	}
}

// Funds racing for one payment — two holders of a lapsed lease, a retry crossing
// its original — write one deposit between them, and every one answers it.
func TestFundsRacingForOnePaymentDepositOnce(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	for n := range 6 {
		org := moneyOrg(fmt.Sprintf("race%d", n))
		db := datastore.New(org.Namespaced(ctx))
		ref := fmt.Sprintf("sqpay_race_%d", n)
		ids := make([]string, 8)
		var wg sync.WaitGroup
		for i := range ids {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 20 {
					id, err := fund(ctx, nil, org, db, org.Name, 40000, "usd", "square", ref)
					if err == nil {
						ids[i] = id
						return
					}
					time.Sleep(5 * time.Millisecond)
				}
			}()
		}
		wg.Wait()
		if got := depositsOf(t, db, org.Name, ref); got != 1 {
			t.Fatalf("FINDING: 8 funds of one payment wrote %d deposits", got)
		}
		for i, id := range ids {
			if id != ids[0] || id == "" {
				t.Fatalf("fund %d answered %q, the first %q", i, id, ids[0])
			}
		}
	}
}

// A refund of a plan sale that stopped before writing its invoice never takes the
// wallet path: the delivery stays open until the sale is finished, and then lands
// on the invoice.
func TestARefundWaitsForTheSalesInvoice(t *testing.T) {
	const secret = "whsec_wait_inv"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "wait-inv", true)
	m := squareMock("cust_wait_inv", "ccof_wait_inv", "sqpay_wait_inv")
	withFakeSquare(t, m)
	body, key := `{"sourceId":"cnon:ok","planId":"team","quantity":3}`, map[string]string{"X-Idempotency-Key": "wait-inv"}
	failWrites(t, ctx, "billing-invoice", 1)
	if r := invokeSubscribeCard(org, ctx, body, key); r.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("the stopped sale answered %d %s", r.StatusCode, bodyText(r))
	}
	refund := refundEvent("evt_wait_inv", "refund.updated", "rf_wait_inv", "sqpay_wait_inv", "COMPLETED", 7500)
	if r := deliverWebhook(ctx, "", secret, refund, ""); r.StatusCode == http.StatusOK {
		t.Fatalf("FINDING: a refund of a sale with no invoice was answered %d and is not delivered again", r.StatusCode)
	}
	restarted(t)
	if r := invokeSubscribeCard(org, ctx, body, key); r.StatusCode != http.StatusCreated {
		t.Fatalf("the sale finished as %d %s", r.StatusCode, bodyText(r))
	}
	if r := deliverWebhook(ctx, "", secret, refund, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("the redelivered refund answered %d %s", r.StatusCode, bodyText(r))
	}
	db := datastore.New(org.Namespaced(ctx))
	invs := invoicesForSub(t, db, parentSub(t, db, "wait-inv", "team").Id())
	if len(invs) != 1 || invs[0].Refunded() != 7500 || m.chargeCalls != 1 {
		t.Fatalf("the refund should land on the finished sale's invoice: %+v, charges %d", invs, m.chargeCalls)
	}
}
