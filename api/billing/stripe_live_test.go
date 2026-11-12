package billing

// These run commerce's real money paths — top-up, decline, 3-D Secure, plan
// purchase and renewal, auto-reload, refund, dispute, and the webhook receiver —
// against STRIPE'S OWN TEST MODE, with Stripe's own test cards. Nothing here
// stands in for Stripe: every PaymentIntent, refund and dispute is a real object
// in the test account, and every webhook body is a real Stripe event read back
// from Stripe's events API.
//
// They need STRIPE_TEST_SECRET_KEY and STRIPE_TEST_PUBLISHABLE_KEY, and fail
// without them. The webhook endpoint (and with it the signing secret) is created
// for the run with the test key and deleted when it ends.
//
// What is NOT Stripe's: the host's finance ledger (fakeLedger, the seam commerce
// credits through), and the delivery of an event to the receiver. Stripe cannot
// reach a test process, so each real event is POSTed to the receiver here,
// signed with the real endpoint's secret by Stripe's documented scheme.

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/providers/stripe"
	types "github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/test/ae"
)

const stripeAPIBase = "https://api.stripe.com/v1"

type liveStripe struct {
	t      *testing.T
	sk     string
	hook   string // the endpoint's signing secret
	ctx    *ae.Context
	org    *organization.Organization
	ledger *fakeLedger
	// subject is this run's buyer, and key its retry keys' prefix: a test account
	// keeps every customer and every idempotency key for days, so a fixed name
	// would meet the last run's.
	subject, run string
}

func liveSetup(t *testing.T) *liveStripe {
	t.Helper()
	sk, pk := os.Getenv("STRIPE_TEST_SECRET_KEY"), os.Getenv("STRIPE_TEST_PUBLISHABLE_KEY")
	if sk == "" || pk == "" {
		t.Fatal("STRIPE_TEST_SECRET_KEY and STRIPE_TEST_PUBLISHABLE_KEY are required: these tests run against Stripe's test mode")
	}
	// The deployment default: Stripe is the only processor that takes a new charge.
	t.Setenv(processor.ChargeProcessors, "")
	run := strconv.FormatInt(time.Now().UnixNano(), 36)
	l := &liveStripe{t: t, sk: sk, run: run, subject: "hanzo/live-" + run}

	ep := l.call("POST", "/webhook_endpoints", url.Values{
		"url":               {"https://example.com/commerce-test"},
		"enabled_events[0]": {"payment_intent.succeeded"}, "enabled_events[1]": {"refund.created"},
		"enabled_events[2]": {"refund.updated"}, "enabled_events[3]": {"charge.dispute.created"},
		"enabled_events[4]": {"charge.dispute.updated"}, "enabled_events[5]": {"charge.dispute.closed"},
	})
	l.hook = str(ep["secret"])
	t.Cleanup(func() { l.call("DELETE", "/webhook_endpoints/"+str(ep["id"]), nil) })

	// The webhook receiver is the global Stripe provider, holding the endpoint's secret.
	prev, _ := processor.Global().Registered(processor.Stripe)
	processor.Global().Register(stripe.NewProvider(stripe.Config{SecretKey: sk, PublishableKey: pk, WebhookSecret: l.hook}))
	t.Cleanup(func() {
		if prev != nil {
			processor.Global().Register(prev)
		}
	})

	l.ledger = newFakeLedger()
	injectLedger(t, l.ledger)
	ctx := ae.NewContext()
	t.Cleanup(ctx.Close)
	l.ctx = &ctx
	l.org = organization.New(datastore.New(ctx))
	l.org.Name = "hanzo"
	l.org.MustCreate()

	pl := plan.New(plan.AuthorityDB(ctx))
	pl.Slug, pl.Name = "pro-3x", "Pro"
	pl.Price, pl.Currency = currency.Cents(1500), currency.USD
	pl.Interval, pl.IntervalCount = types.Monthly, 1
	pl.Category, pl.Status, pl.Managed = "personal", plan.StatusActive, true
	pl.MustCreate()
	return l
}

func str(v any) string { s, _ := v.(string); return s }

// call is one Stripe API call, answered as its JSON object; a refusal fails the test.
func (l *liveStripe) call(method, path string, form url.Values) map[string]any {
	l.t.Helper()
	out, status := l.raw(method, path, form)
	if status >= 300 {
		l.t.Fatalf("stripe %s %s: %d %v", method, path, status, out["error"])
	}
	return out
}

