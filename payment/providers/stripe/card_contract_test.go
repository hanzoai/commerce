package stripe_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/providers/stripe"
)

// saved attaches a test card to a new customer, as a plan sale does.
func saved(t *testing.T, p *stripe.Provider, of string) (customer string, card processor.Card) {
	t.Helper()
	cus, err := p.CreateCustomer(context.Background(), "alice@example.com", "hanzo/alice", map[string]interface{}{"reference": "hanzo/hanzo/alice"})
	if err != nil {
		t.Fatalf("CreateCustomer: %v", err)
	}
	c, err := p.Vault(context.Background(), cus, of)
	if err != nil {
		t.Fatalf("Vault(%s): %v", of, err)
	}
	return cus, c
}

// A browser-made PaymentMethod is attached to the buyer's customer and
// reported as the card it is; a second vault of the same method is the same
// card, and one belonging to another customer is refused.
func TestContract_VaultAttachesOnce(t *testing.T) {
	p, srv := contractProvider(t)
	cus, card := saved(t, p, "pm_card_visa")
	if card.Last4 != "4242" || card.Brand != "visa" || card.Fingerprint == "" || card.CustomerID != cus {
		t.Fatalf("card = %+v", card)
	}
	if m := srv.Method(card.ID); m == nil || m.Customer != cus {
		t.Fatalf("Stripe holds %+v; want attached to %s", m, cus)
	}
	again, err := p.Vault(context.Background(), cus, card.ID)
	if err != nil || again.ID != card.ID {
		t.Fatalf("second vault = %+v, %v", again, err)
	}
	other, _ := p.CreateCustomer(context.Background(), "", "hanzo/bob", map[string]interface{}{"reference": "hanzo/hanzo/bob"})
	if _, err := p.Vault(context.Background(), other, card.ID); err == nil {
		t.Fatal("a card saved to alice was vaulted for bob")
	}
	if _, err := p.Vault(context.Background(), cus, "tok_visa"); err == nil {
		t.Fatal("a token that is not a PaymentMethod was vaulted")
	}
}

// One customer per reference: a retried create under the same reference is
// the customer already made.
func TestContract_CustomerIsMadeOncePerReference(t *testing.T) {
	p, srv := contractProvider(t)
	meta := map[string]interface{}{"reference": "hanzo/hanzo/alice"}
	a, err := p.CreateCustomer(context.Background(), "a@x", "alice", meta)
	b, err2 := p.CreateCustomer(context.Background(), "a@x", "alice", meta)
	if err != nil || err2 != nil || a != b || srv.Customers() != 1 {
		t.Fatalf("customers %s %s (%v %v), %d held", a, b, err, err2, srv.Customers())
	}
}

// With the buyer present, a card whose bank asks for 3-D Secure answers
// ActionRequired with the intent's client secret; once the buyer has
// authenticated, the same order naming the intent reads it settled — and the
// same intent named by another order is refused.
func TestContract_ThreeDSecureIsFinishedByTheBuyer(t *testing.T) {
	p, srv := contractProvider(t)
	cus, card := saved(t, p, "pm_card_authenticationRequired")
	req := processor.PaymentRequest{
		Amount: 2000, Currency: currency.USD, Token: card.ID, CustomerID: cus,
		IdempotencyKey: "subscribe:hanzo/alice:k1", Options: map[string]interface{}{"save": true},
	}
	res, err := p.Charge(context.Background(), req)
	var act *processor.ActionRequired
	if !errors.As(err, &act) || act.ClientSecret == "" || act.Intent != res.TransactionID || res.Success {
		t.Fatalf("Charge = %+v, %v; want ActionRequired", res, err)
	}
	if in := srv.Intent(act.Intent); in.Params.Get("setup_future_usage") != "off_session" {
		t.Fatalf("a saved card's first payment did not set it up for off-session use: %v", in.Params)
	}

	resume := req
	resume.Options = map[string]interface{}{"payment_intent": act.Intent}
	if _, err := p.Charge(context.Background(), resume); !errors.As(err, &act) {
		t.Fatalf("an intent not yet authenticated read %v; want ActionRequired again", err)
	}
	srv.Authenticate(act.Intent)
	got, err := p.Charge(context.Background(), resume)
	if err != nil || !got.Success || got.ProcessorRef == "" {
		t.Fatalf("resume = %+v, %v; want settled", got, err)
	}
	if srv.Intents() != 1 {
		t.Fatalf("Stripe holds %d PaymentIntents; want the one", srv.Intents())
	}

	stolen := resume
	stolen.IdempotencyKey = "subscribe:hanzo/mallory:k9"
	if r, err := p.Charge(context.Background(), stolen); err == nil || r.Success {
		t.Fatalf("another order's intent was accepted: %+v", r)
	}
	bigger := resume
	bigger.Amount = 9000
	if r, err := p.Charge(context.Background(), bigger); err == nil || r.Success {
		t.Fatalf("an intent for a different amount was accepted: %+v", r)
	}
}

