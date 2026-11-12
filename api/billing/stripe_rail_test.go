package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/providers/stripe"
	"github.com/hanzoai/commerce/payment/providers/stripe/stripetest"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The card rail at Stripe, end to end through the real Stripe provider over
// HTTP against Stripe's contract (stripetest): a sale vaults the browser's
// PaymentMethod and charges it with setup_future_usage, 3-D Secure is finished
// by the buyer and read back once, renewals charge off session, Square
// subscribers keep renewing on Square, and every other rail refuses new money.

const railKey = "sk_test_rail"

// onStripeRail puts the card rail at Stripe and the org's registry on the real
// Stripe provider (and sq, when given, as the Square a subscriber already has).
func onStripeRail(t *testing.T, sq *mockSquareProcessor) *stripetest.Server {
	t.Helper()
	srv := stripetest.New(t, railKey)
	srv.Route(t)
	SetRail(func(context.Context) string { return "stripe" })
	t.Cleanup(func() { SetRail(nil) })
	old := processorsForOrg
	processorsForOrg = func(*organization.Organization) *processor.Registry {
		reg := processor.NewRegistry(processor.DefaultConfig())
		reg.Register(stripe.NewProvider(stripe.Config{SecretKey: railKey, PublishableKey: "pk_test_rail"}))
		if sq != nil {
			reg.Register(sq)
		}
		return reg
	}
	t.Cleanup(func() { processorsForOrg = old })
	return srv
}

func body(t *testing.T, r *http.Response) string {
	t.Helper()
	b, _ := io.ReadAll(r.Body)
	return string(b)
}

func TestRail_DefaultIsSquareAndTheHostDecides(t *testing.T) {
	t.Setenv("COMMERCE_CARD_RAIL", "")
	if Rail(context.Background()) != processor.Square {
		t.Fatal("an unset rail is not square")
	}
	t.Setenv("COMMERCE_CARD_RAIL", "Stripe")
	if Rail(context.Background()) != processor.Stripe {
		t.Fatal("COMMERCE_CARD_RAIL=Stripe did not put the rail at stripe")
	}
	SetRail(func(context.Context) string { return "square" })
	defer SetRail(nil)
	if Rail(context.Background()) != processor.Square {
		t.Fatal("the host's reader did not outrank the environment")
	}
	SetRail(func(context.Context) string { return "bitcoin" })
	if Rail(context.Background()) != processor.Square {
		t.Fatal("an unknown rail is not square")
	}
}

