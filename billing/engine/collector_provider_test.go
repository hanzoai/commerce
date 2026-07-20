package engine

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
)

// The ProviderCharger (step-3) leg of the credits -> balance -> card waterfall.
// These pin the exact invariants the money path relies on:
//
//	card covers the remainder            -> invoice PAID, method "card", ref recorded
//	card declines                        -> invoice stays OPEN, no state, charger hit ONCE
//	credits cover the invoice fully      -> the card is NEVER charged

// TestCollectInvoice_ProviderCharger_CoversRemainder proves the vaulted card pays
// whatever credits + balance did not: the invoice is marked PAID by "card" and
// carries the processor ref, and the charger is invoked exactly once.
func TestCollectInvoice_ProviderCharger_CoversRemainder(t *testing.T) {
	inv := &billinginvoice.BillingInvoice{}
	inv.Status = billinginvoice.Open
	inv.AmountDue = 2000
	inv.UserId = "u_card"
	inv.SubscriptionId = "sub_card"

	calls := 0
	charger := ProviderCharger(func(_ context.Context, _ *datastore.Datastore, i *billinginvoice.BillingInvoice, amt int64) (string, error) {
		calls++
		if amt != 2000 {
			t.Fatalf("charger amount = %d, want 2000 (full remainder, no credits/balance)", amt)
		}
		if i.SubscriptionId != "sub_card" {
			t.Fatalf("charger got invoice subscription %q, want sub_card", i.SubscriptionId)
		}
		return "sqpay_ok", nil
	})

	// nil burner (no credits); db nil so the balance leg errors non-fatally, leaving
	// the whole 2000 for the card — exactly the card-only collection shape.
	result, err := CollectInvoice(nil, nil, inv, nil, charger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("want success (card covered remainder); error=%q", result.Error)
	}
	if calls != 1 {
		t.Fatalf("charger called %d times, want exactly 1 (no double-charge)", calls)
	}
	if result.ProviderUsed != 2000 {
		t.Fatalf("ProviderUsed = %d, want 2000", result.ProviderUsed)
	}
	if result.ProviderRef != "sqpay_ok" {
		t.Fatalf("ProviderRef = %q, want sqpay_ok", result.ProviderRef)
	}
	if inv.Status != billinginvoice.Paid {
		t.Fatalf("invoice status = %s, want paid", inv.Status)
	}
	if inv.PaymentMethod != "card" {
		t.Fatalf("invoice paymentMethod = %q, want card", inv.PaymentMethod)
	}
	if inv.PaymentRef != "sqpay_ok" {
		t.Fatalf("invoice paymentRef = %q, want sqpay_ok", inv.PaymentRef)
	}
	if inv.AmountPaid != 2000 {
		t.Fatalf("invoice amountPaid = %d, want 2000", inv.AmountPaid)
	}
}

// TestCollectInvoice_ProviderCharger_DeclineLeavesOpen proves a declined card
// leaves the invoice OPEN (unpaid), records nothing, and calls the charger exactly
// once — so the dunning retry (a later CollectInvoice) is what re-charges, never a
// double-charge inside one attempt.
func TestCollectInvoice_ProviderCharger_DeclineLeavesOpen(t *testing.T) {
	inv := &billinginvoice.BillingInvoice{}
	inv.Status = billinginvoice.Open
	inv.AmountDue = 2000
	inv.UserId = "u_decline"
	inv.SubscriptionId = "sub_decline"

	calls := 0
	charger := ProviderCharger(func(_ context.Context, _ *datastore.Datastore, _ *billinginvoice.BillingInvoice, _ int64) (string, error) {
		calls++
		return "", fmt.Errorf("card declined — insufficient funds")
	})

	result, err := CollectInvoice(nil, nil, inv, nil, charger)
	if err != nil {
		t.Fatalf("a decline must be non-fatal at the call level, got err: %v", err)
	}
	if result.Success {
		t.Fatal("want failure when the card declines")
	}
	if calls != 1 {
		t.Fatalf("charger called %d times, want exactly 1 (no double-charge on decline)", calls)
	}
	if inv.Status != billinginvoice.Open {
		t.Fatalf("invoice status = %s, want OPEN (declined card must not close the invoice)", inv.Status)
	}
	if result.ProviderUsed != 0 {
		t.Fatalf("ProviderUsed = %d, want 0 on decline", result.ProviderUsed)
	}
	if !strings.Contains(result.Error, "declined") {
		t.Fatalf("result.Error = %q, want it to carry the decline reason", result.Error)
	}
}

// TestCollectInvoice_CreditsCoverFully_NoProviderCharge proves the waterfall order:
// when credits settle the invoice, the vaulted card is NEVER charged.
func TestCollectInvoice_CreditsCoverFully_NoProviderCharge(t *testing.T) {
	inv := &billinginvoice.BillingInvoice{}
	inv.Status = billinginvoice.Open
	inv.AmountDue = 1000
	inv.UserId = "u_credit_first"
	inv.SubscriptionId = "sub_credit_first"

	burner := CreditBurner(func(_ *datastore.Datastore, _ string, _ int64, _ string) (int64, error) {
		return 0, nil // credits cover the full amount
	})
	cardCalls := 0
	charger := ProviderCharger(func(_ context.Context, _ *datastore.Datastore, _ *billinginvoice.BillingInvoice, _ int64) (string, error) {
		cardCalls++
		return "should_not_be_used", nil
	})

	result, err := CollectInvoice(nil, nil, inv, burner, charger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("want success (credits cover the invoice)")
	}
	if cardCalls != 0 {
		t.Fatalf("card charged %d times, want 0 — credits cover the invoice, the card must not be touched", cardCalls)
	}
	if inv.PaymentMethod != "credit" {
		t.Fatalf("invoice paymentMethod = %q, want credit", inv.PaymentMethod)
	}
	if result.CreditUsed != 1000 {
		t.Fatalf("CreditUsed = %d, want 1000", result.CreditUsed)
	}
}
