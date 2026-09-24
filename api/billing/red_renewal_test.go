package billing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	squarecore "github.com/square/square-go-sdk/v3/core"
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	gift "github.com/hanzoai/commerce/billing/grant"
	"github.com/hanzoai/commerce/billing/tier"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/creditgrant"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// Adversarial cases of the renewal cycle. redSquare models what Square does
// with an idempotency key, which the package mock does not: a repeat of a key
// with the same request returns the first answer, a repeat with a different
// amount, card or customer is refused with 400 IDEMPOTENCY_KEY_REUSED, and a
// request Square processed can still reach the caller as a 5xx (the money
// moved, the answer was lost).

type redPayment struct {
	token, customer string
	amount          int64
	landed          bool
	res             *processor.PaymentResult
	err             error
}

type redSquare struct {
	*MockSquareProcessor
	mu     sync.Mutex
	script []string // outcome per NEW key: "ok", "decline", "landed5xx", "unauthorized"; default "ok"
	keys   map[string]*redPayment
	calls  int
	before func() // runs once, before the next request is processed
	after  func() // runs once, after Square has processed the next request
}

func newRedSquare(script ...string) *redSquare {
	return &redSquare{MockSquareProcessor: newMockSquare(nil, "", nil), script: script, keys: map[string]*redPayment{}}
}

func (s *redSquare) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	s.mu.Lock()
	hook := s.before
	s.before = nil
	s.mu.Unlock()
	if hook != nil {
		hook()
	}

	res, err := s.process(req)
	s.mu.Lock()
	after := s.after
	s.after = nil
	s.mu.Unlock()
	if after != nil {
		after()
	}
	return res, err
}

func (s *redSquare) process(req processor.PaymentRequest) (*processor.PaymentResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if p, ok := s.keys[req.IdempotencyKey]; ok {
		if p.amount != int64(req.Amount) || p.token != req.Token || p.customer != req.CustomerID {
			err := squarecore.NewAPIError(400, nil, errors.New(`{"errors":[{"category":"INVALID_REQUEST_ERROR","code":"IDEMPOTENCY_KEY_REUSED"}]}`))
			return &processor.PaymentResult{Success: false, Error: err, ErrorMessage: err.Error()}, err
		}
		if p.landed {
			return p.res, nil
		}
		return p.res, p.err
	}
	outcome := "ok"
	if len(s.script) > 0 {
		outcome, s.script = s.script[0], s.script[1:]
	}
	p := &redPayment{token: req.Token, customer: req.CustomerID, amount: int64(req.Amount)}
	ok := &processor.PaymentResult{Success: true, TransactionID: "sqpay_" + req.IdempotencyKey, ProcessorRef: "sqpay_" + req.IdempotencyKey, Status: "COMPLETED"}
	s.keys[req.IdempotencyKey] = p
	switch outcome {
	case "decline":
		p.err = squarecore.NewAPIError(402, nil, errors.New(`{"errors":[{"category":"PAYMENT_METHOD_ERROR","code":"GENERIC_DECLINE"}]}`))
		p.res = &processor.PaymentResult{Success: false, Error: p.err, ErrorMessage: p.err.Error()}
		return p.res, p.err
	case "unauthorized":
		p.err = squarecore.NewAPIError(401, nil, errors.New(`{"errors":[{"category":"AUTHENTICATION_ERROR","code":"UNAUTHORIZED"}]}`))
		p.res = &processor.PaymentResult{Success: false, Error: p.err, ErrorMessage: p.err.Error()}
		delete(s.keys, req.IdempotencyKey) // Square never processed it
		return p.res, p.err
	case "landed5xx":
		p.landed, p.res = true, ok
		err := squarecore.NewAPIError(502, nil, errors.New("bad gateway"))
		return &processor.PaymentResult{Success: false, Error: err, ErrorMessage: err.Error()}, err
	default:
		p.landed, p.res = true, ok
		return ok, nil
	}
}

// captured is what Square moved off the customer's cards, in cents.
func (s *redSquare) captured() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	for _, p := range s.keys {
		if p.landed {
			n += p.amount
		}
	}
	return n
}

func withRedSquare(t *testing.T, s *redSquare) {
	t.Helper()
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(s)
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
}

