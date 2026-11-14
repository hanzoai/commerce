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
	"errors"
	"fmt"
	"github.com/hanzoai/commerce/payment/fx"
	"github.com/hanzoai/commerce/payment/routing"
	"io"
	"math"
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
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	billinginvoice "github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/pendingintent"
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
	start        time.Time
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

	form := url.Values{"url": {"https://example.com/commerce-test"}}
	for i, e := range StripeWebhookEvents {
		form.Set(fmt.Sprintf("enabled_events[%d]", i), e)
	}
	ep := l.call("POST", "/webhook_endpoints", form)
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
	if l.balance() != 2500 || !strings.HasPrefix(out.ProcessorRef, "ch_") {
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
	if f != nil || l.balance() != 2500 {
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
	l.register(id, pendingintent.Topup, 2500)

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
	if f != nil || l.balance() != 2500 {
		t.Fatalf("the finished intent = %+v (%v); want 2500", out, f)
	}
	// The same intent under another key is the credit that is there, not another.
	if out, f := l.topup(id, 2500, "usd", "live-pi-2"); f != nil || l.balance() != 2500 {
		t.Fatalf("the same intent under another key = %+v (%v); want the credit already there", out, f)
	}
	if l.balance() != 2500 {
		t.Fatalf("balance %d; the intent was credited more than once", l.balance())
	}
	// The intent's guard is COMPLETED, so it is never re-claimed as a crashed
	// attempt once the in-flight window passes.
	g, replayed, err := idemBegin(webhookGuardDB(*l.ctx), "billing-intent", id)
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
	pm, _, err := saveCard(context.Background(), db, mustVault(t, l), l.subject, processor.Payer{Email: "alice@example.com"}, "pm_card_chargeCustomerFail")
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
	cp, ok := cardVaultFrom(processorsForOrg(l.org), "")
	if !ok {
		t.Fatal("no card vault for the org under the default policy")
	}
	if cp.Type() != processor.Stripe {
		t.Fatalf("the default policy vaults cards with %s; want stripe", cp.Type())
	}
	return cp
}

// paid is a PaymentIntent that SUCCEEDED for this run's buyer, as one the buyer
// finished in their browser is: its id and its charge.
func (l *liveStripe) paid(cents int64) (id, charge string) {
	pi := l.call("POST", "/payment_intents", url.Values{
		"amount": {strconv.FormatInt(cents, 10)}, "currency": {"usd"}, "payment_method": {"pm_card_visa"},
		"confirm": {"true"}, "automatic_payment_methods[enabled]": {"true"},
		"automatic_payment_methods[allow_redirects]": {"never"}, "metadata[hanzo_subject]": {l.subject},
	})
	return str(pi["id"]), str(pi["latest_charge"])
}

func (l *liveStripe) register(pi, purpose string, cents int64) {
	if err := pendingintent.Put(pi, l.org.Name, l.subject, purpose, cents, "usd", true); err != nil {
		l.t.Fatal(err)
	}
}

// A card sent to its bank is recorded for its buyer before the answer that sends
// them: the amount and the buyer are on file the moment the browser has the secret.
func TestLiveStripe_AuthenticationStepIsRecordedForItsBuyer(t *testing.T) {
	l := liveSetup(t)
	_, f := l.topup("pm_card_authenticationRequired", 2500, "usd", "live-pending-1")
	var ar authRequired
	if f == nil || json.Unmarshal([]byte(f.Message), &ar) != nil || !strings.HasPrefix(ar.ID, "pi_") {
		t.Fatalf("answer = %+v; want the authentication step", f)
	}
	p, found, err := pendingintent.Get(ar.ID)
	if err != nil || !found || p.Subject != l.subject || p.Cents != 2500 || p.Purpose != pendingintent.Topup || p.Done {
		t.Fatalf("pending intent = %+v found %v err %v; want 2500 for %s, open", p, found, err, l.subject)
	}
}

// The buyer finishes their bank's step and closes the tab: the intent's webhook
// credits them, and the browser's own request — if it ever comes — credits nothing more.
func TestLiveStripe_ClosedTabIsCreditedByTheWebhook(t *testing.T) {
	l := liveSetup(t)
	id, charge := l.paid(2500)
	l.register(id, pendingintent.Topup, 2500)
	body := l.event("payment_intent.succeeded", func(o map[string]any) bool { return str(o["id"]) == id })
	if status, resp := l.deliver(body, l.hook); status != 200 || l.balance() != 2500 {
		t.Fatalf("delivery = %d %v; balance %d; want 200 and 2500", status, resp, l.balance())
	}
	if p, _, _ := pendingintent.Get(id); p == nil || !p.Done {
		t.Fatalf("pending intent after its webhook = %+v; want settled", p)
	}
	if len(l.ledger.credits()) != 1 || l.ledger.credits()[0].IdempotencyKey != charge {
		t.Fatalf("host credits = %+v; want one, keyed on the charge", l.ledger.credits())
	}
	// The browser comes back after all.
	if out, f := l.topup(id, 2500, "usd", "live-closed-tab"); f != nil || l.balance() != 2500 {
		t.Fatalf("the late request = %+v (%v); want the credit that is there, not another", out, f)
	}
	if l.balance() != 2500 {
		t.Fatalf("balance %d; credited twice", l.balance())
	}
}

// An intent that succeeded and that no webhook and no sale settled is settled by
// the reconciler: a wallet's is credited, a plan's that never finished is refunded.
func TestLiveStripe_ReconcilerCreditsAndRefunds(t *testing.T) {
	l := liveSetup(t)
	top, _ := l.paid(2500)
	l.register(top, pendingintent.Topup, 2500)
	reconcileIntent(*l.ctx, mustIntent(t, top))
	if l.balance() != 2500 {
		t.Fatalf("balance %d after the reconciler; want 2500", l.balance())
	}
	reconcileIntent(*l.ctx, mustIntent(t, top))
	if l.balance() != 2500 {
		t.Fatalf("balance %d after reconciling twice; credited again", l.balance())
	}

	sale, charge := l.paid(1500)
	l.register(sale, pendingintent.Sale, 1500)
	reconcileIntent(*l.ctx, mustIntent(t, sale))
	ch := l.call("GET", "/charges/"+charge, nil)
	if ch["refunded"] != true {
		t.Fatalf("charge %s for a plan never bought: refunded=%v; want refunded", charge, ch["refunded"])
	}
	if p, _, _ := pendingintent.Get(sale); p == nil || !p.Done {
		t.Fatalf("the sale's intent = %+v; want settled", p)
	}
}

func mustIntent(t *testing.T, id string) *pendingintent.Intent {
	t.Helper()
	p, found, err := pendingintent.Get(id)
	if err != nil || !found {
		t.Fatalf("pending intent %s: %v %v", id, found, err)
	}
	return p
}

// A card whose bank wants authentication to be SAVED is answered as the step, with
// nothing saved; the SetupIntent the buyer then finishes saves it, for that customer only.
func TestLiveStripe_CardSaveNeedingAuthenticationFinishes(t *testing.T) {
	l := liveSetup(t)
	db := datastore.New(l.org.Namespaced(*l.ctx))
	cp := mustVault(t, l)

	_, _, err := saveCard(context.Background(), db, cp, l.subject, processor.Payer{Email: "buyer@example.com"}, "pm_card_authenticationRequiredSetupForOffSession")
	var ar *processor.ActionRequired
	if !errors.As(err, &ar) || !strings.HasPrefix(ar.ID, "seti_") || ar.ClientSecret == "" {
		t.Fatalf("saving a card that needs authentication = %v; want the step for a SetupIntent", err)
	}
	if rows := methodsOf(t, db, l.subject); len(rows) != 0 {
		t.Fatalf("%d cards saved before the buyer authenticated", len(rows))
	}

	// What the browser's step leaves behind: a SetupIntent that SUCCEEDED for the customer.
	cust, err := vaultCustomer(context.Background(), db, cp, l.subject, processor.Payer{Email: "buyer@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	si := l.call("POST", "/setup_intents", url.Values{
		"customer": {cust}, "payment_method": {"pm_card_visa"}, "confirm": {"true"}, "usage": {"off_session"},
		"automatic_payment_methods[enabled]": {"true"}, "automatic_payment_methods[allow_redirects]": {"never"},
	})
	pm, created, err := saveCard(context.Background(), db, cp, l.subject, processor.Payer{Email: "buyer@example.com"}, str(si["id"]))
	if err != nil || !created || pm.Card == nil || pm.Card.Last4 != "4242" || metaStr(pm, "country") != "US" {
		t.Fatalf("finishing the setup = %+v created %v err %v; want the visa saved with its country", pm, created, err)
	}
	// Another customer cannot claim it.
	if _, err := cp.(vaulter).Vault(context.Background(), "cus_someone_else", str(si["id"])); err == nil {
		t.Fatal("a SetupIntent finished for one customer saved a card for another")
	}
}

func methodsOf(t *testing.T, db *datastore.Datastore, subject string) []*paymentmethod.PaymentMethod {
	t.Helper()
	var out []*paymentmethod.PaymentMethod
	iter := paymentmethod.Query(db).Ancestor(db.NewKey("synckey", "", 1, nil)).Filter("CustomerId=", subject).Run()
	for {
		pm := paymentmethod.New(db)
		if _, err := iter.Next(pm); err != nil {
			return out
		}
		out = append(out, pm)
	}
}

// A plan is bought with a card whose bank was asked: the sale is finished from the
// PaymentIntent the buyer finished, on the card already saved — for its own amount
// and account, once.
func TestLiveStripe_PlanFinishedFromTheBuyersIntent(t *testing.T) {
	l := liveSetup(t)
	db := datastore.New(l.org.Namespaced(*l.ctx))
	cp := mustVault(t, l)
	pm, _, err := saveCard(context.Background(), db, cp, l.subject, processor.Payer{Email: "buyer@example.com"}, "pm_card_visa")
	if err != nil {
		t.Fatal(err)
	}
	// The buyer's browser finished a charge of the plan's price on that saved card.
	pi := l.call("POST", "/payment_intents", url.Values{
		"amount": {"1500"}, "currency": {"usd"}, "payment_method": {pm.ProviderRef}, "customer": {customerIDOf(pm)},
		"confirm": {"true"}, "automatic_payment_methods[enabled]": {"true"},
		"automatic_payment_methods[allow_redirects]": {"never"}, "metadata[hanzo_subject]": {l.subject},
	})
	id := str(pi["id"])
	l.register(id, pendingintent.Sale, 1500)

	in := SubscribeIn{SourceID: id, PlanID: "pro-3x", Subject: l.subject, Email: "buyer@example.com", Interval: types.Monthly, IdempotencyKey: l.run + "-finish-1"}
	sale, err := Subscribe(*l.ctx, l.org, in)
	if err != nil || sale.SubscriptionID == "" || sale.AmountCents != 1500 || sale.PaymentMethodID != pm.Id() {
		t.Fatalf("sale from the finished intent = %+v (%v); want a subscription paid 1500 on the saved card", sale, err)
	}
	if p, _, _ := pendingintent.Get(id); p == nil || !p.Done {
		t.Fatalf("the sale's intent = %+v; want settled", p)
	}
	in.IdempotencyKey = l.run + "-finish-2"
	if _, err := Subscribe(*l.ctx, l.org, in); err == nil {
		t.Fatal("the same finished intent bought a plan twice")
	}
	// An intent for another buyer is not this one's.
	other := l.call("POST", "/payment_intents", url.Values{
		"amount": {"1500"}, "currency": {"usd"}, "payment_method": {"pm_card_visa"}, "confirm": {"true"},
		"automatic_payment_methods[enabled]": {"true"}, "automatic_payment_methods[allow_redirects]": {"never"},
		"metadata[hanzo_subject]": {"hanzo/someone-else"},
	})
	if _, err := Subscribe(*l.ctx, l.org, SubscribeIn{SourceID: str(other["id"]), PlanID: "pro-3x", Subject: l.subject, Interval: types.Monthly, IdempotencyKey: l.run + "-finish-3"}); err == nil {
		t.Fatal("another buyer's intent bought this buyer a plan")
	}
}

// A card's country is read from the card, and a sanctioned one is refused by the
// processor itself: Stripe's test cards are issued in no sanctioned country, so
// what is proved is that the country is read and that the rule is the one the
// selection uses.
func TestLiveStripe_CardCountryIsReadFromTheCard(t *testing.T) {
	l := liveSetup(t)
	cp := mustVault(t, l)
	_ = cp
	for pm, want := range map[string]string{"pm_card_visa": "US", "pm_card_br": "BR"} {
		pi := l.call("POST", "/payment_intents", url.Values{
			"amount": {"500"}, "currency": {"usd"}, "payment_method": {pm}, "confirm": {"true"},
			"automatic_payment_methods[enabled]": {"true"}, "automatic_payment_methods[allow_redirects]": {"never"},
		})
		got := l.call("GET", "/payment_methods/"+str(pi["payment_method"]), nil)["card"].(map[string]any)["country"]
		if got != want || processor.Sanctioned(want) {
			t.Fatalf("%s: Stripe reports country %v; want %s, not sanctioned", pm, got, want)
		}
	}
	for _, c := range []string{"IR", "KP", "CU", "SY", "RU", "BY"} {
		if !processor.Sanctioned(c) {
			t.Fatalf("%s is not refused", c)
		}
	}
}

// A payment refunded — by the reconciler or by anyone — is not money in hand: the
// intent still reads succeeded, and neither a top-up nor a sale may redeem it.
func TestLiveStripe_RefundedIntentCannotBeRedeemed(t *testing.T) {
	l := liveSetup(t)
	id, charge := l.paid(2500)
	l.call("POST", "/refunds", url.Values{"charge": {charge}})
	if _, f := l.topup(id, 2500, "usd", "live-refunded-1"); f == nil {
		t.Fatal("a refunded intent was credited")
	}
	if l.balance() != 0 {
		t.Fatalf("balance %d from a refunded intent", l.balance())
	}

	// The reconciler's own refund of a sale that never finished.
	sale, _ := l.paid(1500)
	l.register(sale, pendingintent.Sale, 1500)
	reconcileIntent(*l.ctx, mustIntent(t, sale))
	db := datastore.New(l.org.Namespaced(*l.ctx))
	cp := mustVault(t, l)
	if _, _, err := saveCard(context.Background(), db, cp, l.subject, processor.Payer{Email: "buyer@example.com"}, "pm_card_visa"); err != nil {
		t.Fatal(err)
	}
	if _, err := Subscribe(*l.ctx, l.org, SubscribeIn{SourceID: sale, PlanID: "pro-3x", Subject: l.subject, Interval: types.Monthly, IdempotencyKey: l.run + "-refunded-sale"}); err == nil {
		t.Fatal("an intent the reconciler refunded bought a plan")
	}
	if _, f := l.topup(sale, 1500, "usd", "live-refunded-2"); f == nil {
		t.Fatal("an intent the reconciler refunded was credited to a wallet")
	}
}

// One payment is one use: a top-up's intent cannot also buy a plan, and a plan's
// cannot be a top-up.
func TestLiveStripe_OneIntentIsOneUse(t *testing.T) {
	l := liveSetup(t)
	db := datastore.New(l.org.Namespaced(*l.ctx))
	cp := mustVault(t, l)
	if _, _, err := saveCard(context.Background(), db, cp, l.subject, processor.Payer{Email: "buyer@example.com"}, "pm_card_visa"); err != nil {
		t.Fatal(err)
	}
	top, _ := l.paid(1500)
	l.register(top, pendingintent.Topup, 1500)
	body := l.event("payment_intent.succeeded", func(o map[string]any) bool { return str(o["id"]) == top })
	if status, _ := l.deliver(body, l.hook); status != 200 || l.balance() != 1500 {
		t.Fatalf("top-up intent by webhook: balance %d", l.balance())
	}
	if _, err := Subscribe(*l.ctx, l.org, SubscribeIn{SourceID: top, PlanID: "pro-3x", Subject: l.subject, Interval: types.Monthly, IdempotencyKey: l.run + "-onceuse-1"}); err == nil {
		t.Fatal("a top-up's intent, already credited, bought a plan too")
	}
	plan, _ := l.paid(1500)
	l.register(plan, pendingintent.Sale, 1500)
	if _, f := l.topup(plan, 1500, "usd", "live-onceuse-2"); f == nil || f.Status != 422 {
		t.Fatalf("a plan's intent as a top-up = %+v; want 422", f)
	}
	if l.balance() != 1500 {
		t.Fatalf("balance %d; a plan's intent reached a wallet", l.balance())
	}
}

// The browser's top-up and the intent's webhook credit one payment once, in either
// order.
func TestLiveStripe_BrowserThenWebhookCreditsOnce(t *testing.T) {
	l := liveSetup(t)
	id, _ := l.paid(2500)
	l.register(id, pendingintent.Topup, 2500)
	if out, f := l.topup(id, 2500, "usd", "live-order-1"); f != nil || l.balance() != 2500 {
		t.Fatalf("browser first = %+v (%v)", out, f)
	}
	body := l.event("payment_intent.succeeded", func(o map[string]any) bool { return str(o["id"]) == id })
	if status, _ := l.deliver(body, l.hook); status != 200 || l.balance() != 2500 {
		t.Fatalf("the webhook after the browser: balance %d; want 2500", l.balance())
	}
}

// A buyer may have only a few payments waiting on their bank.
func TestLiveStripe_UnfinishedStepsAreCapped(t *testing.T) {
	l := liveSetup(t)
	var last *PaymentFault
	for i := 0; i < maxUnfinished+1; i++ {
		_, last = l.topup("pm_card_authenticationRequired", 2500, "usd", fmt.Sprintf("live-cap-%d", i))
	}
	if last == nil || last.Status != 429 {
		t.Fatalf("the step past the cap = %+v; want 429", last)
	}
}

// A top-up finished in the browser closes its pending row, so the buyer's cap and
// the reconciler's sweep are not held by money already credited.
func TestLiveStripe_FinishedTopupClosesItsRow(t *testing.T) {
	l := liveSetup(t)
	id, _ := l.paid(2500)
	l.register(id, pendingintent.Topup, 2500)
	if out, f := l.topup(id, 2500, "usd", "live-close-1"); f != nil || out.TransactionID == "" {
		t.Fatalf("top-up = %+v (%v); want a receipt id", out, f)
	}
	if p, _, _ := pendingintent.Get(id); p == nil || !p.Done {
		t.Fatalf("pending row after the browser's top-up = %+v; want settled", p)
	}
	// A replay under another key answers the receipt that is there.
	if out, f := l.topup(id, 2500, "usd", "live-close-2"); f != nil || out.TransactionID == "" || l.balance() != 2500 {
		t.Fatalf("replay = %+v (%v) balance %d; want the same receipt, 2500", out, f, l.balance())
	}
}

// A charge refunded before anything credited it is not credited by the webhook or
// the reconciler either.
func TestLiveStripe_RefundedBeforeCreditIsNotCredited(t *testing.T) {
	l := liveSetup(t)
	id, charge := l.paid(2500)
	l.register(id, pendingintent.Topup, 2500)
	l.call("POST", "/refunds", url.Values{"charge": {charge}})
	reconcileIntent(*l.ctx, mustIntent(t, id))
	if l.balance() != 0 {
		t.Fatalf("balance %d after reconciling a refunded charge", l.balance())
	}
	body := l.event("payment_intent.succeeded", func(o map[string]any) bool { return str(o["id"]) == id })
	if status, _ := l.deliver(body, l.hook); status != 200 || l.balance() != 0 {
		t.Fatalf("webhook for a refunded charge: balance %d", l.balance())
	}
}

// A saved card's plan purchase finishes from the buyer's intent with the card named.
func TestLiveStripe_SavedCardPlanFinishesFromIntentWithTheCardNamed(t *testing.T) {
	l := liveSetup(t)
	db := datastore.New(l.org.Namespaced(*l.ctx))
	cp := mustVault(t, l)
	pm, _, err := saveCard(context.Background(), db, cp, l.subject, processor.Payer{Email: "buyer@example.com"}, "pm_card_visa")
	if err != nil {
		t.Fatal(err)
	}
	pi := l.call("POST", "/payment_intents", url.Values{
		"amount": {"1500"}, "currency": {"usd"}, "payment_method": {pm.ProviderRef}, "customer": {customerIDOf(pm)},
		"confirm": {"true"}, "automatic_payment_methods[enabled]": {"true"},
		"automatic_payment_methods[allow_redirects]": {"never"}, "metadata[hanzo_subject]": {l.subject},
	})
	l.register(str(pi["id"]), pendingintent.Sale, 1500)
	sale, serr := Subscribe(*l.ctx, l.org, SubscribeIn{SourceID: str(pi["id"]), MethodID: pm.Id(), PlanID: "pro-3x", Subject: l.subject, Interval: types.Monthly, IdempotencyKey: l.run + "-saved-finish"})
	if serr != nil || sale.SubscriptionID == "" || sale.AmountCents != 1500 {
		t.Fatalf("saved-card plan from its intent = %+v (%v)", sale, serr)
	}
}

// A webhook for a plan's intent must NEVER close its row: only the sale finishing,
// or the reconciler's refund, does. Closed by the webhook, a plan charged for and
// never finished would have no plan, no refund and no alarm.
func TestRed_SaleWebhookClosesRowSoReconcilerNeverRefunds(t *testing.T) {
	l := liveSetup(t)
	id, charge := l.paid(1500)
	l.register(id, pendingintent.Sale, 1500)
	body := l.event("payment_intent.succeeded", func(o map[string]any) bool { return str(o["id"]) == id })
	if status, _ := l.deliver(body, l.hook); status != 200 {
		t.Fatalf("webhook status %d", status)
	}
	if p := mustIntent(t, id); p.Done {
		t.Fatal("the webhook closed a plan's pending row")
	}
	if l.balance() != 0 {
		t.Fatalf("balance %d: a plan's payment reached a wallet", l.balance())
	}
	if err := reconcileIntent(*l.ctx, mustIntent(t, id)); err != nil {
		t.Fatal(err)
	}
	if ch := l.call("GET", "/charges/"+charge, nil); ch["refunded"] != true {
		t.Fatalf("the plan that was never bought was not refunded: refunded=%v", ch["refunded"])
	}
}

// charge.refunded alone — the one event an endpoint may be subscribed to — takes
// the refund back, once, by the refund's own id, even beside refund.created.
func TestLiveStripe_ChargeRefundedClawsBack(t *testing.T) {
	l := liveSetup(t)
	out, f := l.topup("pm_card_visa", 2500, "usd", "live-chargerefunded")
	if f != nil {
		t.Fatal(f)
	}
	ref := l.call("POST", "/refunds", url.Values{"charge": {out.ProcessorRef}, "amount": {"1000"}})
	ev := l.event("charge.refunded", func(o map[string]any) bool { return str(o["id"]) == out.ProcessorRef })
	if status, resp := l.deliver(ev, l.hook); status != 200 {
		t.Fatalf("charge.refunded delivery = %d %v", status, resp)
	}
	if len(l.ledger.clawbacks) != 1 || l.ledger.clawbacks[0].AmountCents != 1000 {
		t.Fatalf("clawbacks after charge.refunded = %+v; want one of 1000", l.ledger.clawbacks)
	}
	created := l.event("refund.created", func(o map[string]any) bool { return str(o["id"]) == str(ref["id"]) && str(o["status"]) == "succeeded" })
	l.deliver(created, l.hook)
	if len(l.ledger.clawbacks) != 1 {
		t.Fatalf("the same refund clawed back %d times via two events", len(l.ledger.clawbacks))
	}
}

// One org's rows that keep erroring do not hold another org's: they back off, are
// closed with an alarm after giveUp sweeps, and a good row is settled in the same
// sweep.
func TestLiveStripe_ReconcilerIsFairAndBacksOff(t *testing.T) {
	l := liveSetup(t)
	var ghosts []*pendingintent.Intent
	for i := 0; i < 30; i++ {
		id := fmt.Sprintf("pi_ghost_%s_%d", l.run, i)
		if err := pendingintent.Put(id, "no-such-org-"+l.run, "ghost/x", pendingintent.Topup, 100, "usd", true); err != nil {
			t.Fatal(err)
		}
		ghosts = append(ghosts, mustIntent(t, id))
	}
	good, _ := l.paid(2500)
	l.register(good, pendingintent.Topup, 2500)
	reconcileAll(*l.ctx, append(ghosts, mustIntent(t, good)))
	if l.balance() != 2500 {
		t.Fatalf("balance %d: thirty stuck rows of another org held a good one", l.balance())
	}
	g := mustIntent(t, ghosts[0].Id())
	if g.Done || g.Tries != 1 || !g.Next.After(time.Now()) || g.Last == "" {
		t.Fatalf("a stuck row = %+v; want one failed try, backed off, with its reason", g)
	}
	// Due again only after the back-off.
	open, err := pendingintent.Open(time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range open {
		if p.Id() == ghosts[0].Id() {
			t.Fatal("a backed-off row is due at once")
		}
	}
	// A row that keeps erroring is closed.
	for i := 0; i < giveUp; i++ {
		if _, err := pendingintent.Failed(ghosts[1].Id(), "still failing"); err != nil {
			t.Fatal(err)
		}
	}
	reconcileAll(*l.ctx, []*pendingintent.Intent{mustIntent(t, ghosts[1].Id())})
	if !mustIntent(t, ghosts[1].Id()).Done {
		t.Fatal("a row that failed giveUp sweeps was not closed")
	}
}

// pastDue is a plan bought with a good Stripe card, renewed onto a card that declines
// off-session so the period's invoice stays open and the subscription goes past due,
// with the buyer's bank step FINISHED for that invoice: a succeeded PaymentIntent on
// the good card for the amount owed, recorded for the invoice.
type pastDue struct {
	sale     *Sale
	invoice  *billinginvoice.BillingInvoice
	good     *paymentmethod.PaymentMethod
	customer string
	intent   string
	charge   string
}

func (l *liveStripe) pastDue() *pastDue {
	t := l.t
	t.Helper()
	db := datastore.New(l.org.Namespaced(*l.ctx))
	sale, err := Subscribe(*l.ctx, l.org, SubscribeIn{
		SourceID: "pm_card_visa", PlanID: "pro-3x", Subject: l.subject, Email: "buyer@example.com",
		Interval: types.Monthly, IdempotencyKey: l.run + "-pastdue-plan",
	})
	if err != nil {
		t.Fatalf("plan purchase: %v", err)
	}
	good := paymentmethod.New(db)
	if err := good.GetById(sale.PaymentMethodID); err != nil {
		t.Fatal(err)
	}
	// A card that attaches and declines when charged, and the subscription renews on it.
	bad, _, err := saveCard(context.Background(), db, mustVault(t, l), l.subject, processor.Payer{Email: "buyer@example.com"}, "pm_card_chargeCustomerFail")
	if err != nil {
		t.Fatal(err)
	}
	pointAt := func(pm *paymentmethod.PaymentMethod) {
		s := reload(t, db, sale.SubscriptionID)
		s.DefaultPaymentMethod = pm.Id()
		if err := s.Update(); err != nil {
			t.Fatal(err)
		}
	}
	pointAt(bad)
	s := reload(t, db, sale.SubscriptionID)
	shift := time.Until(s.PeriodEnd) + time.Hour
	s.PeriodStart, s.PeriodEnd = s.PeriodStart.Add(-shift), s.PeriodEnd.Add(-shift)
	if err := s.Update(); err != nil {
		t.Fatal(err)
	}
	if out := renewDue(*l.ctx, l.org, ""); len(out) != 1 || out[0].Success {
		t.Fatalf("the renewal on the declining card = %+v; want one failure", out)
	}
	inv := billinginvoice.New(db)
	if found, err := inv.Query().Filter("SubscriptionId=", sale.SubscriptionID).Filter("Status=", billinginvoice.Open).Get(); err != nil || !found {
		t.Fatalf("the past-due invoice: found %v err %v", found, err)
	}

	// The buyer's bank step for it, finished: a succeeded intent on the good card.
	cust := customerIDOf(good)
	pi := l.call("POST", "/payment_intents", url.Values{
		"amount": {"1500"}, "currency": {"usd"}, "payment_method": {good.ProviderRef}, "customer": {cust},
		"confirm": {"true"}, "automatic_payment_methods[enabled]": {"true"},
		"automatic_payment_methods[allow_redirects]": {"never"}, "metadata[hanzo_subject]": {l.subject},
	})
	if err := pendingintent.PutRef(str(pi["id"]), l.org.Name, l.subject, pendingintent.Invoice, inv.Id(), 1500, "usd", true); err != nil {
		t.Fatal(err)
	}
	pointAt(good)
	return &pastDue{sale: sale, invoice: inv, good: good, customer: cust, intent: str(pi["id"]), charge: str(pi["latest_charge"])}
}

func (l *liveStripe) succeededFor(customer string) int {
	n := 0
	for _, it := range l.call("GET", "/payment_intents", url.Values{"customer": {customer}, "limit": {"30"}})["data"].([]any) {
		if it.(map[string]any)["status"] == "succeeded" {
			n++
		}
	}
	return n
}

// The buyer finished their bank's step for a past-due invoice and never pressed pay
// again: the cron settles the invoice from that payment, and charges nothing more.
func TestLiveStripe_CronSettlesAPastDueInvoiceFromTheBuyersIntent(t *testing.T) {
	l := liveSetup(t)
	pd := l.pastDue()
	// The dunning retry the cron runs (billing/workflows: engine.CollectInvoice with the
	// off-session charger) — the buyer present is not.
	db0 := datastore.New(l.org.Namespaced(*l.ctx))
	inv := billinginvoice.New(db0)
	if err := inv.GetById(pd.invoice.Id()); err != nil {
		t.Fatal(err)
	}
	res, err := engine.CollectInvoice(*l.ctx, db0, inv, prepaidFor(*l.ctx, l.org), chargeProviderForOrg(l.org))
	if err != nil || !res.Success {
		t.Fatalf("the cron after the buyer paid = %+v (%v); want it settled", res, err)
	}
	if err := inv.Update(); err != nil { // the activity persists what the collection left
		t.Fatal(err)
	}
	if n := l.succeededFor(pd.customer); n != 2 {
		t.Fatalf("Stripe holds %d succeeded payments for the buyer; want 2 (the first period and the invoice's intent), not a third charge", n)
	}
	db := datastore.New(l.org.Namespaced(*l.ctx))
	paid := billinginvoice.New(db)
	if err := paid.GetById(pd.invoice.Id()); err != nil || paid.Status != billinginvoice.Paid {
		t.Fatalf("the invoice = %v (%v); want paid", paid.Status, err)
	}
	if p := mustIntent(t, pd.intent); !p.Done {
		t.Fatal("the invoice's intent row is still open")
	}
}

// A buyer who has NOT finished their bank's step is not charged again by the cron.
func TestLiveStripe_CronLeavesAnInvoiceWaitingOnItsBuyer(t *testing.T) {
	l := liveSetup(t)
	pd := l.pastDue()
	// Replace the finished intent with one still waiting on the bank.
	if _, err := pendingintent.Settle(pd.intent); err != nil {
		t.Fatal(err)
	}
	waiting := l.call("POST", "/payment_intents", url.Values{
		"amount": {"1500"}, "currency": {"usd"}, "payment_method": {"pm_card_authenticationRequired"}, "confirm": {"true"},
		"automatic_payment_methods[enabled]": {"true"}, "automatic_payment_methods[allow_redirects]": {"never"},
		"metadata[hanzo_subject]": {l.subject},
	})
	if err := pendingintent.PutRef(str(waiting["id"]), l.org.Name, l.subject, pendingintent.Invoice, pd.invoice.Id(), 1500, "usd", true); err != nil {
		t.Fatal(err)
	}
	db := datastore.New(l.org.Namespaced(*l.ctx))
	inv := billinginvoice.New(db)
	if err := inv.GetById(pd.invoice.Id()); err != nil {
		t.Fatal(err)
	}
	res, _ := engine.CollectInvoice(*l.ctx, db, inv, prepaidFor(*l.ctx, l.org), chargeProviderForOrg(l.org))
	if res == nil || res.Success || !strings.Contains(res.Error, "waiting for its buyer") {
		t.Fatalf("the cron = %+v; want it to leave the invoice waiting", res)
	}
	if n := l.succeededFor(pd.customer); n != 2 {
		t.Fatalf("%d succeeded payments; the cron charged beside a waiting intent", n)
	}
	// The buyer pressing pay is answered the same step, not charged a second time.
	got, f := CollectInvoice(*l.ctx, l.org, pd.invoice.Id(), nil)
	if f != nil || got.Paid || !strings.Contains(got.Reason, "authentication_required") || !strings.Contains(got.Reason, str(waiting["id"])) {
		t.Fatalf("pay while waiting = %+v (%v); want the step for %s again", got, f, str(waiting["id"]))
	}
}

// A buyer who has NOT finished is not charged again by the cron, and is answered the
// same step by the next press of "pay"; one who finished after a part-payment landed
// is settled for what is owed and returned the rest.
func TestLiveStripe_InvoiceIntentFinishesAfterTheAmountOwedMoves(t *testing.T) {
	l := liveSetup(t)
	pd := l.pastDue()
	db := datastore.New(l.org.Namespaced(*l.ctx))
	// A part-payment of $5 lands before the buyer presses pay.
	inv := billinginvoice.New(db)
	if err := inv.GetById(pd.invoice.Id()); err != nil {
		t.Fatal(err)
	}
	inv.AmountPaid = 500
	if err := inv.Update(); err != nil {
		t.Fatal(err)
	}
	// The buyer has auto-reload on: an excess refund must not stop it.
	if _, err := WriteAutoRecharge(*l.ctx, l.org, l.subject, AutoRechargeEdit{Enabled: true, ThresholdCents: 1000, AmountCents: 2500}); err != nil {
		t.Fatalf("reload rule: %v", err)
	}
	got, f := CollectInvoice(*l.ctx, l.org, inv.Id(), nil)
	if f != nil || !got.Paid {
		t.Fatalf("pay again = %+v (%v); want the invoice paid", got, f)
	}
	if n := l.succeededFor(pd.customer); n != 2 {
		t.Fatalf("Stripe holds %d succeeded payments; want 2", n)
	}
	ch := l.call("GET", "/charges/"+pd.charge, nil)
	if ch["amount_refunded"].(float64) != 500 {
		t.Fatalf("the intent took 1500 for 1000 owed: refunded %v; want the 500 excess returned", ch["amount_refunded"])
	}

	// Stripe tells commerce about that refund, both ways it can. It is an overpayment
	// returned, not a refund of the invoice: nothing is recorded against the invoice,
	// nothing is clawed back, and the buyer's reload stays on.
	created := l.event("refund.created", func(o map[string]any) bool { return str(o["charge"]) == pd.charge && str(o["status"]) == "succeeded" })
	if status, resp := l.deliver(created, l.hook); status != 200 {
		t.Fatalf("refund.created = %d %v", status, resp)
	}
	charged := l.event("charge.refunded", func(o map[string]any) bool { return str(o["id"]) == pd.charge })
	if status, resp := l.deliver(charged, l.hook); status != 200 {
		t.Fatalf("charge.refunded = %d %v", status, resp)
	}
	after := billinginvoice.New(db)
	if err := after.GetById(inv.Id()); err != nil {
		t.Fatal(err)
	}
	if after.Refunded() != 0 || after.Status != billinginvoice.Paid {
		t.Fatalf("invoice after the excess refund: refunded %d status %s; want 0 and paid", after.Refunded(), after.Status)
	}
	if len(l.ledger.clawbacks) != 0 {
		t.Fatalf("an excess refund clawed back %+v", l.ledger.clawbacks)
	}
	if rule := loadAutoRecharge(db, l.subject); rule == nil || !rule.Enabled {
		t.Fatalf("the buyer's reload was stopped by the excess refund: %+v", rule)
	}
}

// intentsFor counts the PaymentIntents Stripe holds for this run's buyer: an intent
// exists at Stripe the moment a charge reaches it, whatever the request is answered.
func (l *liveStripe) intentsFor() int {
	l.t.Helper()
	out := l.call("GET", "/payment_intents?limit=100&created[gte]="+strconv.FormatInt(l.start.Unix(), 10), nil)
	n := 0
	data, _ := out["data"].([]any)
	for _, d := range data {
		if m, _ := d.(map[string]any)["metadata"].(map[string]any); str(m["hanzo_subject"]) == l.subject {
			n++
		}
	}
	return n
}

// A buyer with five payments already waiting on their bank is refused BEFORE Stripe is
// asked: the cap used to be checked after the intent existed, so each refusal left an
// intent at Stripe that no one held the secret to. One payment settled makes room, and
// the next is charged.
func TestLiveStripe_ABuyerAtTheCapIsRefusedBeforeStripeIsAsked(t *testing.T) {
	l := liveSetup(t)
	l.start = time.Now().Add(-time.Minute)

	var open []string
	for i := 0; i < maxUnfinished; i++ {
		id := fmt.Sprintf("pi_cap_%s_%d", l.run, i)
		if err := pendingintent.Put(id, l.org.Name, strings.ToLower(l.subject), pendingintent.Topup, 500, "usd", true); err != nil {
			t.Fatal(err)
		}
		open = append(open, id)
	}

	if _, f := l.topup("pm_card_threeDSecure2Required", 500, "usd", "cap-1"); f == nil || f.Status != 429 {
		t.Fatalf("a buyer at the cap = %+v; want 429", f)
	}
	if n := l.intentsFor(); n != 0 {
		t.Fatalf("Stripe holds %d intent(s) for a buyer refused at the cap; want none", n)
	}

	if _, err := pendingintent.Settle(open[0]); err != nil {
		t.Fatal(err)
	}
	if _, f := l.topup("pm_card_visa", 500, "usd", "cap-2"); f != nil {
		t.Fatalf("with room again: %d %s (%v)", f.Status, f.Message, f.Err)
	}
	if n := l.intentsFor(); n != 1 {
		t.Fatalf("Stripe holds %d intent(s) once the buyer had room; want 1", n)
	}
}

// A buyer in Brazil is quoted in reais and gets an intent Stripe will offer Pix and
// boleto on: the amount is the quote, the credit is the dollars, and the record that
// attributes the payment is written before the secret leaves.
func TestLiveStripe_LocalIntentIsQuotedAndRecorded(t *testing.T) {
	l := liveSetup(t)
	old := fx.Rate
	fx.Rate = func(context.Context, string) (float64, error) { return 5.5, nil }
	t.Cleanup(func() { fx.Rate = old })

	out, f := OpenTopupIntent(*l.ctx, l.org, TopupIntentIn{Subject: l.subject, Country: "BR", AmountCents: 500})
	if f != nil {
		t.Fatalf("open: %d %s (%v)", f.Status, f.Message, f.Err)
	}
	want := int64(math.Ceil(500 * 5.5 * 1.02))
	if out.Currency != "brl" || out.Amount != want || out.UsdCents != 500 || out.ClientSecret == "" || out.PublishableKey == "" {
		t.Fatalf("out = %+v; want %d brl for 500 usd cents", out, want)
	}
	pi := l.call("GET", "/payment_intents/"+out.IntentID, nil)
	if str(pi["currency"]) != "brl" || int64(pi["amount"].(float64)) != want || pi["status"] != "requires_payment_method" {
		t.Fatalf("the intent at Stripe = %v %v %v", pi["currency"], pi["amount"], pi["status"])
	}
	types := fmt.Sprint(pi["payment_method_types"])
	if !strings.Contains(types, "pix") || !strings.Contains(types, "card") {
		t.Fatalf("Stripe offers %s on this intent; want pix and card", types)
	}
	rec, found, _ := pendingintent.Get(out.IntentID)
	if !found || rec.Cents != 500 || rec.Currency != "usd" || rec.PaidCents != want || rec.PaidCurrency != "brl" || rec.Subject != l.subject {
		t.Fatalf("the waiting record = %+v", rec)
	}
	st, f := ReadTopupIntent(*l.ctx, l.org, l.subject, out.IntentID)
	if f != nil || st.State != "created" || st.Credited {
		t.Fatalf("state = %+v %v; want created, not credited", st, f)
	}
	if _, f := ReadTopupIntent(*l.ctx, l.org, "someone-else", out.IntentID); f == nil || f.Status != 404 {
		t.Fatalf("another buyer read this payment: %v", f)
	}
	if l.balance() != 0 {
		t.Fatalf("opening a payment credited %d", l.balance())
	}
	l.call("POST", "/payment_intents/"+out.IntentID+"/cancel", nil)
}

// ONE PAYMENT, ONE LEDGER ENTRY, whichever writer reaches it first. Real Stripe test-mode
// intents, commerce's own credit paths, no stand-ins for any of them: the buyer's
// in-session finish (TakePayment), their retry, the signed webhook delivered twice, and
// the reconciler's sweep over the pending row. The ledger's dedup is on the charge's
// reference, so the entry count is the proof that every writer posts the same key.

// The buyer's browser comes back first: its credit, its retry, the webhook twice, then a
// sweep of the row that browser closed.
func TestLiveStripe_InSessionThenRetryThenWebhookThenSweepIsOneEntry(t *testing.T) {
	l := liveSetup(t)
	id, charge := l.paid(2500)
	l.register(id, pendingintent.Topup, 2500)

	if _, f := l.topup(id, 2500, "usd", "parity-a"); f != nil {
		t.Fatalf("the in-session finish: %d %s", f.Status, f.Message)
	}
	if _, f := l.topup(id, 2500, "usd", "parity-a"); f != nil { // the buyer's retry replays the sealed answer
		t.Fatalf("the retry: %d %s", f.Status, f.Message)
	}
	body := l.event("payment_intent.succeeded", func(o map[string]any) bool { return str(o["id"]) == id })
	for i := 0; i < 2; i++ {
		if status, _ := l.deliver(body, l.hook); status != 200 {
			t.Fatalf("webhook delivery %d answered %d", i, status)
		}
	}
	reconcileIntents(*l.ctx)

	l.oneEntry(charge)
}

// The buyer never comes back: the reconciler posts first (the row aged past its grace),
// then the webhook twice, then the late in-session finish.
func TestLiveStripe_SweepThenWebhookThenLateBrowserIsOneEntry(t *testing.T) {
	l := liveSetup(t)
	id, charge := l.paid(2500)
	l.register(id, pendingintent.Topup, 2500)

	p, _, _ := pendingintent.Get(id)
	p.CreatedAt = p.CreatedAt.Add(-2 * intentGrace)
	if err := reconcileIntent(*l.ctx, p); err != nil {
		t.Fatalf("the sweep: %v", err)
	}
	if len(l.ledger.credits()) != 1 {
		t.Fatalf("the sweep posted %d credits, want 1: it must really post here", len(l.ledger.credits()))
	}
	body := l.event("payment_intent.succeeded", func(o map[string]any) bool { return str(o["id"]) == id })
	for i := 0; i < 2; i++ {
		if status, _ := l.deliver(body, l.hook); status != 200 {
			t.Fatalf("webhook delivery %d answered %d", i, status)
		}
	}
	if _, f := l.topup(id, 2500, "usd", "parity-b"); f != nil {
		t.Fatalf("the late in-session finish: %d %s", f.Status, f.Message)
	}

	l.oneEntry(charge)
}

// oneEntry asserts the ledger holds exactly one credit for the payment, under the
// charge's reference, for the buyer, in cash.
func (l *liveStripe) oneEntry(charge string) {
	l.t.Helper()
	got := l.ledger.credits()
	if len(got) != 1 {
		l.t.Fatalf("the ledger holds %d entries for one payment, want exactly 1: %+v", len(got), got)
	}
	if g := got[0]; g.IdempotencyKey != charge || g.AmountCents != 2500 || !g.Cash {
		l.t.Fatalf("the entry = %+v; want 2500 cash under %s", g, charge)
	}
	if l.balance() != 2500 {
		l.t.Fatalf("balance %d, want 2500", l.balance())
	}
}

// Stripe cannot exclude Apple Pay or Google Pay by method type on an intent, but it takes
// a Payment Method Configuration per intent. Commerce keeps its own, with the wallets as
// the admin set them, finds it again by name, and the intent carries its id.
func TestLiveStripe_TheWalletSwitchIsEnforcedByACommerceOwnedConfiguration(t *testing.T) {
	l := liveSetup(t)
	sp, err := processorsForOrg(l.org).Existing(*l.ctx, processor.Stripe)
	if err != nil {
		t.Fatal(err)
	}
	wc, ok := sp.(processor.WalletConfigurer)
	if !ok {
		t.Fatal("the Stripe provider keeps no wallet configuration")
	}
	id, err := wc.WalletConfiguration(*l.ctx, false, false)
	if err != nil || !strings.HasPrefix(id, "pmc_") {
		t.Fatalf("wallet configuration: %q %v", id, err)
	}
	if again, _ := wc.WalletConfiguration(*l.ctx, false, false); again != id {
		t.Fatalf("the configuration was made twice: %s then %s", id, again)
	}
	cfg := l.call("GET", "/payment_method_configurations/"+id, nil)
	for _, w := range []string{"apple_pay", "google_pay"} {
		pref := cfg[w].(map[string]any)["display_preference"].(map[string]any)["value"]
		if pref != "off" {
			t.Fatalf("%s is %v in the configuration; want off", w, pref)
		}
	}
	if cfg["card"].(map[string]any)["display_preference"].(map[string]any)["value"] != "on" {
		t.Fatal("cards must stay on")
	}
	maker := sp.(processor.LocalIntentMaker)
	out, err := maker.CreateLocalIntent(*l.ctx, processor.LocalIntent{Amount: 1000, Currency: "eur", Configuration: id, Excluded: []string{"ideal"}, Key: "wallet-" + l.run})
	if err != nil {
		t.Fatalf("an intent with the configuration and an exclusion: %v", err)
	}
	pi := l.call("GET", "/payment_intents/"+out.ID, nil)
	det, _ := pi["payment_method_configuration_details"].(map[string]any)
	if det["id"] != id {
		t.Fatalf("the intent does not carry the configuration: %v", det)
	}
	l.call("POST", "/payment_intents/"+out.ID+"/cancel", nil)
}

// The buyer authorizes a disclosed quote, the charge at confirm is exactly that amount, and
// a policy change after the intent is opened moves nothing already opened.
func TestLiveStripe_TheDisclosedQuoteIsTheChargeAndALaterPolicyMovesNothing(t *testing.T) {
	l := liveSetup(t)
	old := fx.Rate
	fx.Rate = func(_ context.Context, cur string) (float64, error) {
		if cur == "brl" || cur == "usd" {
			return 5.5, nil
		}
		return 0, errors.New("no rate source for " + cur)
	}
	t.Cleanup(func() { fx.Rate = old })

	pol := routing.Default()
	pol.FXSpreadBps = 300
	if _, f := WriteRouting(*l.ctx, l.org.Name, pol, "z", "spread 300"); f != nil {
		t.Fatalf("policy: %v", f.Err)
	}
	// Not viable: Pix in dollars is refused with a clear 400 before anything is opened.
	if _, f := OpenTopupIntent(*l.ctx, l.org, TopupIntentIn{Subject: l.subject, Country: "BR", AmountCents: 500, Currency: "usd", Method: "pix"}); f == nil || f.Status != 400 {
		t.Fatalf("pix in usd: %v", f)
	}
	// A German buyer may choose USD: the country table is a default and no restriction.
	usd, f := OpenTopupIntent(*l.ctx, l.org, TopupIntentIn{Subject: l.subject, Country: "DE", AmountCents: 500, Currency: "usd", IdempotencyKey: "q-usd-" + l.run})
	if f != nil || usd.Quote.Currency != "usd" || usd.Quote.Amount != 500 || usd.Quote.MarkupBps != 0 {
		t.Fatalf("a DE buyer choosing usd: %+v %v", usd, f)
	}
	l.call("POST", "/payment_intents/"+usd.IntentID+"/cancel", nil)

	out, f := OpenTopupIntent(*l.ctx, l.org, TopupIntentIn{Subject: l.subject, Country: "BR", AmountCents: 500, IdempotencyKey: "q-brl-" + l.run})
	if f != nil {
		t.Fatalf("open: %v", f.Err)
	}
	want := int64(math.Ceil(500*5.5*1.03 - 1e-9))
	q := out.Quote
	if q.Currency != "brl" || q.Amount != want || q.UsdCents != 500 || q.MarkupBps != 300 || out.PolicyVersion != 1 || out.Amount != q.Amount {
		t.Fatalf("quote %+v version %d; want %d brl at 300 bps under version 1", q, out.PolicyVersion, want)
	}
	pi := l.call("GET", "/payment_intents/"+out.IntentID, nil)
	if int64(pi["amount"].(float64)) != q.Amount || pi["currency"] != q.Currency {
		t.Fatalf("Stripe will charge %v %v; the buyer was shown %d %s", pi["amount"], pi["currency"], q.Amount, q.Currency)
	}
	rec, found, _ := pendingintent.Get(out.IntentID)
	if !found || rec.PolicyVersion != 1 || rec.PaidCents != q.Amount {
		t.Fatalf("the waiting record does not name the policy version and the quote: %+v", rec)
	}

	// The policy changes mid-checkout: another spread, and Brazil in dollars.
	next := routing.Default()
	next.FXSpreadBps = 100
	next.LocalCurrencies = []routing.LocalCurrencyRule{{Country: "BR", Currency: "usd"}}
	if _, f := WriteRouting(*l.ctx, l.org.Name, next, "z", "mid-checkout"); f != nil {
		t.Fatalf("second policy: %v", f.Err)
	}
	rec2, _, _ := pendingintent.Get(out.IntentID)
	pi2 := l.call("GET", "/payment_intents/"+out.IntentID, nil)
	if rec2.PaidCents != q.Amount || rec2.PaidCurrency != "brl" || rec2.PolicyVersion != 1 || int64(pi2["amount"].(float64)) != q.Amount || pi2["currency"] != "brl" {
		t.Fatalf("a policy change moved an intent that was already open: %+v %v %v", rec2, pi2["amount"], pi2["currency"])
	}
	// A NEW checkout takes the new policy.
	again, f := OpenTopupIntent(*l.ctx, l.org, TopupIntentIn{Subject: l.subject, Country: "BR", AmountCents: 500, IdempotencyKey: "q-new-" + l.run})
	if f != nil || again.PolicyVersion != 2 || again.Quote.Currency != "usd" {
		t.Fatalf("a new checkout under version 2: %+v %v", again, f)
	}
	l.call("POST", "/payment_intents/"+out.IntentID+"/cancel", nil)
	l.call("POST", "/payment_intents/"+again.IntentID+"/cancel", nil)
}
