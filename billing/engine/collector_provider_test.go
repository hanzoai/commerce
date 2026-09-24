package engine

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/types/currency"
)

// The ProviderCharger leg of the prepaid -> card waterfall. These pin the exact
// invariants the money path relies on:
//
//	card covers the remainder            -> invoice PAID, method "card", ref recorded
//	card declines                        -> invoice stays OPEN, no state, charger hit ONCE
//	credits cover the invoice fully      -> the card is NEVER charged
//	card declines after prepaid covered part -> nothing prepaid is spent

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

	// nil prepaid (no credits, no balance): the whole 2000 is the card's —
	// exactly the card-only collection shape.
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
	inv.Id_ = "inv_u_credit_first" // every invoice the collector sees is stored
	inv.SubscriptionId = "sub_credit_first"

	pre := &purse{credit: 1000}
	cardCalls := 0
	charger := ProviderCharger(func(_ context.Context, _ *datastore.Datastore, _ *billinginvoice.BillingInvoice, _ int64) (string, error) {
		cardCalls++
		return "should_not_be_used", nil
	})

	result, err := CollectInvoice(context.Background(), nil, inv, pre, charger)
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

// TestCollectInvoice_DeclineLeavesPrepaidUntouched: prepaid money covers part of
// the invoice and the card is asked for the rest. The card declines, and the
// credits and the balance are exactly where they were — a failed collection
// spends nothing, so the retry does not spend it again.
func TestCollectInvoice_DeclineLeavesPrepaidUntouched(t *testing.T) {
	inv := &billinginvoice.BillingInvoice{}
	inv.Status = billinginvoice.Open
	inv.AmountDue = 2500
	inv.UserId = "u_short"
	inv.Id_ = "inv_u_short" // every invoice the collector sees is stored

	pre := &purse{credit: 300, balance: 700}
	asked := int64(0)
	charger := ProviderCharger(func(_ context.Context, _ *datastore.Datastore, _ *billinginvoice.BillingInvoice, amt int64) (string, error) {
		asked = amt
		return "", fmt.Errorf("card declined")
	})

	result, err := CollectInvoice(context.Background(), nil, inv, pre, charger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success || inv.Status != billinginvoice.Open {
		t.Fatalf("success=%v status=%s, want a failed attempt and an open invoice", result.Success, inv.Status)
	}
	if asked != 1500 {
		t.Fatalf("card asked for %d, want the 1500 prepaid money does not cover", asked)
	}
	if pre.credit != 300 || pre.balance != 700 || len(pre.draws) != 0 {
		t.Fatalf("prepaid after a decline: credit=%d balance=%d draws=%v; a failed collection must spend nothing", pre.credit, pre.balance, pre.draws)
	}
	if inv.AmountPaid != 0 || result.AmountCharged != 0 {
		t.Fatalf("amountPaid=%d charged=%d, want 0", inv.AmountPaid, result.AmountCharged)
	}
}

// TestCollectInvoice_PrepaidAndCard: prepaid money covers what it holds, the
// card the rest, and the invoice is paid by the card that settled it.
func TestCollectInvoice_PrepaidAndCard(t *testing.T) {
	inv := &billinginvoice.BillingInvoice{}
	inv.Status = billinginvoice.Open
	inv.AmountDue = 2500
	inv.UserId = "u_mixed"
	inv.Id_ = "inv_u_mixed" // every invoice the collector sees is stored

	pre := &purse{credit: 300, balance: 700}
	charger := ProviderCharger(func(_ context.Context, _ *datastore.Datastore, _ *billinginvoice.BillingInvoice, amt int64) (string, error) {
		if amt != 1500 {
			t.Fatalf("card asked for %d, want 1500", amt)
		}
		return "sqpay_rest", nil
	})

	result, err := CollectInvoice(context.Background(), nil, inv, pre, charger)
	if err != nil || !result.Success {
		t.Fatalf("result=%+v err=%v, want success", result, err)
	}
	if result.CreditUsed != 300 || result.BalanceUsed != 700 || result.ProviderUsed != 1500 {
		t.Fatalf("credit=%d balance=%d card=%d, want 300/700/1500", result.CreditUsed, result.BalanceUsed, result.ProviderUsed)
	}
	if pre.credit != 0 || pre.balance != 0 {
		t.Fatalf("prepaid left credit=%d balance=%d, want both spent", pre.credit, pre.balance)
	}
	if inv.Status != billinginvoice.Paid || inv.PaymentMethod != "card" || inv.AmountPaid != 2500 {
		t.Fatalf("invoice %s by %q paid %d, want paid by card in full", inv.Status, inv.PaymentMethod, inv.AmountPaid)
	}
}

// purse is a Prepaid holding credit and balance in memory. Draw is all or
// nothing, credits first, exactly as the real one is.
type purse struct {
	credit, balance int64
	readErr         error
	draws           []int64
}

func (p *purse) Available(context.Context, string, currency.Type) (int64, error) {
	if p.readErr != nil {
		return 0, p.readErr
	}
	return p.credit + p.balance, nil
}

func (p *purse) Draw(_ context.Context, _ string, _ currency.Type, amount int64, ref string) (Drawn, error) {
	if ref == "" {
		return Drawn{}, fmt.Errorf("no ref")
	}
	if p.credit+p.balance < amount {
		return Drawn{}, fmt.Errorf("short")
	}
	d := Drawn{Credit: min(p.credit, amount)}
	d.Balance = amount - d.Credit
	if d.Balance > 0 {
		d.Ref = "led_" + ref
	}
	p.credit -= d.Credit
	p.balance -= d.Balance
	p.draws = append(p.draws, amount)
	return d, nil
}
