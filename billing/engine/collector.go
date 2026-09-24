package engine

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
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

// Prepaid is the money a subject paid in advance: its credit grants, then its
// balance on the one ledger the host keeps. The collector draws from it before
// it touches a card.
//
// It is injected, like ProviderCharger, because both halves live above the
// engine — the grants in api/billing, the balance on whichever ledger the host
// installed — and the engine must not import either.
type Prepaid interface {
	// Available is what Draw could take from the subject right now. An error is
	// unknown, never zero.
	Available(ctx context.Context, subject string, cur currency.Type) (int64, error)
	// Draw takes exactly amount, credits first and then the balance, or takes
	// nothing and says why. ref names the act the money pays for, so a retry
	// under the same ref moves no money twice.
	Draw(ctx context.Context, subject string, cur currency.Type, amount int64, ref string) (Drawn, error)
}

// Drawn is what one prepaid draw took, and from where.
type Drawn struct {
	// Credit is the part credit grants covered.
	Credit int64
	// Balance is the part the balance covered.
	Balance int64
	// Ref is the ledger's id for the balance posting. Empty when credits covered
	// the whole draw.
	Ref string
}

// ProviderCharger charges amountCents (the part of the invoice prepaid money does
// not cover) on the external payment provider — the subscription's vaulted
// card-on-file — and returns the processor reference on success.
//
// It is injected exactly like Prepaid so the engine never imports
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

// CollectInvoice collects what an open invoice still owes through the one
// waterfall: prepaid money first (credits, then the balance), the card on file
// for the rest. Either may be nil, and that leg is skipped.
//
// IT IS ALL OR NOTHING AGAINST THE PREPAID MONEY. The allocation is read first
// and the card, the one leg that can refuse for reasons of its own, is charged
// before anything prepaid moves. A decline, or no card at all, therefore leaves
// the credits and the balance where they were. Drawn the other way round, a
// failed collection had already spent them on an invoice that stayed unpaid, and
// every retry spent them again.
//
// What an attempt does collect accumulates on AmountPaid, so a later attempt
// collects only the remainder.
func CollectInvoice(ctx context.Context, db *datastore.Datastore, inv *billinginvoice.BillingInvoice, prepaid Prepaid, chargeProvider ProviderCharger) (*CollectionResult, error) {
	if inv.Status != billinginvoice.Open {
		return nil, fmt.Errorf("invoice must be open to collect, current status: %s", inv.Status)
	}

	result := &CollectionResult{}
	ref := ""
	owed := inv.AmountDue - inv.AmountPaid
	if owed < 0 {
		owed = 0
	}

	// The allocation: prepaid covers what it holds, the card the rest. A balance
	// that cannot be read is unknown, so it covers nothing and the card is asked
	// for the whole amount rather than the draw being guessed at.
	fromPrepaid := int64(0)
	if owed > 0 && prepaid != nil {
		avail, err := prepaid.Available(ctx, inv.UserId, inv.Currency)
		switch {
		case err != nil:
			result.Error = "prepaid balance unreadable: " + err.Error()
		case avail > 0:
			fromPrepaid = min(avail, owed)
		}
	}

	if rest := owed - fromPrepaid; rest > 0 {
		if chargeProvider == nil {
			return unpaid(inv, result, owed)
		}
		charged, err := chargeProvider(ctx, db, inv, rest)
		if err != nil {
			// Declined, no card on file, or a provider error. The provider leg is
			// all or nothing and nothing prepaid has moved, so the invoice stays
			// open exactly as it was for the next attempt.
			result.Error = err.Error()
			if errors.Is(err, processor.ErrUnknownOutcome) {
				// The provider did not state an outcome: it failed to answer, or the
				// payment is still processing, and the money may yet move. The
				// attempt count is part of the gateway key, so it is left where it
				// is and the next attempt reaches the provider under the same key,
				// where it is answered with this payment instead of taking a second.
				inv.LastAttemptAt = time.Now()
				return result, nil
			}
			return unpaid(inv, result, owed)
		}
		result.ProviderUsed, result.ProviderRef = rest, charged
	}

	if fromPrepaid > 0 {
		ref = drawRef(inv)
		d, err := prepaid.Draw(ctx, inv.UserId, inv.Currency, fromPrepaid, ref)
		if err != nil {
			// Reachable only when the balance moved between the read and the draw.
			// Whatever the card took stands as a partial payment; the rest stays owed.
			result.Error = "prepaid draw failed: " + err.Error()
		} else {
			result.CreditUsed, result.BalanceUsed = d.Credit, d.Balance
			if d.Ref != "" {
				ref = d.Ref
			}
		}
	}

	paid := result.ProviderUsed + result.CreditUsed + result.BalanceUsed
	result.AmountCharged = paid
	inv.AmountPaid += paid
	inv.CreditApplied += result.CreditUsed
	inv.AttemptCount++
	inv.LastAttemptAt = time.Now()

	if paid < owed {
		if result.Error == "" {
			result.Error = fmt.Sprintf("insufficient funds: %d cents remaining after credits and balance", owed-paid)
		}
		return result, nil
	}

	// The settling instrument, most specific first: a card charge settled the
	// remainder, credits alone covered it, or the balance did.
	method := "balance"
	switch {
	case result.ProviderUsed > 0:
		method = "card"
	case result.CreditUsed > 0 && result.BalanceUsed == 0:
		method = "credit"
	}
	if result.ProviderRef != "" {
		ref = result.ProviderRef
	}
	result.Success, result.Error = true, ""
	if err := inv.MarkPaid(method, ref); err != nil {
		return result, err
	}
	return result, nil
}

// unpaid records an attempt that moved no money and says what is still owed.
func unpaid(inv *billinginvoice.BillingInvoice, result *CollectionResult, owed int64) (*CollectionResult, error) {
	inv.AttemptCount++
	inv.LastAttemptAt = time.Now()
	if result.Error == "" {
		result.Error = fmt.Sprintf("insufficient funds: %d cents remaining after credits and balance", owed)
	}
	return result, nil
}

// drawRef names the prepaid draw that pays one invoice. It is the draw's
// idempotency key, and it names what the invoice bills — a subscription's
// period — not the invoice row or the attempt: a draw whose posting landed but
// whose answer was lost is found again by the next attempt, and two processes that
// each raised an invoice for the same period draw once between them. A period is
// paid from prepaid money at most once, because the card is charged before the
// draw and a draw that lands settles the invoice. An invoice for no subscription
// is its own act.
func drawRef(inv *billinginvoice.BillingInvoice) string {
	if inv.SubscriptionId == "" {
		return "invoice:" + inv.Id()
	}
	return "sub:" + inv.SubscriptionId + ":period:" + strconv.FormatInt(inv.PeriodStart.Unix(), 10)
}