// redCredit gives subject a promotional credit grant, which no payment of a
// subscription's invoice spends.
func redCredit(t *testing.T, db *datastore.Datastore, subject string, cents int64) {
	t.Helper()
	g := creditgrant.New(db)
	g.UserId = subject
	g.Name = "promo"
	g.AmountCents = cents
	g.RemainingCents = cents
	g.Currency = currency.USD
	g.EffectiveAt = time.Now().Add(-time.Hour)
	if err := g.Create(); err != nil {
		t.Fatalf("seed credit grant: %v", err)
	}
}

// redSpent is how much of a credit grant of cents the subject has burned: a
// plan's invoice is never paid from credits, so a grant is only counted as paid
// when it was actually spent.
func redSpent(t *testing.T, db *datastore.Datastore, subject string, cents int64) int64 {
	t.Helper()
	uncovered, err := BurnCreditsPreview(db, subject, cents)
	if err != nil {
		t.Fatalf("read credit grants: %v", err)
	}
	return uncovered
}

func redPrice(t *testing.T) int64 {
	t.Helper()
	p := lookupPlan("dev")
	if p == nil {
		t.Fatal("plan dev missing")
	}
	return int64(p.Price)
}

// TestRed_CustomerPayUnknownThenRetryChargesTwice: after a declined renewal the
// customer pays the open invoice themselves (POST /invoices/:id/pay); Square
// takes the money and the answer is lost (502). That attempt is not counted,
// and the cycle's retry a day later repeats it under the same key, so one
// period is charged once.
func TestRed_CustomerPayUnknownThenRetryChargesTwice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-pay-unknown")
	sq := newRedSquare("decline", "landed5xx", "ok")
	withRedSquare(t, sq)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "red-pay-unknown", d)

	only(t, cycleAt(t, ctx, org, d, false), engine.RenewalFailed)
	inv := invoicesForSub(t, db, sub.Id())[0]
	_ = invokePay(org, ctx, inv.Id()) // Square charges the card and answers 502
	cycleAt(t, ctx, org, d.Add(24*time.Hour), false)

	if got, price := sq.captured(), redPrice(t); got > price {
		t.Fatalf("Square captured %d cents for one %d-cent renewal: the customer's payment whose answer was lost was counted "+
			"as a decline, and the cycle's retry charged again under a new key", got, price)
	}
}

// TestRed_UnknownThenCustomerPayWithCreditsChargesTwice: the cycle's charge
// lands and its answer is lost. The customer, seeing past_due, pays the
// invoice: the payment repeats the recorded attempt, the same instrument,
// amount and key, and spends no credit, so the period is paid once.
func TestRed_UnknownThenCustomerPayWithCreditsChargesTwice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-unknown-credits")
	sq := newRedSquare("landed5xx", "ok")
	withRedSquare(t, sq)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "red-unknown-credits", d)
	price := redPrice(t)
	credit := price / 4
	redCredit(t, db, "red-unknown-credits", credit)

	only(t, cycleAt(t, ctx, org, d, false), engine.Skipped) // unknown: Square took the full price
	inv := invoicesForSub(t, db, sub.Id())[0]
	_ = invokePay(org, ctx, inv.Id()) // credits burned; same key, smaller amount: 400 key reused
	cycleAt(t, ctx, org, d.Add(time.Hour), false)

	spent := redSpent(t, db, "red-unknown-credits", credit)
	if paid := sq.captured() + spent; paid > price {
		t.Fatalf("the customer paid %d cents (card %d + credits %d) for one %d-cent renewal: a 400 IDEMPOTENCY_KEY_REUSED "+
			"is not a card decline", paid, sq.captured(), spent, price)
	}
}

