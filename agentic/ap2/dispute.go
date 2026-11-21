// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"time"
)

// Evidence is what a dispute brings together: the two mandates as presented and the two
// receipts that answered them (specification.md, "Dispute Evidence").
type Evidence struct {
	// Checkout and Payment are the presentations of the closed Checkout Mandate and Payment
	// Mandate, each with its open mandate when it has one: the SD-JWTs as they were stored.
	Checkout string
	Payment  string
	// CheckoutReceipt and PaymentReceipt are the signed receipts.
	CheckoutReceipt string
	PaymentReceipt  string
	// Merchant is the key the merchant signed the checkout and its receipt with; Processor is
	// the key of the payment receipt's issuer.
	Merchant  *ecdsa.PublicKey
	Processor *ecdsa.PublicKey
	// Want is the audience and nonce the Checkout Mandate was bound to.
	Want Want
	// Spent is what had been accepted under an open Payment Mandate when this one was, for
	// the constraints that need it.
	Spent Spent
}

// Dispute checks that the mandates and receipts of a purchase are what they claim, so that
// they can stand as evidence of what the user and each role saw (specification.md,
// "Dispute"). The mandates are checked as of the time each receipt was issued, since they may
// have expired since. It follows the steps the spec lists: the Checkout Mandate verifies by the
// Merchant rules, with the hash of its checkout_jwt computed from the checkout_jwt itself;
// the Checkout Receipt's reference is the hash of the closed Checkout Mandate; the Payment
// Mandate verifies by the Merchant Payment Processor rules against that checkout; and the
// Payment Receipt's reference is the hash of the closed Payment Mandate. Both receipts must be
// success receipts.
func (v *Verifier) Dispute(ctx context.Context, ev Evidence) error {
	if ev.Merchant == nil || ev.Processor == nil {
		return errors.New("ap2: a dispute needs the merchant's key and the processor's")
	}
	cr, err := ParseCheckoutReceipt(ev.CheckoutReceipt, ev.Merchant, ev.Checkout)
	if err != nil {
		return fmt.Errorf("checkout receipt: %w", err)
	}
	pr, err := ParsePaymentReceipt(ev.PaymentReceipt, ev.Processor, ev.Payment)
	if err != nil {
		return fmt.Errorf("payment receipt: %w", err)
	}
	if cr.Status != Success || pr.Status != Success {
		return fmt.Errorf("%w: a dispute is over a purchase, and a receipt says it did not happen", ErrMandate)
	}
	proof, err := v.at(time.Unix(cr.IssuedAt, 0)).Checkout(ctx, ev.Checkout, ev.Want, ev.Merchant)
	if err != nil {
		return fmt.Errorf("checkout mandate: %w", err)
	}
	if _, err := v.at(time.Unix(pr.IssuedAt, 0)).Payment(ctx, ev.Payment, PayWant{CheckoutJWT: proof.Mandate.CheckoutJWT, OpenCheckout: proof.OpenToken, Spent: ev.Spent}); err != nil {
		return fmt.Errorf("payment mandate: %w", err)
	}
	return nil
}

// at is the verifier with its clock at t.
func (v *Verifier) at(t time.Time) *Verifier {
	c := *v
	c.Now = func() time.Time { return t }
	return &c
}