// A sale on the Stripe rail vaults the browser's PaymentMethod at the buyer's
// Stripe customer, charges THAT card once with setup_future_usage=off_session,
// and records a Stripe-backed subscription whose paid invoice names the charge.
func TestStripeRail_SaleVaultsAndCharges(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("srail")
	srv := onStripeRail(t, nil)

	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"pm_card_visa","provider":"stripe","planId":"go"}`, map[string]string{"X-Idempotency-Key": "sale-1"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", resp.StatusCode, body(t, resp))
	}
	if srv.Intents() != 1 || srv.Customers() != 1 {
		t.Fatalf("Stripe holds %d intents, %d customers; want 1 and 1", srv.Intents(), srv.Customers())
	}
	db := datastore.New(org.Namespaced(ctx))
	pms := pmsFor(t, db, "srail")
	if len(pms) != 1 || pms[0].ProviderType != "stripe" || customerIDOf(pms[0]) == "" || !strings.HasPrefix(pms[0].ProviderRef, "pm_") {
		t.Fatalf("saved cards = %+v", pms)
	}
	in := srv.Intent("pi_contract000003")
	if in == nil {
		for id := 1; id < 10 && in == nil; id++ {
			in = srv.Intent(fmt.Sprintf("pi_contract%06d", id))
		}
	}
	if in == nil || in.Params.Get("setup_future_usage") != "off_session" || in.Params.Get("customer") != customerIDOf(pms[0]) ||
		in.PaymentMethod != pms[0].ProviderRef || in.Amount != lookupPlan("go").Price {
		t.Fatalf("the charge was %+v", in)
	}
	sub := parentSub(t, db, "srail", "go")
	if sub == nil || sub.ProviderType != "stripe" || sub.DefaultPaymentMethod != pms[0].Id() {
		t.Fatalf("subscription = %+v", sub)
	}
	invs := invoicesForSub(t, db, sub.Id())
	if len(invs) != 1 || invs[0].Status != billinginvoice.Paid || !strings.HasPrefix(invs[0].PaymentRef, "ch_") {
		t.Fatalf("first invoice = %+v", invs)
	}

	// The renewal charges the same card, off session, and pays the period: a
	// subscription on the saved card whose period has run out.
	p, err := resolveSubscriptionPlan(db, "dev")
	if err != nil {
		t.Fatal(err)
	}
	due := subscription.New(db)
	due.UserId = "srail"
	due.DefaultPaymentMethod = pms[0].Id()
	due.ProviderType = "stripe"
	due.Quantity = 1
	engine.StartSubscription(due, p)
	due.PeriodStart = time.Now().AddDate(0, -2, 0)
	due.PeriodEnd = time.Now().AddDate(0, -1, 0)
	if err := due.Create(); err != nil {
		t.Fatal(err)
	}
	r := invokeRenew(org, ctx, due.Id())
	if r.StatusCode != http.StatusOK {
		t.Fatalf("renew status=%d body=%s", r.StatusCode, body(t, r))
	}
	if srv.Intents() != 2 {
		t.Fatalf("Stripe holds %d intents after the renewal; want 2", srv.Intents())
	}
	renewal := srv.Intent("pi_contract" + fmt.Sprintf("%06d", lastSeq(srv)))
	if renewal == nil || renewal.Params.Get("off_session") != "true" || renewal.PaymentMethod != pms[0].ProviderRef {
		t.Fatalf("renewal charge = %+v", renewal)
	}
}

// lastCharge is the Charge (ch_…) of the newest settled PaymentIntent.
func lastCharge(srv *stripetest.Server) string {
	return "ch_contract" + fmt.Sprintf("%06d", lastSeq(srv))
}

// lastSeq is the sequence number of the newest PaymentIntent stripetest holds.
func lastSeq(srv *stripetest.Server) int {
	last := 0
	for i := 1; i < 100; i++ {
		if srv.Intent(fmt.Sprintf("pi_contract%06d", i)) != nil {
			last = i
		}
	}
	return last
}

// 3-D Secure on a sale: the first answer is 402 requires_action with the
// intent and its client secret and nothing is bought; once the buyer has
// authenticated, the same order with the same key naming the intent completes
// it — one PaymentIntent, one card, one subscription.
func TestStripeRail_SaleThreeDSecureIsFinishedByTheBuyer(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("s3ds")
	srv := onStripeRail(t, nil)
	key := map[string]string{"X-Idempotency-Key": "sale-3ds"}
	order := `{"sourceId":"pm_card_authenticationRequired","provider":"stripe","planId":"go"`

	resp := invokeSubscribeCard(org, ctx, order+`}`, key)
	if resp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("status=%d body=%s, want 402", resp.StatusCode, body(t, resp))
	}
	out := jsonBody(t, resp)
	pi, _ := out["paymentIntent"].(string)
	if out["code"] != "requires_action" || out["clientSecret"] == "" || !strings.HasPrefix(pi, "pi_") {
		t.Fatalf("402 = %+v", out)
	}
	db := datastore.New(org.Namespaced(ctx))
	if parentSub(t, db, "s3ds", "go") != nil {
		t.Fatal("a subscription opened before the buyer authenticated")
	}

	// Not yet authenticated: still waiting, still nothing bought.
	resp = invokeSubscribeCard(org, ctx, order+`,"paymentIntent":"`+pi+`"}`, key)
	if resp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("resume before authentication = %d %s", resp.StatusCode, body(t, resp))
	}

	srv.Authenticate(pi)
	resp = invokeSubscribeCard(org, ctx, order+`,"paymentIntent":"`+pi+`"}`, key)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("resume = %d %s, want 201", resp.StatusCode, body(t, resp))
	}
	if srv.Intents() != 1 {
		t.Fatalf("Stripe holds %d intents; want the one the buyer authenticated", srv.Intents())
	}
	if pms := pmsFor(t, db, "s3ds"); len(pms) != 1 {
		t.Fatalf("saved cards = %d, want 1", len(pms))
	}
	if sub := parentSub(t, db, "s3ds", "go"); sub == nil || sub.ProviderType != "stripe" {
		t.Fatalf("subscription = %+v", sub)
	}
}

// A declined card buys nothing and leaves no card behind.
func TestStripeRail_DeclineLeavesNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("sdecl")
	onStripeRail(t, nil)
	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"pm_card_chargeDeclined","provider":"stripe","planId":"go"}`, nil)
	if resp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("status=%d body=%s, want 402", resp.StatusCode, body(t, resp))
	}
	db := datastore.New(org.Namespaced(ctx))
	if len(pmsFor(t, db, "sdecl")) != 0 || parentSub(t, db, "sdecl", "go") != nil {
		t.Fatal("a declined sale left a card or a subscription")
	}
}

