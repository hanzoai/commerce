package billing

// Red probes for commerce#10 (an organization orders a seat plan with a starting
// balance). Each test states the property the contract promises; a failure is a
// finding, a pass is a held defense.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/util/test/ae"
)

// R1: a refund of the whole Enterprise payment ends the seats AND takes the
// starting balance back. Standalone books (no host ledger): the balance is the
// deposit row fund wrote.
func TestRed_ARefundOfAnOrderTakesTheBalanceBack(t *testing.T) {
	const secret = "whsec_red_ent_refund"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "red-ent-rf", true)
	m := squareMock("cust_rr", "ccof_rr", "sqpay_red_rf")
	withFakeSquare(t, m)
	orderRow(t, ctx, "red-ent-rf-plan", 20, 500)

	body := `{"sourceId":"cnon:ok","planId":"red-ent-rf-plan","quantity":20,"interval":"year","balanceCents":4000000}`
	r := invokeSubscribeCard(org, ctx, body, map[string]string{"X-Idempotency-Key": "red-rf-1"})
	if r.StatusCode != http.StatusCreated {
		t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
	}
	if got := balanceOf(t, ctx, org, "red-ent-rf"); got != 4000000 {
		t.Fatalf("balance after order %d, want 4000000", got)
	}
	// The cardholder is refunded the whole payment: 20 x 24000 + 4000000.
	deliver(t, ctx, secret, refundEvent("evt_red_rf_1", "refund.updated", "rf_red", "sqpay_red_rf", "COMPLETED", 4480000))
	if got := balanceOf(t, ctx, org, "red-ent-rf"); got != 0 {
		t.Fatalf("FINDING: the whole payment was refunded and the starting balance still holds %d cents", got)
	}
}

// R1b: the same with the host ledger injected (production shape): the refund must
// post a clawback against the wallet the balance was credited to.
func TestRed_ARefundOfAnOrderClawsBackInTheHostLedger(t *testing.T) {
	const secret = "whsec_red_ent_refund2"
	registerSquare(t, secret)
	fake := newFakeLedger()
	injectLedger(t, fake)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "red-ent-rf2", true)
	m := squareMock("cust_rr2", "ccof_rr2", "sqpay_red_rf2")
	withFakeSquare(t, m)
	orderRow(t, ctx, "red-ent-rf2-plan", 20, 500)

	body := `{"sourceId":"cnon:ok","planId":"red-ent-rf2-plan","quantity":20,"interval":"year","balanceCents":4000000}`
	if r := invokeSubscribeCard(org, ctx, body, map[string]string{"X-Idempotency-Key": "red-rf-2"}); r.StatusCode != http.StatusCreated {
		t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
	}
	deliver(t, ctx, secret, disputeEvent("evt_red_dp_1", "dispute.created", "dp_red", "sqpay_red_rf2", "LOST", 4480000))
	if len(fake.clawbacks) == 0 {
		t.Fatalf("FINDING: a lost dispute of the whole Enterprise payment posted no clawback (clawbacks=%d)", len(fake.clawbacks))
	}
}