// Off session, nobody can authenticate: the same card is a decline, never an
// ActionRequired a sweep would have nobody to hand to.
func TestContract_OffSessionAuthenticationIsADecline(t *testing.T) {
	p, _ := contractProvider(t)
	cus, card := saved(t, p, "pm_card_authenticationRequired")
	res, err := p.Charge(context.Background(), processor.PaymentRequest{
		Amount: 2000, Currency: currency.USD, Token: card.ID, CustomerID: cus,
		IdempotencyKey: "collect:sub:1", Options: map[string]interface{}{"off_session": true},
	})
	var act *processor.ActionRequired
	if err == nil || res.Success || errors.As(err, &act) {
		t.Fatalf("off-session charge = %+v, %v; want a decline", res, err)
	}
}

// A renewal charges the saved card off session and settles.
func TestContract_RenewalChargesTheSavedCard(t *testing.T) {
	p, srv := contractProvider(t)
	cus, card := saved(t, p, "pm_card_visa")
	res, err := p.Charge(context.Background(), processor.PaymentRequest{
		Amount: 2000, Currency: currency.USD, Token: card.ID, CustomerID: cus,
		IdempotencyKey: "collect:sub:1:period:1:attempt:0", Options: map[string]interface{}{"off_session": true},
	})
	if err != nil || !res.Success {
		t.Fatalf("renewal = %+v, %v", res, err)
	}
	if in := srv.Intent(res.TransactionID); in.Params.Get("off_session") != "true" || in.Params.Get("customer") != cus {
		t.Fatalf("renewal sent %v", in.Params)
	}
}

// A card saved without a payment is set up for off-session use; a bank that
// asks for authentication is finished by the buyer and read back by its id.
func TestContract_SetUpWithAuthentication(t *testing.T) {
	p, srv := contractProvider(t)
	cus, card := saved(t, p, "pm_card_visa")
	if err := p.SetUp(context.Background(), cus, card.ID, "card:k1", ""); err != nil {
		t.Fatalf("SetUp(visa) = %v", err)
	}
	cus2, card2 := saved(t, p, "pm_card_authenticationRequired")
	err := p.SetUp(context.Background(), cus2, card2.ID, "card:k2", "")
	var act *processor.ActionRequired
	if !errors.As(err, &act) || !act.Setup() {
		t.Fatalf("SetUp(3ds) = %v; want ActionRequired on a setup", err)
	}
	if err := p.SetUp(context.Background(), cus2, card2.ID, "card:other", act.Intent); err == nil {
		t.Fatal("a setup named by another request was accepted")
	}
	srv.Authenticate(act.Intent)
	if err := p.SetUp(context.Background(), cus2, card2.ID, "card:k2", act.Intent); err != nil {
		t.Fatalf("SetUp resume = %v", err)
	}
}

func signed(secret string, at time.Time, body []byte, extra ...string) string {
	ts := strconv.FormatInt(at.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "." + string(body)))
	h := fmt.Sprintf("t=%s,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
	for _, e := range extra {
		h += ",v1=" + e
	}
	return h
}

// Webhooks: Stripe's own type and livemode reach the receiver; any v1 of a
// rolled secret verifies; a delivery signed in the future is refused like one
// from the past.
func TestWebhook_Stripe(t *testing.T) {
	p := stripe.NewProvider(stripe.Config{SecretKey: "sk_test_x", WebhookSecret: "whsec_x"})
	if !p.ReceivesWebhooks() {
		t.Fatal("a provider holding its signing secret does not receive webhooks")
	}
	body := []byte(`{"id":"evt_1","type":"charge.dispute.created","livemode":false,"created":1700000000,"data":{"object":{"id":"dp_1","charge":"ch_1"}}}`)
	evt, err := p.ValidateWebhook(context.Background(), body, signed("whsec_x", time.Now(), body, "deadbeef"))
	if err != nil || evt.Type != "charge.dispute.created" || evt.Data["stripe_livemode"] != false || evt.Data["stripe_created"] != int64(1700000000) {
		t.Fatalf("event = %+v, %v", evt, err)
	}
	rolled := "t=" + strconv.FormatInt(time.Now().Unix(), 10) + ",v1=deadbeef"
	mac := hmac.New(sha256.New, []byte("whsec_x"))
	mac.Write([]byte(strconv.FormatInt(time.Now().Unix(), 10) + "." + string(body)))
	if _, err := p.ValidateWebhook(context.Background(), body, rolled+",v1="+hex.EncodeToString(mac.Sum(nil))); err != nil {
		t.Fatalf("a header whose second v1 verifies was refused: %v", err)
	}
	if _, err := p.ValidateWebhook(context.Background(), body, signed("whsec_x", time.Now().Add(10*time.Minute), body)); err == nil {
		t.Fatal("a delivery signed ten minutes from now was accepted")
	}
	if stripe.NewProvider(stripe.Config{SecretKey: "sk_test_x"}).ReceivesWebhooks() {
		t.Fatal("a provider with no signing secret receives webhooks")
	}
}