// TestRed_PayDuringRetryIsOverwrittenAndChargedAgain: the customer pays the
// past_due invoice while the cycle's retry of it is at Square. Both hold the
// invoice's one lock, so the customer's payment is refused as in progress, the
// paid invoice is never written back open, and the period is paid once.
func TestRed_PayDuringRetryIsOverwrittenAndChargedAgain(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-pay-race")
	sq := newRedSquare("decline", "ok", "ok", "ok")
	withRedSquare(t, sq)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "red-pay-race", d)
	price := redPrice(t)
	credit := price / 4
	redCredit(t, db, "red-pay-race", credit)

	only(t, cycleAt(t, ctx, org, d, false), engine.RenewalFailed)
	inv := invoicesForSub(t, db, sub.Id())[0]
	sq.before = func() { _ = invokePay(org, ctx, inv.Id()) }
	cycleAt(t, ctx, org, d.Add(24*time.Hour), false)

	if got := invoicesForSub(t, db, sub.Id())[0]; got.Status != billinginvoice.Paid {
		t.Errorf("the invoice the customer paid reads %s after the cycle's write", got.Status)
	}
	cycleAt(t, ctx, org, d.Add(72*time.Hour), false)
	spent := redSpent(t, db, "red-pay-race", credit)
	if paid := sq.captured() + spent; paid > price {
		t.Fatalf("the customer paid %d cents (card %d + credits %d) for one %d-cent renewal", paid, sq.captured(), spent, price)
	}
}

// TestRed_CancelDuringRenewalIsLost: the customer cancels at period end while
// the renewal's charge is at Square. The renewal writes only the fields it owns
// onto a fresh read of the row, so the cancel stands and the next period is not
// charged.
func TestRed_CancelDuringRenewalIsLost(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-cancel-race")
	sq := newRedSquare()
	withRedSquare(t, sq)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "red-cancel-race", d)
	sq.before = func() {
		if _, err := cancelSubscription(ctx, org, sub.Id(), true); err != nil {
			t.Errorf("cancel: %v", err)
		}
	}

	only(t, cycleAt(t, ctx, org, d, false), engine.Renewed)
	if got := reloadSub(t, db, sub.Id()); !got.EndCancel {
		cycleAt(t, ctx, org, d.AddDate(0, 1, 0), false)
		t.Fatalf("the customer's cancel at period end was overwritten by the renewal's write (EndCancel=%v); "+
			"the next run charged the month after too: %d cents captured, %d owed", got.EndCancel, sq.captured(), redPrice(t))
	}
}

// TestRed_VoidedRenewalInvoiceKeepsTheTierForever: an org owner (the Admin
// group admits org-level isAdmin) voids the declined renewal invoice through
// POST /v1/billing/invoices/:id/void. The next run ends the subscription on
// its voided invoice, and the subscriber drops to free.
func TestRed_VoidedRenewalInvoiceKeepsTheTierForever(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-void")
	sq := newRedSquare("decline", "decline", "decline", "decline")
	withRedSquare(t, sq)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "red-void", d)

	only(t, cycleAt(t, ctx, org, d, false), engine.RenewalFailed)
	inv := invoicesForSub(t, db, sub.Id())[0]
	if _, f := VoidInvoiceIn(ctx, org, inv.Id(), nil); f != nil {
		t.Fatalf("void: %+v", f)
	}
	for _, at := range []time.Duration{8 * 24 * time.Hour, 40 * 24 * time.Hour, 400 * 24 * time.Hour} {
		cycleAt(t, ctx, org, d.Add(at), false)
	}
	if n := tierOf(t, ctx, org, "red-void"); n != tier.Free {
		t.Fatalf("400 days after a declined renewal whose invoice was voided, the subscriber is %s (status %s) and has paid nothing",
			n, reloadSub(t, db, sub.Id()).Status)
	}
}

// TestRed_OtherModeRowKeepsTheTierForever: a live subscription (Test=false) in
// an org that is in test mode (the org's mode flipped, or the row predates the
// mode stamp) is not charged through the org's processor; it ends at its period
// end, and the paid tier is not served for free.
func TestRed_OtherModeRowKeepsTheTierForever(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-mode")
	org.Live = false
	sq := newRedSquare()
	withRedSquare(t, sq)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "red-mode", d)

	for _, at := range []time.Duration{0, 30 * 24 * time.Hour, 400 * 24 * time.Hour} {
		cycleAt(t, ctx, org, d.Add(at), false)
	}
	got := reloadSub(t, db, sub.Id())
	if got.Status == subscription.Active && tierOf(t, ctx, org, "red-mode") != tier.Free {
		t.Fatalf("400 days past its paid period the row is %s, charged %d cents, and serves tier %s",
			got.Status, sq.captured(), tierOf(t, ctx, org, "red-mode"))
	}
}