// R2: the 500 path. The credit failed after the seats were charged and opened: the
// subscription and its paid invoice exist, the receipt is sealed owing the balance
// (no balanceId, the state the sale seals before it credits), and no deposit was
// written. A retry under the same key must complete the credit exactly once.
func TestRed_ARetryAfterAFailedBalanceCreditCompletesIt(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-ent-500")
	m := squareMock("cust_r5", "ccof_r5", "sqpay_red_500")
	withFakeSquare(t, m)
	orderRow(t, ctx, "red-ent-500-plan", 20, 500)

	body := `{"sourceId":"cnon:ok","planId":"red-ent-500-plan","quantity":20,"interval":"year","balanceCents":40000}`
	key := map[string]string{"X-Idempotency-Key": "red-500"}
	if r := invokeSubscribeCard(org, ctx, body, key); r.StatusCode != http.StatusCreated {
		t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
	}
	// Put the books in the state a failed credit leaves: the receipt owes the
	// balance, and no deposit holds it.
	db := datastore.New(org.Namespaced(ctx))
	g := idempotencykey.New(db)
	if err := g.Get(db.NewKey(g.Kind(), idempotencykey.DeterministicID("billing-subscribe:red-ent-500:", "red-500"), 0, nil)); err != nil {
		t.Fatalf("guard: %v", err)
	}
	var owed Sale
	if err := json.Unmarshal([]byte(g.Response), &owed); err != nil {
		t.Fatalf("receipt: %v", err)
	}
	owed.BalanceID = ""
	sealed, _ := json.Marshal(owed)
	g.Status, g.Response = idempotencykey.StatusCompleted, string(sealed)
	if err := g.Update(); err != nil {
		t.Fatalf("reset guard: %v", err)
	}
	deps := make([]*transaction.Transaction, 0)
	if _, err := transaction.Query(db).Filter("DestinationId=", "red-ent-500").GetAll(&deps); err != nil {
		t.Fatalf("deposits: %v", err)
	}
	for _, d := range deps {
		row := transaction.New(db)
		if err := row.GetById(d.Id()); err != nil {
			t.Fatalf("read deposit: %v", err)
		}
		if err := row.Delete(); err != nil {
			t.Fatalf("delete deposit: %v", err)
		}
	}
	if got := balanceOf(t, ctx, org, "red-ent-500"); got != 0 {
		t.Fatalf("setup: balance %d", got)
	}
	again := invokeSubscribeCard(org, ctx, body, key)
	t.Logf("retry under the same key: %d %s", again.StatusCode, bodyText(again))
	if got := balanceOf(t, ctx, org, "red-ent-500"); got != 40000 || m.chargeCalls != 1 {
		t.Fatalf("FINDING: the retry did not complete the credit: balance %d (want 40000), charges %d", got, m.chargeCalls)
	}
	// And only once: the next retry replays the completed receipt.
	if r := invokeSubscribeCard(org, ctx, body, key); r.StatusCode != http.StatusOK {
		t.Fatalf("replay after completion: %d %s", r.StatusCode, bodyText(r))
	}
	if got := balanceOf(t, ctx, org, "red-ent-500"); got != 40000 || m.chargeCalls != 1 {
		t.Fatalf("a second retry moved the balance to %d with %d charges", got, m.chargeCalls)
	}
}

// R1c: refunds and disputes of an order, in the production shape (a host ledger):
// a partial refund takes back from the balance first and leaves the seats paid; a
// dispute won gives back what its loss took; a refund of the rest takes the seats.
func TestRed_PartialRefundsAndDisputesOfAnOrder(t *testing.T) {
	const secret = "whsec_red_ent_partial"
	registerSquare(t, secret)
	fake := newFakeLedger()
	injectLedger(t, fake)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "red-ent-pt", true)
	withFakeSquare(t, squareMock("cust_pt", "ccof_pt", "sqpay_red_pt"))
	orderRow(t, ctx, "red-ent-pt-plan", 20, 500)
	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"red-ent-pt-plan","quantity":20,"interval":"year","balanceCents":4000000}`,
		map[string]string{"X-Idempotency-Key": "red-pt"}); r.StatusCode != http.StatusCreated {
		t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
	}
	held := func() int64 {
		b, _ := fake.Balance(ctx, ledgerOrg(org), "red-ent-pt", "usd", org.TestMode())
		return b
	}
	if got := held(); got != 4000000 {
		t.Fatalf("the host ledger holds %d after the order, want the 4000000 balance", got)
	}
	paid := func() int64 {
		sub := parentSub(t, datastore.New(org.Namespaced(ctx)), "red-ent-pt", "red-ent-pt-plan")
		invs := invoicesForSub(t, datastore.New(org.Namespaced(ctx)), sub.Id())
		return invs[0].AmountPaid - invs[0].Refunded()
	}

	deliver(t, ctx, secret, refundEvent("evt_pt_1", "refund.updated", "rf_pt_1", "sqpay_red_pt", "COMPLETED", 1000000))
	if got := held(); got != 3000000 || paid() != 480000 {
		t.Fatalf("a $10,000 refund left balance %d, seats paid %d; want 3000000 and 480000", got, paid())
	}
	deliver(t, ctx, secret, disputeEvent("evt_pt_2", "dispute.created", "dp_pt", "sqpay_red_pt", "LOST", 4480000))
	if len(fake.clawbacks) != 2 || fake.clawbacks[1].AmountCents != 3000000 || held() != 0 {
		t.Fatalf("a lost dispute posted %+v and left %d, want a clawback of the 3000000 the refund left, and nothing", fake.clawbacks, held())
	}
	deliver(t, ctx, secret, disputeEvent("evt_pt_3", "dispute.state.updated", "dp_pt", "sqpay_red_pt", "WON", 4480000))
	if len(fake.restores) != 1 || held() != 3000000 {
		t.Fatalf("a won dispute restored %+v and left %d, want what its loss took back: 3000000", fake.restores, held())
	}
}

// R1d: in commerce's own books, a partial refund takes the balance down by what it
// returned, and later refunds never take back more than the payment credited.
func TestRed_RefundsOfAnOrderTakeBackAtMostTheBalance(t *testing.T) {
	const secret = "whsec_red_ent_cap"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "red-ent-cap", true)
	withFakeSquare(t, squareMock("cust_cp", "ccof_cp", "sqpay_red_cp"))
	orderRow(t, ctx, "red-ent-cap-plan", 20, 500)
	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"red-ent-cap-plan","quantity":20,"interval":"year","balanceCents":40000}`,
		map[string]string{"X-Idempotency-Key": "red-cap"}); r.StatusCode != http.StatusCreated {
		t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
	}
	deliver(t, ctx, secret, refundEvent("evt_cp_1", "refund.updated", "rf_cp_1", "sqpay_red_cp", "COMPLETED", 10000))
	if got := balanceOf(t, ctx, org, "red-ent-cap"); got != 30000 {
		t.Fatalf("a $100 refund left %d, want 30000", got)
	}
	deliver(t, ctx, secret, refundEvent("evt_cp_1b", "refund.updated", "rf_cp_1", "sqpay_red_cp", "COMPLETED", 10000))
	deliver(t, ctx, secret, refundEvent("evt_cp_2", "refund.updated", "rf_cp_2", "sqpay_red_cp", "COMPLETED", 480000))
	if got := balanceOf(t, ctx, org, "red-ent-cap"); got != 0 {
		t.Fatalf("refunds of 10000 (twice delivered) and 480000 left %d, want 0: never below what the payment credited", got)
	}
}