// THE MIGRATION: with the rail moved to Stripe, a Square subscriber's renewal
// still charges their Square card and Stripe sees nothing — but the same Square
// card buys nothing new: it has to be added again, at Stripe.
func TestStripeRail_SquareSubscribersKeepRenewing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("sqold")
	m := squareMock("", "", "sqpay_old")
	srv := onStripeRail(t, m)
	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, "sqold", "dev", "ccof_old", "cust_old")

	if r := invokeRenew(org, ctx, sub.Id()); r.StatusCode != http.StatusOK {
		t.Fatalf("renew status=%d body=%s", r.StatusCode, body(t, r))
	}
	if m.chargeCalls != 1 || m.lastChargeToken != "ccof_old" || srv.Intents() != 0 {
		t.Fatalf("square charges=%d token=%q stripe intents=%d; want the renewal on Square alone", m.chargeCalls, m.lastChargeToken, srv.Intents())
	}

	pm := pmsFor(t, db, "sqold")[0]
	resp := invokeSubscribeCard(org, ctx, `{"paymentMethodId":"`+pm.Id()+`","planId":"go"}`, nil)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("a new sale on the Square card = %d %s, want 422", resp.StatusCode, body(t, resp))
	}
	if m.chargeCalls != 1 {
		t.Fatal("the Square card was charged for something new")
	}
	if _, err := TopupCard(ctx, org, TopupCardIn{MethodID: pm.Id(), AmountCents: 2500, Subject: "sqold"}); !IsRailMoved(err) || RailCode(err) != "card_readd" {
		t.Fatalf("a top-up on the Square card = %v; want card_readd", err)
	}
}

// Every other rail opens no new payment while Stripe is the rail.
func TestStripeRail_OtherRailsRefuseNewPayments(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("srefuse")
	m := squareMock("cust", "ccof", "sqpay")
	onStripeRail(t, m)

	for _, b := range []string{
		`{"sourceId":"cnon:ok","provider":"square","planId":"go"}`,
		`{"sourceId":"PP-ORDER-1","provider":"paypal","planId":"go"}`,
		`{"sourceId":"wnon:ok","wallet":true,"planId":"go"}`,
	} {
		if r := invokeSubscribeCard(org, ctx, b, nil); r.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s = %d %s, want 400", b, r.StatusCode, body(t, r))
		}
	}
	if _, f := TakePayment(ctx, org, TakePaymentIn{SourceID: "cnon:ok", AmountCents: 2500, Subject: "srefuse", Provider: "square", IdempotencyKey: "t1"}); f == nil || f.Status != 400 || f.Code != "rail_moved" {
		t.Fatalf("a Square top-up = %+v; want 400 rail_moved", f)
	}
	if _, f := CreatePayPalOrder(ctx, org, PayPalOrderIn{Subject: "srefuse"}); f == nil || f.Code != "rail_moved" {
		t.Fatalf("a PayPal order = %+v; want rail_moved", f)
	}
	if _, err := CreateCryptoDeposit(ctx, org, "srefuse", "base", "usdc", 2500); !IsRailMoved(err) {
		t.Fatalf("a crypto deposit = %v; want rail_moved", err)
	}
	if m.chargeCalls != 0 || m.createCustomerCalls != 0 {
		t.Fatal("Square was asked to take a new payment")
	}
	t.Setenv("STRIPE_SECRET_KEY", "sk_live_rail")
	t.Setenv("STRIPE_PUBLISHABLE_KEY", "pk_live_rail")
	if cfg := ReadPaymentConfig(ctx, org); cfg.Stripe.PublishableKey != "pk_live_rail" || !cfg.Stripe.Enabled ||
		cfg.Rail != "stripe" || cfg.Provider != "stripe" || cfg.Cards["usd"] != "stripe" || cfg.PayPal.Enabled {
		t.Fatalf("payment config = %+v", cfg)
	}
}

// A token top-up on the Stripe rail: 3-D Secure answers the action, and the
// same top-up naming the authenticated intent credits once.
func TestStripeRail_TopupThreeDSecure(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("stopup")
	srv := onStripeRail(t, nil)
	injectLedger(t, newFakeLedger())
	in := TakePaymentIn{SourceID: "pm_card_authenticationRequired", AmountCents: 2500, Subject: "stopup", IdempotencyKey: "top-3ds"}
	_, f := TakePayment(ctx, org, in)
	if f == nil || f.Status != 402 || f.Code != "requires_action" || f.Action == nil || f.Action.ClientSecret == "" {
		t.Fatalf("first top-up = %+v; want 402 requires_action", f)
	}
	srv.Authenticate(f.Action.Intent)
	in.PaymentIntent = f.Action.Intent
	out, f := TakePayment(ctx, org, in)
	if f != nil || out.Status != "ok" || !strings.HasPrefix(out.ProcessorRef, "ch_") {
		t.Fatalf("resumed top-up = %+v %+v", out, f)
	}
	again, f := TakePayment(ctx, org, in)
	if f != nil || again.TransactionID != out.TransactionID || srv.Intents() != 1 {
		t.Fatalf("a retry credited again: %+v %+v (%d intents)", again, f, srv.Intents())
	}
}

