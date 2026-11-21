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

// Evidence is what a dispute brings together: the two mandates as presented, the two receipts that
// answered them (specification.md, "Dispute Evidence"), the keys of the merchant and the processor,
// the audience and nonce the Checkout Mandate was bound to, and what had been spent under an open
// Payment Mandate.
type Evidence struct {
	Checkout        string
	Payment         string
	CheckoutReceipt string
	PaymentReceipt  string
	Merchant        *ecdsa.PublicKey
	Processor       *ecdsa.PublicKey
	Want            Want
	Spent           Spent
}

// Dispute applies the steps of specification.md, "Dispute": the Checkout Mandate verifies by the
// merchant's rules, with its checkout_hash computed from the checkout_jwt; the Checkout Receipt's
// reference is the hash of the closed Checkout Mandate; the Payment Mandate verifies by the
// processor's rules against that checkout; and the Payment Receipt's reference is the hash of the
// closed Payment Mandate. Both receipts must be success receipts. Each mandate is checked as of
// the time its receipt was issued: it may have expired since.
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
	proof, err := v.at(cr.IssuedAt).Checkout(ctx, ev.Checkout, ev.Want, ev.Merchant)
	if err != nil {
		return fmt.Errorf("checkout mandate: %w", err)
	}
	if _, err := v.at(pr.IssuedAt).Payment(ctx, ev.Payment, PayWant{CheckoutJWT: proof.Mandate.CheckoutJWT, OpenCheckout: proof.OpenToken, Spent: ev.Spent}); err != nil {
		return fmt.Errorf("payment mandate: %w", err)
	}
	return nil
}

// at is the verifier with its clock at the Unix time t.
func (v *Verifier) at(t int64) *Verifier {
	c := *v
	c.Now = func() time.Time { return time.Unix(t, 0) }
	return &c
}