// R3: the same key with another body (more seats, a balance) must not answer 2xx
// with a receipt for a different order.
func TestRed_AKeyReusedForAnotherOrderIsRefused(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-ent-reuse")
	m := squareMock("cust_ru", "ccof_ru", "sqpay_red_ru")
	withFakeSquare(t, m)
	orderRow(t, ctx, "red-ent-reuse-plan", 20, 500)
	key := map[string]string{"X-Idempotency-Key": "red-reuse"}
	if r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"red-ent-reuse-plan","quantity":20,"interval":"year"}`, key); r.StatusCode != http.StatusCreated {
		t.Fatalf("order: %d %s", r.StatusCode, bodyText(r))
	}
	r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"red-ent-reuse-plan","quantity":40,"interval":"year","balanceCents":5000000}`, key)
	raw, _ := io.ReadAll(r.Body)
	var s Sale
	_ = json.Unmarshal(raw, &s)
	if r.StatusCode < 300 && (s.Seats != 40 || s.BalanceCents != 5000000) {
		t.Fatalf("FINDING: key reuse for a 40-seat $50,000 order answered %d with the 20-seat receipt (seats %d, balance %d)", r.StatusCode, s.Seats, s.BalanceCents)
	}
}

// R4: two submits of one order with two keys (two tabs, a key minted per click)
// race past the one-paid-subscription check: at most one charge.
func TestRed_TwoKeysForOneOrderChargeOnce(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-ent-race")
	m := squareMock("cust_rc", "ccof_rc", "sqpay_red_rc")
	withFakeSquare(t, m)
	orderRow(t, ctx, "red-ent-race-plan", 20, 500)
	body := `{"sourceId":"cnon:ok","planId":"red-ent-race-plan","quantity":20,"interval":"year","balanceCents":5000000}`
	var wg sync.WaitGroup
	codes := make([]int, 4)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = invokeSubscribeCard(org, ctx, body, map[string]string{"X-Idempotency-Key": fmt.Sprintf("red-race-%d", i)}).StatusCode
		}(i)
	}
	wg.Wait()
	subs := subsOf(t, datastore.New(org.Namespaced(ctx)), "red-ent-race")
	t.Logf("codes %v, charges %d, subscriptions %d, balance %d", codes, m.chargeCalls, len(subs), balanceOf(t, ctx, org, "red-ent-race"))
	if m.chargeCalls > 1 {
		t.Fatalf("FINDING: one order submitted under %d keys charged %d times", len(codes), m.chargeCalls)
	}
}

