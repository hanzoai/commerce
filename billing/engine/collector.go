package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"

	txutil "github.com/hanzoai/commerce/models/transaction/util"
)

// CollectionResult describes the outcome of a payment collection attempt.
type CollectionResult struct {
	Success       bool   `json:"success"`
	CreditUsed    int64  `json:"creditUsed"`
	BalanceUsed   int64  `json:"balanceUsed"`
	ProviderUsed  int64  `json:"providerUsed"`
	ProviderRef   string `json:"providerRef,omitempty"`
	AmountCharged int64  `json:"amountCharged"`
	Error         string `json:"error,omitempty"`
}

// CreditBurner is the function signature for burning credits.
// This matches the existing BurnCredits function in api/billing/credit_grants.go.
type CreditBurner func(db *datastore.Datastore, userId string, amount int64, meterId string) (int64, error)

// ProviderCharger charges amountCents (the invoice remainder left unpaid after
// credits + balance) on the external payment provider — the subscription's
// vaulted card-on-file — and returns the processor reference on success.
//
// It is injected exactly like CreditBurner so the engine never imports
// api/billing / payment / the Square SDK: no import cycle, and the collector
// stays provider-agnostic (it knows "charge the remainder", not "which
// provider"). Resolving the card is the callback's job (invoice -> subscription
// -> DefaultPaymentMethod -> providerRef + Square customer id). A nil charger
// means "no external provider" — credits + balance only (the prior Phase-5
// stub), so an unpaid remainder leaves the invoice OPEN.
//
// Fail-closed contract: on a decline / missing card / provider error the
// callback returns a non-nil error and MUST NOT have moved money — the provider
// leg is all-or-nothing, so the collector records nothing and leaves the invoice
// open for the dunning workflow to retry. Exactly ONE charge is attempted per
// CollectInvoice call (no double-charge within a call).
type ProviderCharger func(ctx context.Context, db *datastore.Datastore, inv *billinginvoice.BillingInvoice, amountCents int64) (providerRef string, err error)

// CollectInvoice attempts to collect payment for an invoice using the
// provider-agnostic waterfall: credits -> balance -> external provider.
// burnCredits and chargeProvider are injected to avoid circular imports; either
// may be nil (that leg is skipped).
func CollectInvoice(ctx context.Context, db *datastore.Datastore, inv *billinginvoice.BillingInvoice, burnCredits CreditBurner, chargeProvider ProviderCharger) (*CollectionResult, error) {
	if inv.Status != billinginvoice.Open {
		return nil, fmt.Errorf("invoice must be open to collect, current status: %s", inv.Status)
	}

	result := &CollectionResult{}
	remaining := inv.AmountDue

	// Step 1: Burn credits
	if remaining > 0 && burnCredits != nil {
		afterCredits, err := burnCredits(db, inv.UserId, remaining, "")
		if err != nil {
			// Non-fatal: continue without credits
			_ = err
		} else {
			result.CreditUsed = remaining - afterCredits
			remaining = afterCredits
		}
	}

	// Step 2: Deduct from transaction balance
	if remaining > 0 {
		balanceUsed, err := deductFromBalance(ctx, db, inv.UserId, inv.Currency, remaining)
		if err != nil {
			// Non-fatal: continue to external provider
			_ = err
		} else {
			result.BalanceUsed = balanceUsed
			remaining -= balanceUsed
		}
	}

	// Step 3: External provider — charge the subscription's vaulted card-on-file
	// for whatever credits + balance did not cover. Injected (ProviderCharger),
	// like burnCredits, so the engine never imports the payment/Square layer.
	if remaining > 0 && chargeProvider != nil {
		ref, err := chargeProvider(ctx, db, inv, remaining)
		if err != nil {
			// Declined / no card on file / provider error. Non-fatal here: the
			// remainder stays unpaid and the invoice is left OPEN below (the
			// dunning workflow retries it). The provider leg is all-or-nothing —
			// no partial charge is recorded, so a later retry never double-charges.
			result.Error = err.Error()
		} else {
			result.ProviderUsed = remaining
			result.ProviderRef = ref
			remaining = 0
		}
	}

	if remaining > 0 {
		// Still unpaid after credits + balance (+ any provider attempt): partial
		// failure — the invoice stays open. Preserve a provider decline message
		// when there was one, else report the shortfall.
		result.Success = false
		if result.Error == "" {
			result.Error = fmt.Sprintf("insufficient funds: %d cents remaining after credits and balance", remaining)
		}
	} else {
		result.Success = true
	}

	result.AmountCharged = inv.AmountDue - remaining

	// Update invoice
	inv.CreditApplied = result.CreditUsed
	inv.AmountPaid = result.AmountCharged
	inv.AttemptCount++
	inv.LastAttemptAt = time.Now()

	if result.Success {
		// The settling instrument, most-specific first: a card charge (the
		// provider leg) settled the remainder → "card"; a pure credit burn →
		// "credit"; otherwise the transaction balance → "balance".
		method := "balance"
		switch {
		case result.ProviderUsed > 0:
			method = "card"
		case result.CreditUsed > 0 && result.BalanceUsed == 0:
			method = "credit"
		}
		if err := inv.MarkPaid(method, result.ProviderRef); err != nil {
			return result, err
		}
	}

	return result, nil
}

// deductFromBalance withdraws the specified amount from the user's
// transaction balance if sufficient funds exist.
func deductFromBalance(ctx context.Context, db *datastore.Datastore, userId string, cur currency.Type, amount int64) (int64, error) {
	if cur == "" {
		cur = "usd"
	}

	// Check available balance
	data, err := txutil.GetTransactionsByCurrency(ctx, userId, "user", cur, false)
	if err != nil {
		return 0, err
	}

	balData, ok := data.Data[cur]
	if !ok || int64(balData.Balance) < amount {
		// Use whatever is available
		available := int64(0)
		if ok {
			available = int64(balData.Balance)
		}
		if available <= 0 {
			return 0, nil
		}
		amount = available
	}

	// Create withdrawal transaction
	tx := transaction.New(db)
	tx.Parent = db.NewKey("synckey", "", 1, nil)
	tx.Type = transaction.Withdraw
	tx.Amount = currency.Cents(amount)
	tx.Currency = cur
	tx.SourceKind = "user"
	tx.SourceId = userId
	tx.DestinationKind = "billing"
	tx.DestinationId = "invoice"
	tx.Notes = "Invoice payment"

	if err := tx.Create(); err != nil {
		return 0, fmt.Errorf("failed to create withdrawal: %w", err)
	}

	return amount, nil
}
