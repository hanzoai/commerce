package billing

// red2_decline_test.go — second adversarial pass over decline-reason (74131d896).

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/payment/processor"
	squarelib "github.com/hanzoai/commerce/thirdparty/square"
	"github.com/hanzoai/commerce/util/test/ae"
)

// slowSquare is the mock Square whose Charge takes as long as a real one does
// (Square's p50 for CreatePayment is hundreds of ms), so concurrent requests overlap.
type slowSquare struct {
	*MockSquareProcessor
	delay time.Duration
}

func (s *slowSquare) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	time.Sleep(s.delay)
	return s.MockSquareProcessor.Charge(ctx, req)
}

// TestRed2_ParallelCardsSlipPastTheCeiling — the ceiling is read before the charge
// and written after the refusal, with no reservation between, and the write is a
// read-modify-write. A card tester sending N top-ups at once, each under its own
// X-Idempotency-Key, has all N tried at Square in the first second of the hour.
func TestRed2_ParallelCardsSlipPastTheCeiling(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red2-parallel")
	m := squareMock("cust_p", "ccof_p", "sqpay_p")
	m.chargeErr = cardDeclined
	s := &slowSquare{MockSquareProcessor: m, delay: 150 * time.Millisecond}
	prev := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(s)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = prev })

	const n = 20
	var wg sync.WaitGroup
	statuses := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, f := TakePayment(ctx, org, TakePaymentIn{
				SourceID: fmt.Sprintf("cnon:stolen-%d", i), AmountCents: 500,
				Subject: "red2-parallel", IdempotencyKey: fmt.Sprintf("k-%d", i),
			})
			if f != nil {
				statuses[i] = f.Status
			}
		}(i)
	}
	wg.Wait()

	db := datastore.New(org.Namespaced(ctx))
	recorded := count(db, walletScope+"red2-parallel", windowOf())
	t.Logf("statuses=%v cards tried at Square=%d refusals recorded=%d", statuses, m.chargeCalls, recorded)
	if m.chargeCalls > declineCeiling {
		t.Errorf("%d cards were tried at Square in one burst; the ceiling is %d an hour", m.chargeCalls, declineCeiling)
	}
	if recorded != m.chargeCalls {
		t.Errorf("%d refusals happened and %d were recorded: the wallet's count loses concurrent increments", m.chargeCalls, recorded)
	}
}

// TestRed2_AutoRechargeDeclinesSpendTheBuyersCeiling — the auto-recharge cron charges
// the org's default card off-session every 15 minutes, through chargeAndCredit, keyed
// on org.Name: the same wallet key a tenant member's own card top-up and card save are
// held on. A default card that has started failing spends four of the five refusals
// every hour before the buyer has done anything, so the buyer's first mistyped CVV
// locks the org out of every card path, including saving the card that would fix
// auto-recharge. Renewals were exempted from the ceiling for being off-session;
// auto-recharge is just as off-session and is not.
func TestRed2_AutoRechargeDeclinesSpendTheBuyersCeiling(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red2-autorecharge")
	db := datastore.New(org.Namespaced(ctx))
	pm := seedSavedCard(t, db, org.Name, "ccof_old", "cust_ar")

	m := squareMock("cust_ar", "ccof_new", "sqpay_ar")
	m.chargeErr = &processor.Decline{Processor: processor.Square, Category: "PAYMENT_METHOD_ERROR", Code: "CARD_EXPIRED"}
	withFakeSquare(t, m)

	// Four cron runs in one hour (:00 :15 :30 :45), each in its own 15-minute window,
	// exactly as RunAutoRecharge derives its guard.
	for w := 0; w < 4; w++ {
		guard := fmt.Sprintf("recharge:%s:amount:2500:cur:usd:w%d", org.Name, 1000+w)
		if _, _, err := chargeAndCredit(ctx, nil, org, db, pm, 2500, "usd", org.Name, guard, "Auto-recharge"); err == nil {
			t.Fatalf("cron run %d: %v", w, err)
		}
	}

	// The org admin notices, and pays with a new card: one CVV typo.
	m.chargeErr = &processor.Decline{Processor: processor.Square, Category: "PAYMENT_METHOD_ERROR", Code: "CVV_FAILURE"}
	if _, f := TakePayment(ctx, org, TakePaymentIn{SourceID: "cnon:typo", AmountCents: 2500, Subject: org.Name}); f == nil || f.Status != 402 {
		t.Fatalf("precondition: the typo answered %+v", f)
	}
	// The corrected card.
	m.chargeErr = nil
	if _, f := TakePayment(ctx, org, TakePaymentIn{SourceID: "cnon:right", AmountCents: 2500, Subject: org.Name}); f != nil {
		t.Errorf("the org's corrected card answered %d %q after one typo: the cron's off-session refusals spent the buyer's ceiling", f.Status, f.Message)
	}
}

