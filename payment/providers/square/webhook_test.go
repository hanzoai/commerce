// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package square

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hanzoai/commerce/payment/processor"
	thirdparty "github.com/hanzoai/commerce/thirdparty/square"
)

const (
	testKey = "test-signature-key"
	testURL = "https://pay.example.test/v1/commerce/webhooks/square"
)

// sign is Square's documented signature: base64(HMAC-SHA256(key, url+body)).
func sign(key, url string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(url))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func event(id, typ string, created time.Time) []byte {
	return []byte(fmt.Sprintf(
		`{"merchant_id":"m","type":%q,"event_id":%q,"created_at":%q,"data":{"type":"payment","id":"pay_1","object":{"payment":{"id":"pay_1","status":"COMPLETED","amount_money":{"amount":1000,"currency":"USD"}}}}}`,
		typ, id, created.UTC().Format(time.RFC3339)))
}

func verifier() *Provider {
	return NewProvider(Config{WebhookSignatureKey: testKey, WebhookURL: testURL})
}

// The deployment's environment seeds the global verifier: env → configFromEnv →
// Configure → ValidateWebhook over a Square-spec signature.
func TestEnvSeedsTheVerifier(t *testing.T) {
	t.Setenv("SQUARE_ENVIRONMENT", "production")
	t.Setenv("SQUARE_ACCESS_TOKEN", "tok")
	t.Setenv("SQUARE_LOCATION_ID", "loc")
	t.Setenv("SQUARE_APPLICATION_ID", "app")
	t.Setenv("SQUARE_WEBHOOK_SIGNATURE_KEY", testKey)
	t.Setenv("SQUARE_WEBHOOK_URL", testURL)

	p := NewProvider(configFromEnv())
	if !p.IsAvailable(context.Background()) {
		t.Fatal("env-seeded provider not available")
	}
	body := event("evt_env", "payment.updated", time.Now())
	evt, err := p.ValidateWebhook(context.Background(), body, sign(testKey, testURL, body))
	if err != nil || evt == nil || evt.ID != "evt_env" {
		t.Fatalf("valid signature refused: evt=%+v err=%v", evt, err)
	}
}

// A retry carries the event's ORIGINAL created_at, because Square resends the same
// body. A delivery first sent while the receiver was down is retried minutes to
// days later, and it must verify then: refusing it by age is how a deploy turned
// into a permanently lost payment event.
func TestARetryVerifiesWhateverItsAge(t *testing.T) {
	p := verifier()
	for _, age := range []time.Duration{6 * time.Minute, 3 * time.Hour, 71 * time.Hour, 30 * 24 * time.Hour} {
		body := event("evt_retry", "payment.updated", time.Now().Add(-age))
		if _, err := p.ValidateWebhook(context.Background(), body, sign(testKey, testURL, body)); err != nil {
			t.Errorf("a %s-old retry was refused: %v", age, err)
		}
	}
}

// Verifying needs the key and the URL, not charge credentials.
func TestVerifiesWithoutChargeCredentials(t *testing.T) {
	p := verifier()
	if p.IsAvailable(context.Background()) {
		t.Fatal("a provider with no access token must not charge")
	}
	body := event("evt_nocreds", "refund.updated", time.Now())
	if _, err := p.ValidateWebhook(context.Background(), body, sign(testKey, testURL, body)); err != nil {
		t.Fatalf("refused with key and URL configured: %v", err)
	}
}

func TestRefusesWhatSquareDidNotSign(t *testing.T) {
	p := verifier()
	body := event("evt_1", "payment.updated", time.Now())
	good := sign(testKey, testURL, body)
	cases := map[string]struct {
		body []byte
		sig  string
	}{
		"tampered body":     {append(append([]byte{}, body...), ' '), good},
		"another key":       {body, sign("other-key", testURL, body)},
		"another URL":       {body, sign(testKey, "https://api.example.test/v1/billing/webhooks/square", body)},
		"body-only HMAC":    {body, sign(testKey, "", body)},
		"not base64":        {body, "%%%"},
		"empty signature":   {body, ""},
		"signed, no evt id": {[]byte(`{"type":"payment.updated","data":{"object":{}}}`), sign(testKey, testURL, []byte(`{"type":"payment.updated","data":{"object":{}}}`))},
	}
	for name, c := range cases {
		if _, err := p.ValidateWebhook(context.Background(), c.body, c.sig); !errors.Is(err, processor.ErrWebhookValidationFailed) {
			t.Errorf("%s: err = %v, want ErrWebhookValidationFailed", name, err)
		}
	}
}