// TestRed_UnrecognizedDryRunChargesCards: a dry run spelled ?dryRun=yes (or
// ?dryrun=true, ?dry_run=1) is refused, never read as a real run.
func TestRed_UnrecognizedDryRunChargesCards(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-dryrun")
	sq := newRedSquare()
	withRedSquare(t, sq)
	db := datastore.New(org.Namespaced(ctx))
	cardSubThrough(t, db, "red-dryrun", time.Now().Add(-time.Minute))

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/cycle/run?dryRun=yes", nil)
	resp := driveSeeded(func(c *zip.Ctx) {
		c.Locals("organization", org)
		c.SetContext(ctx)
	}, "/v1/billing/cycle/run", req, RunBillingCycle)
	if resp.StatusCode == http.StatusOK && sq.calls > 0 {
		t.Fatalf("a request that asked for a dry run charged a card (%d cents); an unparseable dryRun must be refused", sq.captured())
	}
}

// TestRed_RealignEndsSubscribersMidPeriod: a subscriber sits on [P1, P2] with
// its first invoice paying [P0, P1]. The cycle never realigns it on its own. An
// operator's realignment moves it back onto [P0, P1], which ended ten days
// before the next run: that run finds it overdue, charges nothing and does not
// end it. It confers nothing (lapsed) until the customer renews it or
// subscribes again.
func TestRed_RealignEndsSubscribersMidPeriod(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-realign")
	m := squareMock("cust_r", "ccof_r", "sqpay_r")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))

	sub := subscribeByCard(t, ctx, org, `{"sourceId":"cnon:ok","planId":"dev"}`)
	row := reloadSub(t, db, sub.Id())
	p1 := row.PeriodEnd
	row.PeriodStart, row.PeriodEnd = p1, p1.AddDate(0, 1, 0) // a period ahead of the paid invoice
	if err := row.Update(); err != nil {
		t.Fatalf("shift: %v", err)
	}
	if r := Realign(ctx, []*organization.Organization{org}, false); r.Realigned != 1 {
		t.Fatalf("realigned %d rows, want the one", r.Realigned)
	}

	res := only(t, cycleAt(t, ctx, org, p1.Add(10*24*time.Hour), false), engine.Overdue)
	got := reloadSub(t, db, sub.Id())
	if got.Status == subscription.Canceled || m.chargeCalls != 1 || res.AmountCents != 0 {
		t.Fatalf("a realigned subscriber ten days past its paid period is %s (%v) after %d charges; want it left as it is and nothing charged",
			got.Status, got.Metadata["endReason"], m.chargeCalls)
	}
	if !engine.Lapsed(got, p1.Add(10*24*time.Hour)) {
		t.Fatalf("row %s through %s is not lapsed ten days past its paid period; it would serve a plan nobody paid for", got.Status, got.PeriodEnd)
	}
}

// TestRed_PartialPaymentKeptWhenTheRenewalIsVoided: the customer pays the
// declined renewal invoice themselves and the card declines again; no credit is
// spent on it. They cancel at period end. The cycle voids the invoice and ends
// the subscription, and nothing the customer paid toward the period that is
// never served is kept.
func TestRed_PartialPaymentKeptWhenTheRenewalIsVoided(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-partial-void")
	sq := newRedSquare("decline", "decline")
	withRedSquare(t, sq)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "red-partial-void", d)
	credit := redPrice(t) / 4
	redCredit(t, db, "red-partial-void", credit)

	only(t, cycleAt(t, ctx, org, d, false), engine.RenewalFailed)
	inv := invoicesForSub(t, db, sub.Id())[0]
	_ = invokePay(org, ctx, inv.Id()) // credits burned, card declines
	if _, err := cancelSubscription(ctx, org, sub.Id(), true); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	only(t, cycleAt(t, ctx, org, d.Add(time.Hour), false), engine.CanceledAtPeriodEnd)

	got := invoicesForSub(t, db, sub.Id())[0]
	left, _ := BurnCreditsPreview(db, "red-partial-void", credit)
	if got.Status == billinginvoice.Void && got.AmountPaid > 0 && left == credit {
		t.Fatalf("invoice voided holding %d cents the customer paid toward a period never served; nothing was returned", got.AmountPaid)
	}
}

