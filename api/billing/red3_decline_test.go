package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	squarelib "github.com/hanzoai/commerce/thirdparty/square"
	"github.com/hanzoai/commerce/util/test/ae"
)

// idemSquare is Square's CreatePayment as its idempotency contract has it: the first
// request under a key is answered and stored; the same key with the same body is
// answered with the stored payment; the same key with a different body is 400
// IDEMPOTENCY_KEY_REUSED.
type idemSquare struct {
	mu      sync.Mutex
	status  string // the status a new payment is answered with
	seen    map[string]string
	keys    []string
	sources []string
	codes   []int
}

func (s *idemSquare) RoundTrip(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	answer := func(code int, body string) (*http.Response, error) {
		s.codes = append(s.codes, code)
		return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	}
	if r.Method != http.MethodPost || r.URL.Path != "/v2/payments" {
		return answer(404, `{"errors":[{"category":"INVALID_REQUEST_ERROR","code":"NOT_FOUND"}]}`)
	}
	raw, _ := io.ReadAll(r.Body)
	var req struct {
		Key      string `json:"idempotency_key"`
		Source   string `json:"source_id"`
		Customer string `json:"customer_id"`
		Amount   struct {
			Amount int64 `json:"amount"`
		} `json:"amount_money"`
	}
	_ = json.Unmarshal(raw, &req)
	s.keys = append(s.keys, req.Key)
	s.sources = append(s.sources, req.Source)
	fp := fmt.Sprintf("%s|%s|%d", req.Source, req.Customer, req.Amount.Amount)
	if s.seen == nil {
		s.seen = map[string]string{}
	}
	if prev, ok := s.seen[req.Key]; ok && prev != fp {
		return answer(400, `{"errors":[{"category":"INVALID_REQUEST_ERROR","code":"IDEMPOTENCY_KEY_REUSED","detail":"The idempotency key was reused with a different request."}]}`)
	}
	s.seen[req.Key] = fp
	return answer(200, fmt.Sprintf(`{"payment":{"id":"sqpay_%s","status":%q,"amount_money":{"amount":%d,"currency":"USD"}}}`,
		req.Key[:6], s.status, req.Amount.Amount))
}

// movingPrepaid is a prepaid balance the test moves between attempts, as usage and
// top-ups move a real one.
type movingPrepaid struct{ avail int64 }

func (p *movingPrepaid) Available(context.Context, string, currency.Type) (int64, error) {
	return p.avail, nil
}
func (p *movingPrepaid) Draw(_ context.Context, _ string, _ currency.Type, amount int64, _ string) (engine.Drawn, error) {
	return engine.Drawn{Balance: amount}, nil
}

// TestRed3_AnUnknownRenewalOutcomeWedgesTheInvoice — the collector keeps its attempt
// count, and so the renewal's Square key, on every unknown outcome, while the amount
// a renewal charges the card is what the moving prepaid balance does not cover. Once
// Square has stored the key (a PENDING payment, or any answer that was lost), an
// attempt that asked under it for a different amount would be answered
// IDEMPOTENCY_KEY_REUSED, another unknown outcome, and the invoice could never be
// collected by card. The next attempt resends the first request and is paid by it.
func TestRed3_AnUnknownRenewalOutcomeWedgesTheInvoice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	sq := &idemSquare{status: "PENDING"}
	old := http.DefaultClient.Transport
	http.DefaultClient.Transport = sq
	t.Cleanup(func() { http.DefaultClient.Transport = old })
	sp := squarelib.NewProcessor(squarelib.Config{AccessToken: "sq-test", LocationID: "L1", Environment: "sandbox"})
	prev := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(sp)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = prev })

	org := moneyOrg("red3-wedge")
	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, "red3-wedge", "dev", "ccof_w", "cust_w")
	inv := seedOpenInvoice(t, db, sub, 1900)
	bal := &movingPrepaid{}

	// Attempt 1: Square takes the payment and has not settled it.
	if _, err := engine.CollectInvoice(ctx, db, inv, bal, chargeProviderForOrg(org)); err != nil {
		t.Fatal(err)
	}
	// It settles; meanwhile the buyer's balance moved by a dollar.
	sq.status = "COMPLETED"
	bal.avail = 100
	for i := 0; i < 3 && inv.Status != billinginvoice.Paid; i++ {
		if _, err := engine.CollectInvoice(ctx, db, inv, bal, chargeProviderForOrg(org)); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("keys=%v codes=%v status=%s attempts=%d", sq.keys, sq.codes, inv.Status, inv.AttemptCount)
	if inv.Status != billinginvoice.Paid || sq.codes[len(sq.codes)-1] == 400 {
		t.Errorf("after %d attempts the invoice is %s and Square answers IDEMPOTENCY_KEY_REUSED to every one: "+
			"the key %q is kept for a charge whose amount moved", len(sq.keys), inv.Status, sq.keys[0])
	}
}

