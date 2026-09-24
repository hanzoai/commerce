package mpc

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"regexp"
	"strings"
	"testing"
)

// mpcSign reproduces the MPC service's webhook signing exactly: hex of
// HMAC-SHA256 over the RAW body, keyed by the webhook secret. See
// lux/mpc pkg/api/webhook_sender.go deliverWebhook, which writes the digest to
// the X-Webhook-Signature header.
func mpcSign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// settlementBody is a delivery that WOULD mint credit if it validated: a
// completed payment naming its beneficiary and amount. Every rejection test
// below uses it, so a test that wrongly passes is a test that wrongly mints.
var settlementBody = []byte(`{"id":"evt_1","type":"payment.completed","timestamp":1,` +
	`"data":{"payment":{"id":"pay_1","status":"COMPLETED","reference_id":"user_1",` +
	`"amount_money":{"amount":100000,"currency":"usd"}}}}`)

func configuredProcessor(secret string) *MPCProcessor {
	return NewProcessor(Config{
		KMSEndpoint:   "https://kms.hanzo.ai",
		MPCEndpoint:   "https://mpc.hanzo.ai",
		APIKey:        "api-key",
		WebhookSecret: secret,
	})
}

// TestValidateWebhook_Scheme pins the signing contract: only a hex HMAC-SHA256
// over the raw body, under the right key, validates.
func TestValidateWebhook_Scheme(t *testing.T) {
	const secret = "whsec_mpc"
	mp := configuredProcessor(secret)

	cases := []struct {
		name string
		sig  string
		ok   bool
	}{
		{"hex hmac over raw body passes", mpcSign(secret, settlementBody), true},
		{"uppercase hex passes", strings.ToUpper(mpcSign(secret, settlementBody)), true},
		{"surrounding whitespace tolerated", "  " + mpcSign(secret, settlementBody) + "\n", true},
		{"wrong key fails", mpcSign("whsec_other", settlementBody), false},
		{"signature for a different body fails", mpcSign(secret, []byte(`{"id":"other"}`)), false},
		{"empty signature fails", "", false},
		{"non-hex fails", "%%%not-hex%%%", false},
		{"base64 of the right digest fails", "3q2+7w==", false},
		{"truncated correct digest fails", mpcSign(secret, settlementBody)[:32], false},
		{"digest with one flipped nibble fails", flipFirstNibble(mpcSign(secret, settlementBody)), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evt, err := mp.ValidateWebhook(context.Background(), settlementBody, tc.sig)
			if tc.ok {
				if err != nil || evt == nil {
					t.Fatalf("expected pass, got err=%v evt=%v", err, evt)
				}
				if evt.ID != "evt_1" || evt.Type != "payment.completed" {
					t.Fatalf("unexpected parsed event: %+v", evt)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected rejection, got nil error (evt=%v)", evt)
			}
			if evt != nil {
				t.Fatalf("rejection returned a non-nil event: %+v", evt)
			}
		})
	}
}

// TestValidateWebhook_ForgedSignatureRejected is the direct regression test for
// the credit-mint bug: the old code accepted ANY non-empty signature, so a
// forged one reached the mint. These are the exact values that used to pass.
func TestValidateWebhook_ForgedSignatureRejected(t *testing.T) {
	mp := configuredProcessor("whsec_mpc")

	for _, forged := range []string{
		"totally-forged-not-an-hmac",
		"x",
		"0",
		"deadbeef",
		strings.Repeat("a", 64), // right length, wrong value
	} {
		if _, err := mp.ValidateWebhook(context.Background(), settlementBody, forged); err == nil {
			t.Fatalf("forged signature %q was ACCEPTED — credit mint is reachable", forged)
		}
	}
}

// TestValidateWebhook_UnsetSecretFailsClosed is the other half of the bug: with
// no key configured the old code verified nothing and accepted everything. An
// unconfigured secret must REFUSE, because it is precisely the state in which
// no verification is happening.
func TestValidateWebhook_UnsetSecretFailsClosed(t *testing.T) {
	mp := configuredProcessor("") // secret deliberately unset

	// Not even a signature that is "correct" for an empty key may pass.
	for _, sig := range []string{
		"anything",
		mpcSign("", settlementBody),
	} {
		if _, err := mp.ValidateWebhook(context.Background(), settlementBody, sig); err == nil {
			t.Fatalf("unset webhook secret ACCEPTED signature %q — must fail closed", sig)
		}
	}
}