func (l *liveStripe) raw(method, path string, form url.Values) (map[string]any, int) {
	body, status := l.bytes(method, path, form)
	var out map[string]any
	_ = json.Unmarshal(body, &out)
	return out, status
}

func (l *liveStripe) bytes(method, path string, form url.Values) ([]byte, int) {
	l.t.Helper()
	u := stripeAPIBase + path
	var rd io.Reader
	if method == "GET" && form != nil {
		u += "?" + form.Encode()
	} else if form != nil {
		rd = strings.NewReader(form.Encode())
	}
	req, _ := http.NewRequest(method, u, rd)
	req.SetBasicAuth(l.sk, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		l.t.Fatalf("stripe %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return b, resp.StatusCode
}

// event waits for the real Stripe event of type typ whose object satisfies match,
// and returns its body exactly as Stripe serves it.
func (l *liveStripe) event(typ string, match func(obj map[string]any) bool) []byte {
	l.t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for {
		list := l.call("GET", "/events", url.Values{"type": {typ}, "limit": {"50"}})
		items, _ := list["data"].([]any)
		for _, it := range items {
			ev, _ := it.(map[string]any)
			data, _ := ev["data"].(map[string]any)
			obj, _ := data["object"].(map[string]any)
			if match(obj) {
				body, _ := l.bytes("GET", "/events/"+str(ev["id"]), nil)
				return body
			}
		}
		if time.Now().After(deadline) {
			l.t.Fatalf("no %s event arrived within 90s", typ)
		}
		time.Sleep(2 * time.Second)
	}
}

// deliver POSTs a Stripe event to the receiver, signed with the endpoint's
// secret: t=<unix>,v1=HMAC-SHA256(secret, "<t>.<body>").
func (l *liveStripe) deliver(body []byte, secret string) (int, map[string]any) {
	l.t.Helper()
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "." + string(body)))
	req := httptest.NewRequest(http.MethodPost, "/v1/commerce/webhooks/stripe", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", "t="+ts+",v1="+hex.EncodeToString(mac.Sum(nil)))
	resp := driveSeeded(func(c *zip.Ctx) { c.SetContext(*l.ctx) }, "/v1/commerce/webhooks/:provider", req, HandleProviderWebhook)
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// topup is one top-up through TakePayment, then what the host's settle step does
// when the handler returns: it credits the host's ledger under the payment's
// reference (the key a refund's clawback and a webhook's credit find it by).
func (l *liveStripe) topup(source string, cents int64, cur, key string) (*TakePaymentOut, *PaymentFault) {
	out, f := TakePayment(*l.ctx, l.org, TakePaymentIn{
		SourceID: source, AmountCents: cents, Currency: cur, Subject: l.subject, IdempotencyKey: l.run + "-" + key,
	})
	if f == nil {
		if _, _, err := l.ledger.Credit(*l.ctx, creditledger.CreditInput{
			Org: l.org.Name, Subject: l.subject, AmountCents: cents, Currency: cur,
			IdempotencyKey: out.ProcessorRef, Test: out.Test,
		}); err != nil {
			l.t.Fatalf("host settle credit: %v", err)
		}
	}
	return out, f
}

func (l *liveStripe) balance() int64 { return int64(balanceOf(l.t, *l.ctx, l.org, l.subject)) }

// A card that pays: the real PaymentIntent succeeded for the amount, the balance is
// credited once, and Stripe's own payment_intent.succeeded event — delivered, and
// delivered again — credits nothing more.
func TestLiveStripe_TopupSettlesOnce(t *testing.T) {
	l := liveSetup(t)
	out, f := l.topup("pm_card_visa", 2500, "usd", "live-topup-1")
	if f != nil {
		t.Fatalf("top-up: %d %s (%v)", f.Status, f.Message, f.Err)
	}
	if out.BalanceCents != 2500 || !strings.HasPrefix(out.ProcessorRef, "ch_") {
		t.Fatalf("out = %+v; want 2500 on a charge id", out)
	}
	ch := l.call("GET", "/charges/"+out.ProcessorRef, nil)
	if ch["status"] != "succeeded" || ch["amount"].(float64) != 2500 || ch["currency"] != "usd" || ch["livemode"] != false {
		t.Fatalf("Stripe holds %v %v %v", ch["status"], ch["amount"], ch["currency"])
	}

	body := l.event("payment_intent.succeeded", func(o map[string]any) bool { return str(o["latest_charge"]) == out.ProcessorRef })
	credits := len(l.ledger.credits())
	status, resp := l.deliver(body, l.hook)
	if status != 200 || l.balance() != 2500 || len(l.ledger.credits()) != credits {
		t.Fatalf("delivery = %d %v; balance %d, credits %d->%d; want 200, 2500, unchanged", status, resp, l.balance(), credits, len(l.ledger.credits()))
	}
	if status, resp = l.deliver(body, l.hook); status != 200 || resp["duplicate"] != true {
		t.Fatalf("redelivery = %d %v; want a duplicate", status, resp)
	}
}

// A forged or unsigned delivery changes nothing.
func TestLiveStripe_WebhookRefusesWhatStripeDidNotSign(t *testing.T) {
	l := liveSetup(t)
	out, f := l.topup("pm_card_visa", 1000, "usd", "live-forge-1")
	if f != nil {
		t.Fatal(f)
	}
	body := l.event("payment_intent.succeeded", func(o map[string]any) bool { return str(o["latest_charge"]) == out.ProcessorRef })
	if status, _ := l.deliver(body, "whsec_not_the_endpoints_secret"); status != 401 {
		t.Fatalf("a delivery signed with another secret = %d; want 401", status)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/commerce/webhooks/stripe", bytes.NewReader(body))
	if r := driveSeeded(func(c *zip.Ctx) { c.SetContext(*l.ctx) }, "/v1/commerce/webhooks/:provider", req, HandleProviderWebhook); r.StatusCode != 400 {
		t.Fatalf("an unsigned delivery = %d; want 400", r.StatusCode)
	}
}

// A declined card credits nothing, and the next attempt — another card, the same
// amount, the same retry key — is charged: Stripe's saved decline is not replayed.
func TestLiveStripe_DeclineThenAnotherCard(t *testing.T) {
	l := liveSetup(t)
	if _, f := l.topup("pm_card_chargeDeclined", 2500, "usd", "live-decline-1"); f == nil || f.Status != 402 {
		t.Fatalf("declined card = %+v; want 402", f)
	}
	if l.balance() != 0 {
		t.Fatalf("balance %d after a decline", l.balance())
	}
	out, f := l.topup("pm_card_visa", 2500, "usd", "live-decline-1")
	if f != nil || out.BalanceCents != 2500 {
		t.Fatalf("the next card = %+v (%v); want 2500", out, f)
	}
}

// A card that needs its bank's authentication is not paid: the answer is the
// step the browser runs, and nothing is credited.
func TestLiveStripe_AuthenticationRequiredIsTheBuyersStep(t *testing.T) {
	l := liveSetup(t)
	_, f := l.topup("pm_card_authenticationRequired", 2500, "usd", "live-3ds-1")
	if f == nil || f.Status != 402 {
		t.Fatalf("3-D Secure card = %+v; want 402", f)
	}
	var ar authRequired
	if err := json.Unmarshal([]byte(f.Message), &ar); err != nil || ar.Code != "authentication_required" ||
		!strings.HasPrefix(ar.ID, "pi_") || !strings.Contains(ar.ClientSecret, "_secret_") {
		t.Fatalf("answer %q; want authentication_required with a PaymentIntent and its client secret", f.Message)
	}
	pi := l.call("GET", "/payment_intents/"+ar.ID, nil)
	if pi["status"] != "requires_action" || l.balance() != 0 {
		t.Fatalf("Stripe holds %v, balance %d; want requires_action and nothing credited", pi["status"], l.balance())
	}
}

// A PaymentIntent the buyer finished is charged by naming it: for its own amount
// and its own buyer, and once.
func TestLiveStripe_FinishedIntentIsCreditedOnceAndOnlyToItsBuyer(t *testing.T) {
	l := liveSetup(t)
	make := func(subject string, cents int64) string {
		pi := l.call("POST", "/payment_intents", url.Values{
			"amount": {strconv.FormatInt(cents, 10)}, "currency": {"usd"}, "payment_method": {"pm_card_visa"},
			"confirm": {"true"}, "automatic_payment_methods[enabled]": {"true"},
			"automatic_payment_methods[allow_redirects]": {"never"}, "metadata[hanzo_subject]": {subject},
		})
		return str(pi["id"])
	}
	id := make(l.subject, 2500)

	if _, f := l.topup(id, 3000, "usd", "live-pi-amount"); f == nil {
		t.Fatal("an intent for 2500 paid a top-up of 3000")
	}
	if _, f := TakePayment(*l.ctx, l.org, TakePaymentIn{SourceID: id, AmountCents: 2500, Currency: "usd", Subject: "hanzo/mallory-" + l.run, IdempotencyKey: l.run + "-live-pi-thief"}); f == nil {
		t.Fatal("an intent made for alice paid mallory's top-up")
	}
	if l.balance() != 0 {
		t.Fatalf("balance %d after refused attempts", l.balance())
	}
	out, f := l.topup(id, 2500, "usd", "live-pi-1")
	if f != nil || out.BalanceCents != 2500 {
		t.Fatalf("the finished intent = %+v (%v); want 2500", out, f)
	}
	if _, f := l.topup(id, 2500, "usd", "live-pi-2"); f == nil || f.Status != 409 {
		t.Fatalf("the same intent under another key = %+v; want 409", f)
	}
	if l.balance() != 2500 {
		t.Fatalf("balance %d; the intent was credited more than once", l.balance())
	}
	// The intent's guard is COMPLETED, so it is never re-claimed as a crashed
	// attempt once the in-flight window passes.
	db := datastore.New(l.org.Namespaced(*l.ctx))
	g, replayed, err := idemBegin(db, "billing-intent", id)
	if err != nil || !replayed || g.Status != idempotencykey.StatusCompleted {
		t.Fatalf("the spent intent's guard = replayed %v status %v (%v); want a completed one", replayed, g.Status, err)
	}
}

// lostAnswers performs every PaymentIntent create at Stripe and then drops the
// answer, as a connection that dies after the request was received does.
type lostAnswers struct{ next http.RoundTripper }

func (l lostAnswers) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := l.next.RoundTrip(r)
	if err == nil && r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/v1/payment_intents") {
		resp.Body.Close()
		return nil, fmt.Errorf("connection reset by peer")
	}
	return resp, err
}

// A charge whose answer was lost may have gone through: the buyer is not told the
// card was declined, and a retry — re-tokenized, so a new card token — is refused
// rather than charged again.
func TestLiveStripe_LostAnswerIsNeverChargedTwice(t *testing.T) {
	l := liveSetup(t)
	real := http.DefaultTransport
	http.DefaultTransport = lostAnswers{real}
	_, f := l.topup("pm_card_visa", 2500, "usd", "live-lost-1")
	http.DefaultTransport = real
	if f == nil || f.Status != 503 || strings.Contains(strings.ToLower(f.Message), "declined") {
		t.Fatalf("a lost answer = %+v; want 503 and not a decline", f)
	}
	if _, f = l.topup("pm_card_mastercard", 2500, "usd", "live-lost-1"); f == nil {
		t.Fatal("the retry under the same key was charged")
	}
	n := 0
	for _, it := range l.call("GET", "/payment_intents", url.Values{"limit": {"30"}})["data"].([]any) {
		if m, _ := it.(map[string]any)["metadata"].(map[string]any); fmt.Sprint(m["hanzo_subject"]) == l.subject {
			n++
		}
	}
	if n != 1 || l.balance() != 0 {
		t.Fatalf("Stripe holds %d PaymentIntents for the buyer and %d is credited; want one charge, uncredited until reconciled", n, l.balance())
	}
}

// A refund Stripe makes is taken back out of the wallet it credited, once, whether
// its event arrives as created, updated, or twice.
func TestLiveStripe_RefundClawsBack(t *testing.T) {
	l := liveSetup(t)
	out, f := l.topup("pm_card_visa", 2500, "usd", "live-refund-1")
	if f != nil {
		t.Fatal(f)
	}
	ref := l.call("POST", "/refunds", url.Values{"charge": {out.ProcessorRef}, "amount": {"1000"}})
	match := func(o map[string]any) bool { return str(o["id"]) == str(ref["id"]) && str(o["status"]) == "succeeded" }
	created := l.event("refund.created", match)
	if status, resp := l.deliver(created, l.hook); status != 200 {
		t.Fatalf("refund.created delivery = %d %v", status, resp)
	}
	if len(l.ledger.clawbacks) != 1 || l.ledger.clawbacks[0].AmountCents != 1000 {
		t.Fatalf("clawbacks after a $10 refund = %+v; want one of 1000", l.ledger.clawbacks)
	}
	l.deliver(created, l.hook)
	l.deliver(l.event("refund.updated", match), l.hook)
	if len(l.ledger.clawbacks) != 1 {
		t.Fatalf("a refund clawed back %d times; want once", len(l.ledger.clawbacks))
	}
}

// A dispute Stripe opens withholds the payment's money; one the merchant wins gives it back.
func TestLiveStripe_DisputeClawsBackAndWinRestores(t *testing.T) {
	l := liveSetup(t)
	out, f := l.topup("pm_card_createDispute", 3000, "usd", "live-dispute-1")
	if f != nil {
		t.Fatal(f)
	}
	onCharge := func(o map[string]any) bool { return str(o["charge"]) == out.ProcessorRef }
	opened := l.event("charge.dispute.created", onCharge)
	if status, resp := l.deliver(opened, l.hook); status != 200 {
		t.Fatalf("dispute delivery = %d %v", status, resp)
	}
	if len(l.ledger.clawbacks) != 1 || l.ledger.clawbacks[0].AmountCents != 3000 {
		t.Fatalf("clawbacks after a dispute = %+v; want one of 3000", l.ledger.clawbacks)
	}
	var ev struct {
		Data struct {
			Object map[string]any `json:"object"`
		} `json:"data"`
	}
	_ = json.Unmarshal(opened, &ev)
	// Test mode: this evidence wins the dispute.
	l.call("POST", "/disputes/"+str(ev.Data.Object["id"]), url.Values{"evidence[uncategorized_text]": {"winning_evidence"}, "submit": {"true"}})
	won := l.event("charge.dispute.closed", func(o map[string]any) bool { return onCharge(o) && str(o["status"]) == "won" })
	if status, resp := l.deliver(won, l.hook); status != 200 {
		t.Fatalf("dispute won delivery = %d %v", status, resp)
	}
	if len(l.ledger.restores) != 1 || l.ledger.restores[0].AmountCents != 3000 {
		t.Fatalf("restores after a win = %+v; want one of 3000", l.ledger.restores)
	}
}

// A plan bought with a Stripe card saves the card, charges it for the first
// period, and renews on it — off-session, for the same price, on the same
// customer — until the plan's next period.
func TestLiveStripe_PlanPurchaseAndRenewal(t *testing.T) {
	l := liveSetup(t)
	db := datastore.New(l.org.Namespaced(*l.ctx))
	sale, err := Subscribe(*l.ctx, l.org, SubscribeIn{
		SourceID: "pm_card_visa", PlanID: "pro-3x", Subject: l.subject, Email: "alice@example.com",
		Interval: types.Monthly, IdempotencyKey: l.run + "-live-plan-1",
	})
	if err != nil {
		t.Fatalf("plan purchase: %v", err)
	}
	if sale.AmountCents != 1500 || sale.SubscriptionID == "" {
		t.Fatalf("sale = %+v; want a subscription paid 1500", sale)
	}
	pm := paymentmethod.New(db)
	if err := pm.GetById(sale.PaymentMethodID); err != nil {
		t.Fatal(err)
	}
	cust := customerIDOf(pm)
	if pm.ProviderType != "stripe" || !strings.HasPrefix(cust, "cus_") || pm.Card == nil || pm.Card.Last4 != "4242" || pm.Card.Brand != "visa" {
		t.Fatalf("saved card = %s %s %+v", pm.ProviderType, cust, pm.Card)
	}
	first := l.call("GET", "/payment_intents", url.Values{"customer": {cust}, "limit": {"10"}})["data"].([]any)
	if len(first) != 1 || first[0].(map[string]any)["status"] != "succeeded" || first[0].(map[string]any)["amount"].(float64) != 1500 {
		t.Fatalf("Stripe holds %v after the purchase; want one succeeded PaymentIntent of 1500", first)
	}

	// The period ends.
	s := reload(t, db, sale.SubscriptionID)
	shift := time.Until(s.PeriodEnd) + time.Hour
	s.PeriodStart, s.PeriodEnd = s.PeriodStart.Add(-shift), s.PeriodEnd.Add(-shift)
	if err := s.Update(); err != nil {
		t.Fatal(err)
	}
	out := renewDue(*l.ctx, l.org, "")
	if len(out) != 1 || !out[0].Success || out[0].AmountCents != 1500 {
		t.Fatalf("renewal = %+v; want one success of 1500", out)
	}
	all := l.call("GET", "/payment_intents", url.Values{"customer": {cust}, "limit": {"10"}})["data"].([]any)
	if len(all) != 2 {
		t.Fatalf("Stripe holds %d PaymentIntents after a renewal; want 2", len(all))
	}
	for _, it := range all {
		if pi := it.(map[string]any); pi["status"] != "succeeded" || pi["customer"] != cust {
			t.Fatalf("PaymentIntent %v is %v on %v", pi["id"], pi["status"], pi["customer"])
		}
	}
	if renewed := reload(t, db, sale.SubscriptionID); !renewed.PeriodEnd.After(time.Now()) {
		t.Fatalf("the subscription's period ends %v; want a period running now", renewed.PeriodEnd)
	}
	if again := renewDue(*l.ctx, l.org, ""); len(again) != 0 {
		t.Fatalf("a second sweep renewed %+v; want nothing due", again)
	}
}

// The default auto-reload: a payer under the threshold, with a Stripe card on
// file, is reloaded by an off-session Stripe charge, once, and credited.
func TestLiveStripe_AutoReload(t *testing.T) {
	l := liveSetup(t)
	db := datastore.New(l.org.Namespaced(*l.ctx))
	if _, err := Subscribe(*l.ctx, l.org, SubscribeIn{
		SourceID: "pm_card_visa", PlanID: "pro-3x", Subject: l.subject, Email: "alice@example.com",
		Interval: types.Monthly, IdempotencyKey: l.run + "-live-reload-plan",
	}); err != nil {
		t.Fatalf("plan purchase: %v", err)
	}
	if _, err := WriteAutoRecharge(*l.ctx, l.org, l.subject, AutoRechargeEdit{Enabled: true, ThresholdCents: 1000, AmountCents: 2500}); err != nil {
		t.Fatalf("reload rule: %v", err)
	}
	res, ok := reloadPayer(*l.ctx, nil, l.org, db, l.subject)
	if !ok || res.Error != "" {
		t.Fatalf("reload = %+v ok=%v", res, ok)
	}
	if l.balance() != 2500 {
		t.Fatalf("balance %d after the reload; want 2500", l.balance())
	}
	pm := defaultPaymentMethod(db, l.subject)
	pms := l.call("GET", "/payment_intents", url.Values{"customer": {customerIDOf(pm)}, "limit": {"5"}})["data"].([]any)
	top := pms[0].(map[string]any)
	if top["amount"].(float64) != 2500 || top["status"] != "succeeded" || fmt.Sprint(top["metadata"].(map[string]any)["hanzo_subject"]) != l.subject {
		t.Fatalf("Stripe's newest PaymentIntent = %v", top)
	}
}

// A reload the bank declines moves nothing and says so, and a card Stripe will
// not charge off-session is not a card the reload keeps trying all day.
func TestLiveStripe_ReloadOnADecliningCardCreditsNothing(t *testing.T) {
	l := liveSetup(t)
	db := datastore.New(l.org.Namespaced(*l.ctx))
	// Save a card that attaches but is declined when charged.
	pm, _, err := saveCard(context.Background(), db, mustVault(t, l), l.subject, "alice@example.com", "pm_card_chargeCustomerFail")
	if err != nil {
		t.Fatalf("saving the card: %v", err)
	}
	if _, err := WriteAutoRecharge(*l.ctx, l.org, l.subject, AutoRechargeEdit{Enabled: true, ThresholdCents: 1000, AmountCents: 2500}); err != nil {
		t.Fatalf("reload rule: %v", err)
	}
	res, _ := reloadPayer(*l.ctx, nil, l.org, db, l.subject)
	if res.Error == "" || l.balance() != 0 {
		t.Fatalf("reload on %s = %+v, balance %d; want a refusal and nothing credited", pm.Id(), res, l.balance())
	}
}

func mustVault(t *testing.T, l *liveStripe) cardVault {
	t.Helper()
	cp, ok := cardVaultFrom(processorsForOrg(l.org))
	if !ok {
		t.Fatal("no card vault for the org under the default policy")
	}
	if cp.Type() != processor.Stripe {
		t.Fatalf("the default policy vaults cards with %s; want stripe", cp.Type())
	}
	return cp
}