// TestRed3_ASavedCardTopUpReplayIsHeldAtTheCeiling — a buyer retrying a saved-card
// top-up that already completed (a lost response) is owed its receipt, even when the
// wallet has since reached its ceiling: the replay moves no money, so it is answered
// before the wallet is reserved or its ceiling read.
func TestRed3_ASavedCardTopUpReplayIsHeldAtTheCeiling(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red3-replay")
	db := datastore.New(org.Namespaced(ctx))
	pm := seedSavedCard(t, db, "red3-replay", "ccof_r", "cust_r")
	m := squareMock("cust_r", "ccof_r", "sqpay_r")
	withFakeSquare(t, m)

	in := TopupCardIn{MethodID: pm.Id(), AmountCents: 2500, Subject: "red3-replay", IdempotencyKey: "buy-1"}
	first, err := TopupCard(ctx, org, in)
	if err != nil {
		t.Fatalf("the first top-up: %v", err)
	}
	m.chargeErr = cardDeclined
	for i := 0; i < declineCeiling; i++ {
		_, _ = TakePayment(ctx, org, TakePaymentIn{SourceID: fmt.Sprintf("cnon:%d", i), AmountCents: 500 + int64(i), Subject: "red3-replay"})
	}
	again, err := TopupCard(ctx, org, in)
	if err != nil || again.TransactionID != first.TransactionID {
		t.Errorf("a retry of a completed saved-card top-up answered %v (%+v), want its receipt %s", err, again, first.TransactionID)
	}
}

// slowFailing answers every charge after delay with a scrubbed processor failure that
// is not a refusal — what Square answers a garbage or re-used nonce.
type slowFailing struct {
	*MockSquareProcessor
	delay time.Duration
	calls atomic.Int64
}

func (s *slowFailing) Charge(context.Context, processor.PaymentRequest) (*processor.PaymentResult, error) {
	s.calls.Add(1)
	time.Sleep(s.delay)
	err := processor.NewPaymentError(processor.Square, "CARD_TOKEN_USED", "square answered 400 INVALID_REQUEST_ERROR CARD_TOKEN_USED", nil)
	return &processor.PaymentResult{Success: false, Error: err, ErrorMessage: err.Error()}, err
}

// TestRed3_AMemberHoldsTheWalletWithAttemptsThatNeverCount — one member of a tenant
// org replaying a spent nonce back to back holds the org's wallet reservation, and
// every other member's card attempt is answered 409 PAYMENT_IN_PROGRESS, for as long
// as those failures go uncounted. They count, so the ceiling arrives.
func TestRed3_AMemberHoldsTheWalletWithAttemptsThatNeverCount(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red3-hold")
	s := &slowFailing{MockSquareProcessor: squareMock("cust_h", "ccof_h", "sqpay_h"), delay: 40 * time.Millisecond}
	prev := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(s)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = prev })

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ { // four tabs, so the reservation is never free for long
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				_, _ = TakePayment(ctx, org, TakePaymentIn{SourceID: "cnon:spent", AmountCents: 500, Subject: "red3-hold",
					IdempotencyKey: fmt.Sprintf("m-%d-%d", w, i)})
			}
		}(w)
	}
	time.Sleep(50 * time.Millisecond)
	held := 0
	for i := 0; i < 20; i++ {
		_, f := TakePayment(ctx, org, TakePaymentIn{SourceID: "cnon:alice", AmountCents: 700, Subject: "red3-hold", IdempotencyKey: fmt.Sprintf("alice-%d", i)})
		if f != nil && errors.Is(f.Err, errAttemptInFlight) {
			held++
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(stop)
	wg.Wait()
	db := datastore.New(org.Namespaced(ctx))
	n := count(db, walletScope+"red3-hold", windowOf())
	t.Logf("processor calls=%d refusals counted=%d victim attempts held=%d/20", s.calls.Load(), n, held)
	if held >= 15 && n < declineCeiling {
		t.Errorf("another member's card attempts were held %d/20 times by %d uncounted attempts; the ceiling (%d) never engaged", held, s.calls.Load(), declineCeiling)
	}
}