// TestRed2_AMemberCanStopTheOrgsAutoRecharge — the converse: five declines by any
// member of a tenant org (their own card, wrong CVV) hold the org's wallet, and the
// cron's off-session charge of the org's good default card is then refused before
// Square is asked. The org's prepaid balance is not refilled for the hour, and the
// member can repeat it every hour.
func TestRed2_AMemberCanStopTheOrgsAutoRecharge(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red2-member")
	db := datastore.New(org.Namespaced(ctx))
	pm := seedSavedCard(t, db, org.Name, "ccof_good", "cust_m")

	m := squareMock("cust_m", "ccof_good", "sqpay_m")
	m.chargeErr = &processor.Decline{Processor: processor.Square, Category: "PAYMENT_METHOD_ERROR", Code: "CVV_FAILURE"}
	withFakeSquare(t, m)
	for i := 0; i < declineCeiling; i++ {
		_, _ = TakePayment(ctx, org, TakePaymentIn{SourceID: fmt.Sprintf("cnon:bad-%d", i), AmountCents: 500 + int64(i), Subject: org.Name})
	}
	m.chargeErr = nil
	_, _, err := chargeAndCredit(ctx, nil, org, db, pm, 2500, "usd", org.Name, "recharge:red2-member:amount:2500:cur:usd:w1", "Auto-recharge")
	if IsDeclineCeiling(err) {
		t.Errorf("the org's auto-recharge of its good default card was refused before Square by a member's refusals: %v", err)
	}
}

// keyedSquare answers CreatePayment from a script and records each request's
// idempotency key.
type keyedSquare struct {
	mu      sync.Mutex
	answers []string // bodies for successive POST /v2/payments, each 200
	keys    []string
}

func (k *keyedSquare) RoundTrip(r *http.Request) (*http.Response, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	body := `{"errors":[{"category":"INVALID_REQUEST_ERROR","code":"NOT_FOUND"}]}`
	status := 404
	if r.Method == http.MethodPost && r.URL.Path == "/v2/payments" {
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			IdempotencyKey string `json:"idempotency_key"`
		}
		_ = json.Unmarshal(raw, &req)
		k.keys = append(k.keys, req.IdempotencyKey)
		status = 200
		body = k.answers[len(k.keys)-1]
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}

// TestRed2_APendingPaymentIsRetriedUnderAFreshKey — Square answers 200 with a
// payment it has not settled (status PENDING). Charge turns that into a Decline, the
// top-up counts it as a refusal, and the buyer is told "Your card was declined by the
// bank." — which the bank never said. The buyer pays again, and the refusal count has
// moved the retry onto a fresh gateway key, so Square creates a SECOND payment
// instead of replaying the first. When the first settles, the buyer has paid twice
// and been credited once; the webhook finds no record of the first payment id and
// can only hand it to manual reconciliation. On main the retry reuses the key.
func TestRed2_APendingPaymentIsRetriedUnderAFreshKey(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	w := &keyedSquare{answers: []string{
		`{"payment":{"id":"sqpay_pending","status":"PENDING","amount_money":{"amount":2500,"currency":"USD"}}}`,
		`{"payment":{"id":"sqpay_second","status":"COMPLETED","amount_money":{"amount":2500,"currency":"USD"}}}`,
	}}
	old := http.DefaultClient.Transport
	http.DefaultClient.Transport = w
	t.Cleanup(func() { http.DefaultClient.Transport = old })
	sp := squarelib.NewProcessor(squarelib.Config{AccessToken: "sq-test", LocationID: "L1", Environment: "sandbox"})
	prev := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(sp)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = prev })

	org := moneyOrg("red2-pending")
	_, f := TakePayment(ctx, org, TakePaymentIn{SourceID: "cnon:first", AmountCents: 2500, Subject: "red2-pending"})
	if f == nil {
		t.Fatal("precondition: a PENDING payment was credited")
	}
	t.Logf("first attempt: %d %q", f.Status, f.Message)
	_, f2 := TakePayment(ctx, org, TakePaymentIn{SourceID: "cnon:second", AmountCents: 2500, Subject: "red2-pending"})
	t.Logf("second attempt: %+v keys=%v", f2, w.keys)
	if len(w.keys) == 2 && w.keys[0] != w.keys[1] {
		t.Errorf("a payment Square had taken and not yet settled was retried under a fresh key (%q then %q): two payments for one top-up", w.keys[0], w.keys[1])
	}
	if f.Status == 402 && strings.Contains(f.Message, "declined by the bank") {
		t.Errorf("a PENDING payment was told to the buyer as %q", f.Message)
	}
}