// R5: malformed and boundary inputs never reach the card.
func TestRed_OrderInputsAtTheBoundary(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-ent-bounds")
	m := squareMock("cust_rb", "ccof_rb", "sqpay_red_rb")
	withFakeSquare(t, m)
	orderRow(t, ctx, "red-ent-b", 20, 500)
	for _, body := range []string{
		`{"sourceId":"cnon:ok","planId":"red-ent-b","quantity":"20","interval":"year"}`,
		`{"sourceId":"cnon:ok","planId":"red-ent-b","quantity":20.0,"interval":"year"}`,
		`{"sourceId":"cnon:ok","planId":"red-ent-b","quantity":2e1,"interval":"year"}`,
		`{"sourceId":"cnon:ok","planId":"red-ent-b","quantity":9223372036854775807,"interval":"year"}`,
		`{"sourceId":"cnon:ok","planId":"red-ent-b","quantity":0,"interval":"year"}`,
		`{"sourceId":"cnon:ok","planId":"red-ent-b","quantity":-20,"interval":"year"}`,
		`{"sourceId":"cnon:ok","planId":"red-ent-b","quantity":20,"interval":"year","balanceCents":40000.0}`,
		`{"sourceId":"cnon:ok","planId":"red-ent-b","quantity":20,"interval":"year","balanceCents":"40000"}`,
		`{"sourceId":"cnon:ok","planId":"red-ent-b","quantity":20,"interval":"year","balanceCents":9223372036854775807}`,
		`{"sourceId":"cnon:ok","planId":"red-ent-b","quantity":20,"interval":"year","balanceCents":9223372036854775800}`,
		`{"sourceId":"cnon:ok","planId":"red-ent-b","quantity":20,"interval":"year","balanceCents":-9223372036854775800}`,
		`{"sourceId":"cnon:ok","planId":"red-ent-b","quantity":20,"interval":"year","balanceCents":18446744073709551600}`,
		`{"sourceId":"credits","planId":"red-ent-b","quantity":20,"interval":"year","balanceCents":10000}`,
		`{"sourceId":"cnon:ok","planId":"red-ent-b","quantity":20,"interval":"year","balanceCents":10000,"userId":"red-ent-bounds/bob"}`,
		`{"sourceId":"cnon:ok","planId":"red-ent-b","quantity":20,"interval":"year","balanceCents":10000,"coupon":"50OFFBETA"}`,
		`{"sourceId":"cnon:ok","planId":"red-ent-b","quantity":20,"interval":"year","balanceCents":10000,"lines":[{"planId":"team-premium","quantity":1}]}`,
	} {
		r := invokeSubscribeCard(org, ctx, body, nil)
		t.Logf("%d %s <- %s", r.StatusCode, bodyText(r), body)
		if r.StatusCode < 400 {
			t.Errorf("FINDING: accepted %s", body)
		}
	}
	if m.chargeCalls != 0 {
		t.Fatalf("FINDING: %d charges for refused orders", m.chargeCalls)
	}
	// The largest order the row sells is one charge, no overflow.
	r := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"red-ent-b","quantity":500,"interval":"year","balanceCents":5000000}`, nil)
	if r.StatusCode != http.StatusCreated || m.lastChargeAmount != 500*24000+5000000 {
		t.Fatalf("500 seats + $50,000: %d charged %d %s", r.StatusCode, m.lastChargeAmount, bodyText(r))
	}
}

// R6: a member's wallet never buys the organization's order through the HTTP door,
// whatever userId it names and whatever it says about being an admin.
func TestRed_AMemberCannotOrderForTheOrg(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-ent-member")
	m := squareMock("cust_rm", "ccof_rm", "sqpay_red_rm")
	withFakeSquare(t, m)
	orderRow(t, ctx, "red-ent-m", 20, 500)
	for _, userID := range []string{"", "red-ent-member", "RED-ENT-MEMBER", "red-ent-member/bob"} {
		body := fmt.Sprintf(`{"sourceId":"cnon:ok","planId":"red-ent-m","quantity":20,"interval":"year","balanceCents":10000,"userId":%q}`, userID)
		r := invokeSubscribeCard(org, ctx, body, map[string]string{"X-User-IsOrgAdmin": "false", "X-User-Id": "bob"})
		t.Logf("member userId=%q: %d %s", userID, r.StatusCode, bodyText(r))
		if r.StatusCode < 400 {
			t.Errorf("FINDING: a member ordered for userId=%q", userID)
		}
	}
	if m.chargeCalls != 0 {
		t.Fatalf("FINDING: %d member charges", m.chargeCalls)
	}
}