// TestRed_TaxRecalcMarksTheRenewalPaidUncharged: POST
// /invoices/:id/calculate-tax on an issued renewal invoice is refused, so what
// it owes is never recomputed under a payment, and the renewal is not marked
// paid on less than its price.
func TestRed_TaxRecalcMarksTheRenewalPaidUncharged(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-tax")
	sq := newRedSquare("decline", "decline")
	withRedSquare(t, sq)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "red-tax", d)
	price := redPrice(t)
	credit := price / 2
	redCredit(t, db, "red-tax", credit)

	only(t, cycleAt(t, ctx, org, d, false), engine.RenewalFailed)
	inv := invoicesForSub(t, db, sub.Id())[0]
	_ = invokePay(org, ctx, inv.Id()) // half in credits, the card declines
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/invoices/"+inv.Id()+"/calculate-tax?country=ZZ", nil)
	_ = driveSeeded(func(c *zip.Ctx) {
		c.Locals("organization", org)
		c.SetContext(ctx)
	}, "/v1/billing/invoices/:id/calculate-tax", req, CalculateInvoiceTax)
	cycleAt(t, ctx, org, d.Add(24*time.Hour), false)

	got := reloadSub(t, db, sub.Id())
	if paid := sq.captured() + credit; got.Status == subscription.Active && paid < price {
		t.Fatalf("the renewal is paid and the subscription active on %d cents (card %d + credits %d) of a %d-cent period",
			paid, sq.captured(), credit, price)
	}
}

// TestRed_UnknownOutcomeNeverEnds: Square answers 401 on every attempt (our
// access token rotated or revoked; a missing processor or a KMS hydration
// failure reads the same way). The outcome is unknown, so the attempt is
// repeated under its key, and once the retry schedule is spent the subscription
// is escalated to unpaid, which confers no tier.
func TestRed_UnknownOutcomeNeverEnds(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-unknown-forever")
	script := make([]string, 16)
	for i := range script {
		script[i] = "unauthorized"
	}
	sq := newRedSquare(script...)
	withRedSquare(t, sq)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "red-unknown-forever", d)

	for _, at := range []time.Duration{0, 24 * time.Hour, 72 * time.Hour, 168 * time.Hour, 30 * 24 * time.Hour, 365 * 24 * time.Hour} {
		cycleAt(t, ctx, org, d.Add(at), false)
	}
	got := reloadSub(t, db, sub.Id())
	if got.Status == subscription.PastDue || got.Status == subscription.Active {
		t.Fatalf("a year after its paid period the subscription is %s with tier %s after %d unanswered attempts",
			got.Status, tierOf(t, ctx, org, "red-unknown-forever"), sq.calls)
	}
}

// TestRed_LandedChargeVoidedAfterGrace: the renewal charge lands at Square while
// the scheduler's client that asked for the run disconnects. The run is not tied
// to the request, and the attempt is recorded before the charge is sent, so the
// next run, three days later, never voids the invoice or ends the subscription
// the customer paid for.
func TestRed_LandedChargeVoidedAfterGrace(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-landed-voided")
	sq := newRedSquare("ok")
	withRedSquare(t, sq)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Add(-time.Hour).Truncate(time.Second)
	sub := cardSubThrough(t, db, "red-landed-voided", d)

	rctx, cancel := context.WithCancel(ctx)
	sq.after = cancel // the scheduler's client disconnects while Square answers 200
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/cycle/run?dryRun=false", nil)
	_ = driveSeeded(func(c *zip.Ctx) {
		c.Locals("organization", org)
		c.SetContext(rctx)
	}, "/v1/billing/cycle/run", req, RunBillingCycle)
	if sq.captured() == 0 {
		t.Fatal("the renewal charge did not reach Square")
	}

	r := cycleAt(t, ctx, org, d.Add(80*time.Hour), false) // the next run, after a three-day outage
	for _, res := range r.Results {
		if res.Action == engine.ExpiredOverdue {
			t.Fatalf("the renewal's charge landed (%d cents) and the next run voided its invoice and ended the subscription (%s); "+
				"an attempt must be recorded before the charge is sent", sq.captured(), reloadSub(t, db, sub.Id()).Status)
		}
	}
}