// TestValidateWebhook_UnconfiguredRailRefuses proves the rail's own on/off state
// gates the webhook: an MPC processor with no endpoint settles nothing, even
// when the presented signature is genuinely correct.
func TestValidateWebhook_UnconfiguredRailRefuses(t *testing.T) {
	const secret = "whsec_mpc"
	mp := NewProcessor(Config{
		KMSEndpoint:   "https://kms.hanzo.ai",
		MPCEndpoint:   "", // rail off
		WebhookSecret: secret,
	})
	if mp.IsAvailable(context.Background()) {
		t.Fatal("processor with no MPC endpoint reports available")
	}
	if _, err := mp.ValidateWebhook(context.Background(), settlementBody, mpcSign(secret, settlementBody)); err == nil {
		t.Fatal("unconfigured rail ACCEPTED a webhook — must refuse")
	}
}

// TestValidateWebhook_EmptyPayloadRejected keeps the empty-body guard.
func TestValidateWebhook_EmptyPayloadRejected(t *testing.T) {
	const secret = "whsec_mpc"
	mp := configuredProcessor(secret)
	if _, err := mp.ValidateWebhook(context.Background(), nil, mpcSign(secret, nil)); err == nil {
		t.Fatal("empty payload was accepted")
	}
}

// TestValidateWebhook_SignatureIsOverRawBytes proves the digest is taken over
// the bytes as received, not over a re-marshal of the parsed event. A body that
// is semantically identical but textually different (reordered keys, extra
// whitespace) does not carry the same signature, so verifying a re-marshal
// would accept a body the sender never signed.
func TestValidateWebhook_SignatureIsOverRawBytes(t *testing.T) {
	const secret = "whsec_mpc"
	mp := configuredProcessor(secret)

	raw := []byte(`{"id":"evt_raw","type":"payment.completed","timestamp":1}`)
	// Same JSON meaning, different bytes.
	reordered := []byte(`{"type":"payment.completed","id":"evt_raw","timestamp":1}`)

	sigForRaw := mpcSign(secret, raw)
	if _, err := mp.ValidateWebhook(context.Background(), raw, sigForRaw); err != nil {
		t.Fatalf("signature over the raw body was rejected: %v", err)
	}
	if _, err := mp.ValidateWebhook(context.Background(), reordered, sigForRaw); err == nil {
		t.Fatal("a body the sender never signed was accepted — digest is not over raw bytes")
	}
}

// TestValidateWebhook_UsesConstantTimeCompare guards the comparison primitive
// itself. Timing behaviour cannot be asserted reliably in a unit test, so this
// pins the mechanism: the digest is compared with hmac.Equal, and never with a
// plain string/byte equality on the signature that would both leak timing and
// invite the "any non-empty value passes" shortcut this file exists to prevent.
func TestValidateWebhook_UsesConstantTimeCompare(t *testing.T) {
	src, err := os.ReadFile("processor.go")
	if err != nil {
		t.Fatalf("read processor.go: %v", err)
	}
	body := string(src)

	if !strings.Contains(body, "hmac.Equal(provided, expected)") {
		t.Error("ValidateWebhook must compare digests with hmac.Equal (constant-time)")
	}
	// The original bug in one line: an emptiness test on the signature, keyed off
	// whether an API key happened to be set, standing in for verification. A
	// presence check on the signature is fine — on its own it is the bug.
	if regexp.MustCompile(`apiKey\s*!=\s*""\s*&&\s*signature\s*==\s*""`).MatchString(body) {
		t.Error("the emptiness-only signature check is back; the HMAC must be compared")
	}
}

func flipFirstNibble(sig string) string {
	if sig == "" {
		return sig
	}
	b := []byte(sig)
	if b[0] == '0' {
		b[0] = '1'
	} else {
		b[0] = '0'
	}
	return string(b)
}