// stripeDelivery posts a Stripe-signed event to the webhook receiver.
func stripeDelivery(t *testing.T, ctx context.Context, secret, raw string) *http.Response {
	t.Helper()
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "." + raw))
	req := httptest.NewRequest(http.MethodPost, "/v1/commerce/webhooks/stripe", strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", "t="+ts+",v1="+hex.EncodeToString(mac.Sum(nil)))
	return driveSeeded(func(c *zip.Ctx) { c.SetContext(ctx) }, "/v1/commerce/webhooks/:provider", req, HandleProviderWebhook)
}

// A Stripe refund and a chargeback of a Stripe top-up take the credit back,
// once each; a forged signature moves nothing; payment_intent.succeeded never
// credits a second time.
func TestStripeRail_WebhooksClawBack(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	db := datastore.New(ctx)
	org := organization.New(db)
	org.Name = "hanzo"
	org.Live = true
	org.MustCreate()
	srv := onStripeRail(t, nil)

	const secret = "whsec_rail"
	prev, _ := processor.Global().Registered(processor.Stripe)
	processor.Global().Register(stripe.NewProvider(stripe.Config{SecretKey: railKey, WebhookSecret: secret}))
	t.Cleanup(func() {
		if prev != nil {
			processor.Global().Register(prev)
		}
	})

	// A card saved at Stripe (set up for off-session use), then a top-up on it:
	// the payment the refund and the dispute below name.
	m, _, err := CreateMethod(ctx, org, "alice@example.com", CreateMethodIn{CustomerId: "hanzo/alice", Type: "card", ProviderRef: "pm_card_visa", ProviderType: "stripe"})
	if err != nil {
		t.Fatalf("save card: %v", err)
	}
	if setups := srv.Setups(); setups != 1 {
		t.Fatalf("Stripe holds %d SetupIntents; a card saved without a payment is set up once", setups)
	}
	if _, err := TopupCard(ctx, org, TopupCardIn{MethodID: m.Id, AmountCents: 2500, Subject: "hanzo/alice", IdempotencyKey: "wh-1"}); err != nil {
		t.Fatalf("top-up: %v", err)
	}
	charge := lastCharge(srv)
	test := org.TestMode()
	held := func() currency.Cents { return balanceOf(t, ctx, org, "hanzo/alice") }
	if held() != 2500 {
		t.Fatalf("balance after the top-up = %d, want 2500 (test=%v)", held(), test)
	}

	evt := func(id, typ string, obj map[string]any) string {
		b, _ := json.Marshal(map[string]any{"id": id, "type": typ, "livemode": true, "created": time.Now().Unix(), "data": map[string]any{"object": obj}})
		return string(b)
	}
	if r := stripeDelivery(t, ctx, "whsec_forged", evt("evt_forged", "refund.updated", map[string]any{"id": "re_x", "charge": charge, "status": "succeeded", "amount": 2500, "currency": "usd"})); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("forged = %d", r.StatusCode)
	}
	if r := stripeDelivery(t, ctx, secret, evt("evt_pi", "payment_intent.succeeded", map[string]any{"id": "pi_x", "latest_charge": charge, "amount": 2500, "currency": "usd", "status": "succeeded"})); r.StatusCode != http.StatusOK {
		t.Fatalf("payment_intent.succeeded = %d %s", r.StatusCode, body(t, r))
	}
	if held() != 2500 {
		t.Fatalf("payment_intent.succeeded moved the balance to %d", held())
	}

	refund := evt("evt_re", "refund.updated", map[string]any{"id": "re_1", "charge": charge, "status": "succeeded", "amount": 1000, "currency": "usd"})
	for i := 0; i < 2; i++ {
		if r := stripeDelivery(t, ctx, secret, refund); r.StatusCode != http.StatusOK {
			t.Fatalf("refund delivery %d = %d %s", i, r.StatusCode, body(t, r))
		}
	}
	dispute := evt("evt_dp", "charge.dispute.created", map[string]any{"id": "dp_1", "charge": charge, "status": "needs_response", "amount": 1500, "currency": "usd"})
	if r := stripeDelivery(t, ctx, secret, dispute); r.StatusCode != http.StatusOK {
		t.Fatalf("dispute = %d %s", r.StatusCode, body(t, r))
	}
	if held() != 0 {
		t.Fatalf("balance after a 1000 refund (delivered twice) and a 1500 chargeback = %d, want 0", held())
	}
}