// TestRed_CardSubscriberInEcosystemOrgRenewsFree: a hanzo.ai customer who
// signs up through hanzo.id lands in the shared "hanzo" org (IAM stamps the
// app's org) and buys a plan with their card. The plan renews on that card like
// anywhere else; it is never rolled forward for free.
func TestRed_CardSubscriberInEcosystemOrgRenewsFree(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("hanzo")
	m := squareMock("cust_eco", "ccof_eco", "sqpay_eco")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))

	sub := subscribeByCard(t, ctx, org, `{"sourceId":"cnon:ok","planId":"dev","userId":"hanzo/alice"}`)
	first := m.chargeCalls
	end := reloadSub(t, db, sub.Id()).PeriodEnd
	for i := 0; i < 3; i++ {
		cycleAt(t, ctx, org, end.AddDate(0, i, 0), false)
	}
	got := reloadSub(t, db, sub.Id())
	if m.chargeCalls == first && got.Status == subscription.Active {
		t.Fatalf("a card-bought subscription (charged once at checkout) was rolled forward to %s for free: %d renewal charges in three months",
			got.PeriodEnd.Format(time.RFC3339), m.chargeCalls-first)
	}
}

type redCatalog map[string]*gift.CatalogPlan

func (c redCatalog) Lookup(slug string) *gift.CatalogPlan { return c[slug] }

// TestRed_TimedGiftNeverEnds: grant.Grant gives a plan for Request.Duration. A
// 30-day gift ends when its 30 days do; it is never charged and never rolled
// forward.
func TestRed_TimedGiftNeverEnds(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-gift")
	withRedSquare(t, newRedSquare())
	db := datastore.New(org.Namespaced(ctx))
	p := lookupPlan("dev")
	cat := redCatalog{"dev": {Slug: "dev", Name: "Dev", PriceCents: int64(p.Price), Currency: "usd"}}
	res, err := gift.Grant(ctx, db, cat, gift.Request{UserId: "red-gift", PlanSlug: "dev", Duration: 30 * 24 * time.Hour, Reason: "30-day beta gift", GrantedBy: "red"})
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	for _, at := range []time.Duration{31 * 24 * time.Hour, 120 * 24 * time.Hour, 400 * 24 * time.Hour} {
		cycleAt(t, ctx, org, time.Now().Add(at), false)
	}
	if got := reloadSub(t, db, res.SubscriptionID); got.Status == subscription.Active {
		t.Fatalf("a 30-day gift is %s through %s, 400 days after it was granted", got.Status, got.PeriodEnd.Format(time.RFC3339))
	}
}

// TestRed_BalanceRetryAfterUnknownCardPaysTwice: a balance-bought subscription
// that also has a card renews from prepaid money only. A short balance is a
// declined renewal, never a charge on the card; a top-up before the next retry
// pays the period once, from the balance.
func TestRed_BalanceRetryAfterUnknownCardPaysTwice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("red-bal-unknown")
	sq := newRedSquare("landed5xx")
	withRedSquare(t, sq)
	db := datastore.New(org.Namespaced(ctx))
	d := time.Now().Truncate(time.Second)
	sub := cardSubThrough(t, db, "red-bal-unknown", d)
	if _, err := engine.CreatePaidFirstInvoice(db, sub, "balance", "bal_first"); err != nil {
		t.Fatalf("first invoice: %v", err)
	}
	price := redPrice(t)

	only(t, cycleAt(t, ctx, org, d, false), engine.RenewalFailed) // prepaid money short; the card is not asked
	seedBalance(t, ctx, org, "red-bal-unknown", price)
	only(t, cycleAt(t, ctx, org, d.Add(time.Hour), false), engine.Skipped)
	only(t, cycleAt(t, ctx, org, d.Add(24*time.Hour), false), engine.Retried)

	withdrawn := int64(price) - int64(balanceOf(t, ctx, org, "red-bal-unknown"))
	if sq.calls != 0 || withdrawn != price {
		t.Fatalf("one %d-cent renewal: %d card calls, %d cents from the balance; want the balance to pay it once and the card untouched",
			price, sq.calls, withdrawn)
	}
}
