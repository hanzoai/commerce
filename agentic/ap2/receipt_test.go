// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckoutReceipts(t *testing.T) {
	w := newWorld(t)
	_, jwt, hash := w.checkout("chk_1", 1, 500)
	text := w.checkoutMandate(jwt, hash).text()
	p, _ := parsePresentation(text)
	ref := p.reference()
	pub := &w.merchant.Key.PublicKey

	ok := CheckoutReceipt{Status: Success, Issuer: "https://demo-merchant.example", IssuedAt: w.now.Unix(), Reference: ref, OrderID: "ord_1"}
	signed, err := w.merchant.receipt(&ok)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ParseCheckoutReceipt(signed, pub, text)
	if err != nil || r.Status != Success || r.Result != "success" || r.OrderID != "ord_1" || r.Reference != ref {
		t.Fatalf("receipt = %+v %v", r, err)
	}
	bad := CheckoutReceipt{Status: Failure, Issuer: ok.Issuer, IssuedAt: ok.IssuedAt, Reference: ref, Error: string(InvalidMandate), ErrorDescription: "no"}
	signed2, err := w.merchant.receipt(&bad)
	if err != nil {
		t.Fatal(err)
	}
	if r, err = ParseCheckoutReceipt(signed2, pub, text); err != nil || r.Result != "error" || r.Error != "invalid_mandate" {
		t.Fatalf("receipt = %+v %v", r, err)
	}

	// A receipt is the merchant's, for this mandate, and unaltered.
	other := newParty(t, "other")
	for name, f := range map[string]func() error{
		"another issuer's key": func() error { _, err := ParseCheckoutReceipt(signed, &other.key.PublicKey, text); return err },
		"a tampered receipt": func() error {
			i := strings.Index(signed, ".") + 3
			c := byte('A')
			if signed[i] == 'A' {
				c = 'B'
			}
			_, err := ParseCheckoutReceipt(signed[:i]+string(c)+signed[i+1:], pub, text)
			return err
		},
		"another mandate": func() error {
			_, j2, h2 := w.checkout("chk_2", 1, 500)
			_, err := ParseCheckoutReceipt(signed, pub, w.checkoutMandate(j2, h2).text())
			return err
		},
		"not a JWT": func() error { _, err := ParseCheckoutReceipt("nope", pub, ""); return err },
	} {
		if err := f(); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, err := ParseCheckoutReceipt(signed, pub, text[:len(text)-1]); err == nil {
		t.Error("an unreadable mandate accepted")
	}
}

// The shapes the schemas allow, and no others.
func TestReceiptShapes(t *testing.T) {
	w := newWorld(t)
	for name, r := range map[string]CheckoutReceipt{
		"a success with no order":      {Status: Success, Issuer: "iss", IssuedAt: 1, Reference: "ref"},
		"a success with an error":      {Status: Success, Issuer: "iss", IssuedAt: 1, Reference: "ref", OrderID: "o", Error: "e"},
		"an error with no description": {Status: Failure, Issuer: "iss", IssuedAt: 1, Reference: "ref", Error: "e"},
		"an error with an order":       {Status: Failure, Issuer: "iss", IssuedAt: 1, Reference: "ref", Error: "e", ErrorDescription: "d", OrderID: "o"},
		"no reference":                 {Status: Success, Issuer: "iss", IssuedAt: 1, OrderID: "o"},
		"no issuer":                    {Status: Success, IssuedAt: 1, Reference: "ref", OrderID: "o"},
		"a status nobody defined":      {Status: "Pending", Issuer: "iss", IssuedAt: 1, Reference: "ref", OrderID: "o"},
	} {
		if _, err := w.merchant.receipt(&r); err == nil {
			t.Errorf("%s signed", name)
		}
	}

	pay := PaymentReceipt{Status: Success, Issuer: "iss", IssuedAt: 1, Reference: "ref", PaymentID: "p", PSPConfirmationID: "psp", NetworkConfirmationID: "net"}
	signed, err := w.merchant.receipt(&pay)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ParsePaymentReceipt(signed, &w.merchant.Key.PublicKey, "")
	if err != nil || r.Result != "success" || r.NetworkConfirmationID != "net" {
		t.Fatalf("%+v %v", r, err)
	}
	for name, r := range map[string]PaymentReceipt{
		"a success with no network confirmation": {Status: Success, Issuer: "iss", IssuedAt: 1, Reference: "ref", PaymentID: "p", PSPConfirmationID: "psp"},
		"a success with no psp confirmation":     {Status: Success, Issuer: "iss", IssuedAt: 1, Reference: "ref", PaymentID: "p", NetworkConfirmationID: "net"},
		"no payment id":                          {Status: Success, Issuer: "iss", IssuedAt: 1, Reference: "ref", PSPConfirmationID: "psp", NetworkConfirmationID: "net"},
		"an error with a confirmation":           {Status: Failure, Issuer: "iss", IssuedAt: 1, Reference: "ref", PaymentID: "p", Error: "e", ErrorDescription: "d", PSPConfirmationID: "psp"},
		"an error with no code":                  {Status: Failure, Issuer: "iss", IssuedAt: 1, Reference: "ref", PaymentID: "p", ErrorDescription: "d"},
	} {
		if _, err := w.merchant.receipt(&r); err == nil {
			t.Errorf("%s signed", name)
		}
	}
}

func TestAReceiptThatIsNotValidIsNotAccepted(t *testing.T) {
	w := newWorld(t)
	// signed by the right key, but not a receipt the schema allows
	forged, _ := w.merchant.Sign(map[string]any{"status": "Success", "iss": "x", "iat": 1, "reference": "r"})
	_, err := ParseCheckoutReceipt(forged, &w.merchant.Key.PublicKey, "")
	if !errors.Is(err, ErrMalformed) {
		t.Fatalf("err = %v", err)
	}
}

func TestJWKRefusals(t *testing.T) {
	p := newParty(t, "k")
	good := NewJWK(&p.key.PublicKey, "k")
	if _, err := good.Key(); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(k *JWK){
		"another curve":          func(k *JWK) { k.Crv = "P-384" },
		"another key type":       func(k *JWK) { k.Kty = "OKP" },
		"a short coordinate":     func(k *JWK) { k.X = k.X[:40] },
		"padding":                func(k *JWK) { k.X += "=" },
		"a point off the curve":  func(k *JWK) { k.Y = k.X },
		"use enc":                func(k *JWK) { k.Use = "enc" },
		"another alg":            func(k *JWK) { k.Alg = "ES384" },
		"key_ops without verify": func(k *JWK) { k.KeyOps = []string{"sign"} },
	} {
		k := good
		mutate(&k)
		if _, err := k.Key(); !errors.Is(err, ErrKey) {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, raw := range map[string]any{
		"a private part":  map[string]any{"kty": "EC", "crv": "P-256", "x": good.X, "y": good.Y, "d": "AAAA"},
		"an extra member": map[string]any{"kty": "EC", "crv": "P-256", "x": good.X, "y": good.Y, "extra": 1},
		"not an object":   "key",
	} {
		if _, err := decodeJWK(raw); !errors.Is(err, ErrKey) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