func TestRefusesWithoutKeyOrURL(t *testing.T) {
	body := event("evt_1", "payment.updated", time.Now())
	for name, cfg := range map[string]Config{
		"no key": {WebhookURL: testURL},
		"no URL": {WebhookSignatureKey: testKey},
	} {
		if _, err := NewProvider(cfg).ValidateWebhook(context.Background(), body, sign(testKey, testURL, body)); !errors.Is(err, processor.ErrWebhookValidationFailed) {
			t.Errorf("%s: err = %v, want ErrWebhookValidationFailed", name, err)
		}
	}
}

func TestNamesTheEventsTheReceiverBranchesOn(t *testing.T) {
	p := verifier()
	for sq, want := range map[string]string{
		"payment.updated":       "payment.updated",
		"invoice.payment_made":  "invoice.paid",
		"dispute.state.updated": "dispute.updated",
		"dispute.state.changed": "dispute.updated",
		"refund.created":        "refund.created",
	} {
		body := event("evt_"+sq, sq, time.Now())
		evt, err := p.ValidateWebhook(context.Background(), body, sign(testKey, testURL, body))
		if err != nil {
			t.Fatalf("%s: %v", sq, err)
		}
		if evt.Type != want || evt.Processor != processor.Square {
			t.Errorf("%s: type %q processor %q, want %q square", sq, evt.Type, evt.Processor, want)
		}
		if evt.Data["payment"] == nil {
			t.Errorf("%s: data.object not passed through: %+v", sq, evt.Data)
		}
	}
}

// One Square processor: the global slot is this package's Provider, and the API
// client underneath cannot be registered as a second one, because it is not a
// PaymentProcessor — it has no verifier to offer.
func TestTheRegistryHoldsOneSquareType(t *testing.T) {
	got, err := processor.Global().Registered(processor.Square)
	if err != nil {
		t.Fatalf("no Square in the global registry: %v", err)
	}
	if _, ok := got.(*Provider); !ok {
		t.Fatalf("the global Square slot holds %T, want *square.Provider", got)
	}
	var client any = thirdparty.NewProcessor(thirdparty.Config{AccessToken: "tok", LocationID: "loc"})
	if _, ok := client.(processor.PaymentProcessor); ok {
		t.Fatal("thirdparty/square is a PaymentProcessor again: a second Square verifier can be registered")
	}
}

func TestConfigFromEnvSandboxReadsNoProductionName(t *testing.T) {
	t.Setenv("SQUARE_ENVIRONMENT", "sandbox")
	t.Setenv("SQUARE_ACCESS_TOKEN", "EAAA-prod-hostile")
	t.Setenv("SQUARE_LOCATION_ID", "PRODLOC-hostile")
	t.Setenv("SQUARE_APPLICATION_ID", "sq0idp-hostile")
	t.Setenv("SQUARE_SANDBOX_ACCESS_TOKEN", "sb-tok")
	t.Setenv("SQUARE_SANDBOX_LOCATION_ID", "sb-loc")
	t.Setenv("SQUARE_SANDBOX_APPLICATION_ID", "")

	cfg := configFromEnv()
	if cfg.AccessToken != "sb-tok" || cfg.LocationID != "sb-loc" || cfg.Environment != "sandbox" {
		t.Fatalf("sandbox switch broken: %+v", cfg)
	}
	t.Setenv("SQUARE_SANDBOX_ACCESS_TOKEN", "")
	t.Setenv("SQUARE_SANDBOX_LOCATION_ID", "")
	if p := NewProvider(configFromEnv()); p.IsAvailable(context.Background()) {
		t.Fatal("a sandbox deployment with no sandbox account charges through the production names")
	}
}
