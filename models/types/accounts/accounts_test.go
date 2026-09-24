package accounts

import (
	"encoding/json"
	"strings"
	"testing"
)

// A PAN and a CVV must not survive a round trip through JSON, in either direction.
//
// StripeAccount is embedded ANONYMOUSLY into Account, and Account is a field of
// payment.Payment — so encoding/json PROMOTES the embedded fields to the top level
// of any serialized payment. With `json:"number"` and `json:"cvc"` on them, every
// projection of a payment was one marshal away from carrying cardholder data, and
// two sites marshalled the whole struct.
//
// The INPUT half is asserted too, and it is deliberate rather than incidental: the
// shipped card boundary is SAQ-A — Square hosted fields return a single-use nonce
// and a PAN never touches this code — so a model that can be handed one over JSON
// contradicts the architecture around it.
func TestAccount_NeverSerializesCardholderData(t *testing.T) {
	a := Account{Type: StripeType}
	a.Number = "4111111111111111"
	a.CVC = "737"
	a.LastFour = "1111"
	a.Brand = "Visa"
	a.Stripe = StripeAccount{Number: "4242424242424242", CVC: "123", LastFour: "4242"}

	out, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(out)
	for _, secret := range []string{"4111111111111111", "737", "4242424242424242", "123"} {
		if strings.Contains(body, secret) {
			t.Errorf("serialized account carries cardholder data %q: %s", secret, body)
		}
	}
	// The safe descriptors a receipt actually needs are still there.
	for _, keep := range []string{"1111", "Visa"} {
		if !strings.Contains(body, keep) {
			t.Errorf("serialized account dropped %q, which a receipt needs: %s", keep, body)
		}
	}
}

func TestAccount_NeverAcceptsCardholderData(t *testing.T) {
	// Both spellings a caller could reach for: promoted from the embed, and nested
	// under "stripe".
	const body = `{"type":"stripe","number":"4111111111111111","cvc":"737",
	               "stripe":{"number":"4242424242424242","cvc":"123"}}`
	var a Account
	if err := json.Unmarshal([]byte(body), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if a.Number != "" || a.CVC != "" {
		t.Errorf("a posted PAN/CVV reached the model: number=%q cvc=%q", a.Number, a.CVC)
	}
	if a.Stripe.Number != "" || a.Stripe.CVC != "" {
		t.Errorf("a posted PAN/CVV reached the nested account: number=%q cvc=%q",
			a.Stripe.Number, a.Stripe.CVC)
	}
	// The rest of the object still decodes — this closes two fields, not the model.
	if a.Type != StripeType {
		t.Errorf("type = %q, want %q", a.Type, StripeType)
	}
}
