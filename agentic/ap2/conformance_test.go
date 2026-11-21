// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// The spec's own mandates (testdata/SOURCE): an Open Checkout Mandate, the Closed Checkout
// Mandate the agent signed under it, and the same for the payment. They are checked here with
// the code the verifier runs, everything the examples let a verifier check. The Trusted
// Agent Provider's key ("agent-provider-key-1") and the merchant's ("merchant-key-1") are not
// published with the examples, so the signature of each open token and of the checkout JWT
// cannot be; the closed tokens are signed by the key the open mandates carry in cnf, and
// that signature is.

func vector(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name + ".token")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(raw))
}

func mustParse(t *testing.T, s string) *presentation {
	t.Helper()
	p, err := parsePresentation(s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func mustDelegate(t *testing.T, v *Verifier, tk *token, now time.Time) *Delegate {
	t.Helper()
	d, err := v.delegate(tk, now)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// strict decodes a resolved delegate payload into a mandate with unknown members refused,
// and requires the mandate to encode back to the same JSON.
func strict(t *testing.T, claims map[string]any, into any) {
	t.Helper()
	raw, _ := json.Marshal(claims)
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		t.Fatalf("decode: %v\n%s", err, raw)
	}
	back, err := json.Marshal(into)
	if err != nil {
		t.Fatal(err)
	}
	var a, b any
	json.Unmarshal(raw, &a)
	json.Unmarshal(back, &b)
	if !equal(a, b) {
		t.Fatalf("re-encoded\n%s\nwant\n%s", back, raw)
	}
}

func TestSpecCheckoutMandates(t *testing.T) {
	chain := mustParse(t, vector(t, "checkout_chain"))
	if len(chain.tokens) != 2 {
		t.Fatalf("%d tokens", len(chain.tokens))
	}
	open, closed := chain.tokens[0], chain.tokens[1]
	if open.jws.header.Typ != "example+sd-jwt" || open.jws.header.Kid != "agent-provider-key-1" || closed.jws.header.Typ != "kb+sd-jwt" || open.alg != "sha-256" {
		t.Fatalf("headers %+v %+v", open.jws.header, closed.jws.header)
	}

	// The single-token examples are the two halves of the chained one, byte for byte.
	if got := vector(t, "checkout_open"); got != open.text() {
		t.Fatal("the open example is not the first token of the chain")
	}
	if got := vector(t, "checkout_closed"); got != closed.text() {
		t.Fatal("the closed example is not the second token of the chain")
	}

	vc := &Verifier{}
	od := mustDelegate(t, vc, open, time.Unix(1777342400, 0))
	var om OpenCheckoutMandate
	strict(t, od.Claims, &om)
	if om.VCT != VCTCheckoutOpen || len(om.Constraints) != 2 {
		t.Fatalf("open mandate %+v", om)
	}
	li := om.Constraints[0].(LineItems)
	if len(li.Items) != 1 || li.Items[0].Quantity != 1 || li.Items[0].AcceptableItems[0].ID != "supershoe_limited_edition_gold_sneaker_womens_9_0" {
		t.Fatalf("line_items %+v", li)
	}
	if am := om.Constraints[1].(AllowedMerchants); am.Allowed[0].ID != "merchant_1" || am.Allowed[0].Website != "https://demo-merchant.example" {
		t.Fatalf("allowed_merchants %+v", am)
	}

	// The closed token is signed by the key in the open mandate's cnf.
	key, err := om.CNF.JWK.Key()
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.jws.verify(key); err != nil {
		t.Fatalf("the closed example's signature: %v", err)
	}
	// ... and binds to the open token as presented, by sd_hash.
	if h, _ := open.hash(); closed.claims["sd_hash"] != h || h != "FzLoxbbtgQGYZxoSM2NJYJtkFTSsdfUBoVEQ12k7JN8" {
		t.Fatalf("sd_hash %v, hash of the open token %s", closed.claims["sd_hash"], h)
	}

	cd := mustDelegate(t, vc, closed, time.Unix(1777342400, 0))
	var cm CheckoutMandate
	strict(t, cd.Claims, &cm)
	if cm.VCT != VCTCheckout || cm.CheckoutHash != "NivWhuqfzcvZNapvIEJ2-3tsdQLkiuIcye2g46WVgX8" {
		t.Fatalf("closed mandate %+v", cm)
	}
	// checkout_hash is the hash of checkout_jwt, in the algorithm of the SD-JWT.
	if h, _ := digest(closed.alg, cm.CheckoutJWT); h != cm.CheckoutHash {
		t.Fatalf("hash of checkout_jwt %s, checkout_hash %s", h, cm.CheckoutHash)
	}
	// aud, nonce and iat of the key-bound token.
	if closed.claims["aud"] != "merchant" || closed.claims["nonce"] != "b9c8d7e6f5a4b3c2d1e0f9a8b7c6d5e4" {
		t.Fatalf("claims %+v", closed.claims)
	}
	// The checkout JWT of the example is ES256 and carries a merchant, line items and a total.
	j, err := parseJWS(cm.CheckoutJWT)
	if err != nil || j.header.Alg != "ES256" || j.header.Kid != "merchant-key-1" {
		t.Fatalf("checkout_jwt %v %+v", err, j)
	}

	// Its signer and the open token's issuer are not published, so a verifier that trusts
	// other keys refuses the example at its first token, and for that reason.
	v := &Verifier{Trust: Keys{}, Now: func() time.Time { return time.Unix(1777342400, 0) }}
	_, err = v.Checkout(context.Background(), vector(t, "checkout_chain"), Want{Audience: "merchant", Nonce: "b9c8d7e6f5a4b3c2d1e0f9a8b7c6d5e4"}, &newParty(t, "m").key.PublicKey)
	refusedWith(t, err, InvalidCredential, ErrKey)
}

func TestSpecPaymentMandates(t *testing.T) {
	chain := mustParse(t, vector(t, "payment_chain"))
	open, closed := chain.tokens[0], chain.tokens[1]
	if got := vector(t, "payment_open"); got != open.text() {
		t.Fatal("the open example is not the first token of the chain")
	}
	if got := vector(t, "payment_closed"); got != closed.text() {
		t.Fatal("the closed example is not the second token of the chain")
	}
	now := time.Unix(1777342400, 0)
	vc := &Verifier{}
	od := mustDelegate(t, vc, open, now)
	var om OpenPaymentMandate
	strict(t, od.Claims, &om)
	if om.VCT != VCTPaymentOpen || len(om.Constraints) != 3 {
		t.Fatalf("open payment mandate %+v", om)
	}
	if r := om.Constraints[0].(AmountRange); r.Currency != "USD" || r.Max != 20000 || *r.Min != 0 {
		t.Fatalf("amount_range %+v", r)
	}
	key, err := om.CNF.JWK.Key()
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.jws.verify(key); err != nil {
		t.Fatalf("the closed example's signature: %v", err)
	}
	if h, _ := open.hash(); closed.claims["sd_hash"] != h {
		t.Fatalf("sd_hash %v, hash of the open token %s", closed.claims["sd_hash"], h)
	}
	cd := mustDelegate(t, vc, closed, now)
	var pm PaymentMandate
	strict(t, cd.Claims, &pm)
	if pm.VCT != VCTPayment || pm.PaymentAmount != (Amount{Amount: 19900, Currency: "USD"}) || pm.Payee.ID != "merchant_1" {
		t.Fatalf("closed payment mandate %+v", pm)
	}

	// The payment is bound to the checkout: its transaction_id is the checkout mandate's
	// checkout_hash, and the open payment mandate's payment.reference is the hash of the open
	// checkout mandate.
	cc := mustParse(t, vector(t, "checkout_chain"))
	ccd := mustDelegate(t, vc, cc.tokens[1], now)
	if pm.TransactionID != ccd.Claims["checkout_hash"] {
		t.Fatalf("transaction_id %s, checkout_hash %v", pm.TransactionID, ccd.Claims["checkout_hash"])
	}
	openCheckoutHash, _ := cc.tokens[0].hash()
	if ref := om.Constraints[2].(Reference); ref.ConditionalTransactionID != openCheckoutHash {
		t.Fatalf("payment.reference %s, hash of the open checkout mandate %s", ref.ConditionalTransactionID, openCheckoutHash)
	}
	// Every constraint of the open payment mandate holds for the closed one.
	env := payEnv{now: now, openCheckout: openCheckoutHash}
	if err := (&om).check(&pm, env); err != nil {
		t.Fatal(err)
	}
	// Presented with another open checkout mandate it does not.
	env.openCheckout = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if err := (&om).check(&pm, env); err == nil {
		t.Fatal("the reference to another checkout mandate held")
	}
}

// The checkout JWT of the spec's example mandate is a UCP Checkout object, as the AP2 schemas
// define it (the decoded view in the documentation shows another shape; the encoded token is
// the example). It decodes into the Checkout type, with unknown members refused, and holds the
// item and the merchant its open mandate allows.
func TestSpecExampleCheckoutSatisfiesItsOpenMandate(t *testing.T) {
	chain := mustParse(t, vector(t, "checkout_chain"))
	now := time.Unix(1777342400, 0)
	v := &Verifier{}
	o, oerr := openCheckout(mustDelegate(t, v, chain.tokens[0], now).Claims)
	if oerr != nil {
		t.Fatal(oerr)
	}
	var cm CheckoutMandate
	claims(mustDelegate(t, v, chain.tokens[1], now).Claims, &cm)
	j, err := parseJWS(cm.CheckoutJWT)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := object(j.payload)
	if err != nil {
		t.Fatal(err)
	}
	strict(t, raw, &Checkout{})
	co, err := decodeCheckout(j.payload)
	if err != nil {
		t.Fatal(err)
	}
	if total, _ := co.Total("total"); co.Currency != "USD" || total != 19900 || co.Status != Incomplete || co.Merchant.ID != "merchant_1" {
		t.Fatalf("checkout = %+v", co)
	}
	// The payment of the example mandate pays exactly this checkout.
	pay := mustParse(t, vector(t, "payment_chain"))
	var pm PaymentMandate
	claims(mustDelegate(t, v, pay.tokens[1], now).Claims, &pm)
	if total, _ := co.Total("total"); pm.PaymentAmount.Amount != total || pm.PaymentAmount.Currency != co.Currency {
		t.Fatalf("the payment of %+v does not pay the checkout", pm.PaymentAmount)
	}
	if err := o.check(co); err != nil {
		t.Fatal(err)
	}
	co.LineItems[0].Quantity = 2
	if err := o.check(co); err == nil {
		t.Fatal("two pairs of shoes satisfied a mandate for one")
	}
}
