package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/thirdparty/mpc"
)

// mpcSigHeader is the header the MPC service sends its digest in.
const mpcSigHeader = "X-Webhook-Signature"

// mpcSign reproduces the MPC service's signing: hex of HMAC-SHA256 over the raw
// body, keyed by the webhook secret (lux/mpc pkg/api/webhook_sender.go).
func mpcSign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// mpcSettlementEvent is a delivery that mints credit if it validates: a
// completed payment naming beneficiary and amount ($1,000.00).
func mpcSettlementEvent(eventID string) []byte {
	return []byte(`{"id":"` + eventID + `","type":"payment.completed","timestamp":1,` +
		`"data":{"payment":{"id":"pay_` + eventID + `","status":"COMPLETED",` +
		`"reference_id":"victim-user","amount_money":{"amount":100000,"currency":"usd"}}}}`)
}

// registerMPC installs a fully configured MPC processor — the prod shape, with
// MPC_ENDPOINT and a webhook secret provisioned — restoring the previous one on
// cleanup.
func registerMPC(t *testing.T, secret string) {
	t.Helper()
	prev, _ := processor.Global().Get(processor.MPC)
	t.Cleanup(func() {
		if prev != nil {
			processor.Global().Register(prev)
		}
	})
	processor.Global().Register(mpc.NewProcessor(mpc.Config{
		KMSEndpoint:   "https://kms.hanzo.ai",
		MPCEndpoint:   "https://mpc.hanzo.ai",
		APIKey:        "api-key",
		WebhookSecret: secret,
	}))
}

// TestMPCWebhook_ForgedSignatureRejected is the end-to-end regression test for
// the unauthenticated credit-mint bug. The mpc rail is registered in the global
// registry by the provider barrel, and /v1/billing/webhooks/:provider is
// deliberately unauthenticated — the signature is the only trust anchor. Before
// the fix this exact request returned 200 and reached the mint with an
// attacker-chosen tenant, beneficiary and amount. It must now be a 401.
func TestMPCWebhook_ForgedSignatureRejected(t *testing.T) {
	registerMPC(t, "whsec_mpc")

	forged := []string{
		"totally-forged-not-an-hmac",
		"x",
		"deadbeef",
		strings.Repeat("a", 64), // correct length, wrong digest
	}

	for _, sig := range forged {
		t.Run(sig, func(t *testing.T) {
			r := newTestEngine()
			req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhooks/mpc",
				strings.NewReader(string(mpcSettlementEvent("evt_forged"))))
			req.Header.Set(mpcSigHeader, sig)
			// Attacker also chooses the tenant.
			req.Header.Set("X-Org-Id", "victim-org")

			resp, terr := r.Test(req)
			if terr != nil {
				t.Fatalf("Test: %v", terr)
			}
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 — a forged signature must not reach the mint", resp.StatusCode)
			}
		})
	}
}

// TestMPCWebhook_MissingSignatureRejected proves an unsigned delivery never gets
// past header selection.
func TestMPCWebhook_MissingSignatureRejected(t *testing.T) {
	registerMPC(t, "whsec_mpc")

	r := newTestEngine()
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhooks/mpc",
		strings.NewReader(string(mpcSettlementEvent("evt_nosig"))))
	resp, terr := r.Test(req)
	if terr != nil {
		t.Fatalf("Test: %v", terr)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a delivery with no signature header", resp.StatusCode)
	}
}

// TestMPCWebhook_UnsetSecretRejects proves the fail-closed property at the HTTP
// boundary: with no webhook secret provisioned the rail refuses every delivery
// rather than accepting all of them, which is what the old code did.
func TestMPCWebhook_UnsetSecretRejects(t *testing.T) {
	registerMPC(t, "") // secret deliberately unset

	body := mpcSettlementEvent("evt_nosecret")
	r := newTestEngine()
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhooks/mpc", strings.NewReader(string(body)))
	// Even a well-formed digest cannot help: there is no key to verify against.
	req.Header.Set(mpcSigHeader, mpcSign("", body))
	resp, terr := r.Test(req)
	if terr != nil {
		t.Fatalf("Test: %v", terr)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 — an unconfigured secret must fail closed", resp.StatusCode)
	}
}

// TestMPCWebhook_CorrectSignatureReachesTheRail proves the fix did not simply
// close the door on everyone: a genuinely signed delivery passes signature
// validation and is processed (the header the real sender uses is selected, the
// digest over the raw body matches, and the handler proceeds past the 401).
func TestMPCWebhook_CorrectSignatureReachesTheRail(t *testing.T) {
	const secret = "whsec_mpc"
	registerMPC(t, secret)

	body := mpcSettlementEvent("evt_genuine")
	r := newTestEngine()
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhooks/mpc", strings.NewReader(string(body)))
	req.Header.Set(mpcSigHeader, mpcSign(secret, body))
	resp, terr := r.Test(req)
	if terr != nil {
		t.Fatalf("Test: %v", terr)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusBadRequest {
		t.Fatalf("status = %d, want the signature to be ACCEPTED for a correctly signed delivery", resp.StatusCode)
	}
}

// TestPickSignatureHeader_MPC proves the real sender's header is selected, both
// on the mpc hint and through the generic candidate list. Without this the
// verified path is unreachable for a genuine delivery.
func TestPickSignatureHeader_MPC(t *testing.T) {
	for _, hint := range []string{"mpc", ""} {
		h := http.Header{}
		h.Set(mpcSigHeader, "sig-value")
		if got := pickSignatureHeader(h, hint); got != "sig-value" {
			t.Errorf("hint %q: pickSignatureHeader = %q, want the X-Webhook-Signature value", hint, got)
		}
	}
}
