package engine

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/types/currency"
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
	// Unknown is set when the processor did not say whether the money moved
	// (ErrChargeUnknown). The attempt is not counted, so it is repeated under the
	// same idempotency key.
	Unknown bool `json:"unknown,omitempty"`
}

// ErrChargeUnknown marks a payment whose outcome the processor or the ledger did
// not report: the request failed in transit, the processor could not be reached,
// or it answered with anything but a refusal. The money may have moved. A
// ProviderCharger or a Collection wraps it (errors.Is) so the attempt is repeated
// under the SAME idempotency key, which answers with the first payment if there
// was one, instead of counting a decline and charging again under a new key.
var ErrChargeUnknown = errors.New("charge outcome unknown")

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
	// nothing and says why: an error wrapping ErrPrepaidShort when the money
	// cannot cover it. ref names the act the money pays for, so a retry under the
	// same ref moves no money twice.
	Draw(ctx context.Context, subject string, cur currency.Type, amount int64, ref string) (Drawn, error)
	// Return puts amount back on the subject's balance, once per ref: a retry
	// under the same ref returns nothing more. It answers the ledger's id for the
	// posting.
	Return(ctx context.Context, subject string, cur currency.Type, amount int64, ref string) (string, error)
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
// means "no external provider" — credits + balance only, so an unpaid remainder
// leaves the invoice OPEN.
//
// Fail-closed contract: on a decline / missing card / provider error the
// callback returns a non-nil error and MUST NOT have moved money — the provider
// leg is all-or-nothing, so the collector records nothing and leaves the invoice
// open for the next attempt. An outcome the processor did not report wraps
// ErrChargeUnknown. Exactly ONE charge is attempted per call (no double-charge
// within a call).
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
		if err != nil && errors.Is(err, ErrChargeUnknown) {
			// The card may have been charged. The attempt is not counted, so the
			// next one carries this attempt's key and gets its answer back.
			result.Error, result.Unknown = err.Error(), true
			inv.LastAttemptAt = time.Now()
			return result, nil
		}
		if err != nil {
			// Declined, no card on file, or a provider error. The provider leg is
			// all or nothing and nothing prepaid has moved, so the invoice stays
			// open exactly as it was for the next attempt.
			result.Error = err.Error()
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

// AttemptKey is the idempotency key of inv's current payment attempt: the key
// recorded as pending when there is one, else the key of the next attempt,
// scoped to (subscription, billed period, attempt number) — or to the invoice
// for one that bills no subscription. The attempt number is AttemptCount before
// the attempt is counted, so a retry after a decline is a new key while a repeat
// of an attempt whose outcome was not known is the same one.
func AttemptKey(inv *billinginvoice.BillingInvoice) string {
	if inv.PendingKey != "" {
		return inv.PendingKey
	}
	if inv.SubscriptionId == "" {
		return "collect:inv:" + inv.Id() + ":attempt:" + strconv.Itoa(inv.AttemptCount)
	}
	return "collect:sub:" + inv.SubscriptionId +
		":period:" + strconv.FormatInt(inv.PeriodStart.Unix(), 10) +
		":attempt:" + strconv.Itoa(inv.AttemptCount)
}

// Pay makes one payment attempt on an OPEN invoice the caller holds the lock of
// (LockInvoice), for all it still owes, through c and nothing else — so a
// subscription's invoice is paid the way the subscription was bought, and never
// from money the customer holds for something else.
//
// The attempt is written to the invoice before any money is asked for: the
// instrument c.Choose picked, the amount, and the key. An attempt that finds one
// already written repeats it exactly, so an answer lost to a crash, a timeout or
// a 5xx is looked up under its own key rather than sent again under a new one,
// and no path can ask for a different amount under a key already used. A known
// outcome clears it; a decline counts the attempt. The caller persists the
// invoice after Pay returns.
func Pay(ctx context.Context, db *datastore.Datastore, inv *billinginvoice.BillingInvoice, c Collection, now time.Time) (*CollectionResult, error) {
	if inv.Status != billinginvoice.Open {
		return nil, fmt.Errorf("invoice must be open to collect, current status: %s", inv.Status)
	}
	result := &CollectionResult{}
	if inv.PendingKey == "" {
		owed := inv.AmountDue - inv.AmountPaid
		if owed <= 0 {
			return result, markPaid(inv, inv.PaymentMethod, "", now, result)
		}
		if c.Choose == nil || c.Pay == nil {
			result.Error = "nothing on file pays this invoice"
			inv.AttemptCount++
			inv.LastAttemptAt = now
			return result, nil
		}
		method, err := c.Choose(ctx, db, inv, owed)
		if err != nil && errors.Is(err, ErrChargeUnknown) {
			// Choose could not tell (a ledger read failed). Nothing was asked for, so
			// nothing is recorded or counted; the next run chooses again.
			result.Error, result.Unknown = err.Error(), true
			return result, nil
		}
		if err != nil {
			result.Error = err.Error()
			inv.AttemptCount++
			inv.LastAttemptAt = now
			return result, nil
		}
		inv.PendingMethod, inv.PendingAmount, inv.PendingKey, inv.PendingRef = method, owed, AttemptKey(inv), ""
		inv.LastAttemptAt = now
		if err := inv.Update(); err != nil {
			return nil, fmt.Errorf("record the attempt on invoice %s before paying: %w", inv.Id(), err)
		}
	}

	ref, err := c.Pay(ctx, db, inv, inv.PendingMethod, inv.PendingAmount)
	inv.LastAttemptAt = now
	switch {
	case err != nil && errors.Is(err, ErrChargeUnknown):
		result.Error, result.Unknown = err.Error(), true
		if ref != "" {
			inv.PendingRef = ref
		}
		return result, nil
	case err != nil:
		result.Error = err.Error()
		clearPending(inv)
		inv.AttemptCount++
		return result, nil
	}
	method, amount := inv.PendingMethod, inv.PendingAmount
	clearPending(inv)
	inv.AttemptCount++
	inv.AmountPaid += amount
	result.AmountCharged, result.ProviderRef = amount, ref
	if method == PaidByCard {
		result.ProviderUsed = amount
	} else {
		result.BalanceUsed = amount
	}
	if inv.AmountPaid < inv.AmountDue {
		return result, nil
	}
	return result, markPaid(inv, method, ref, now, result)
}

// PayInvoice is the customer paying a subscription's open invoice themselves:
// one attempt through c (Pay), on the same recorded attempt the billing cycle
// uses, and persisted. A decline moves the next scheduled retry to the first
// point of RetrySchedule still ahead; an unknown outcome is left for the next
// run to repeat under its key. The subscription moves onto the paid period at
// the cycle's next run. The caller holds the invoice's lock (LockInvoice).
func PayInvoice(ctx context.Context, db *datastore.Datastore, inv *billinginvoice.BillingInvoice, c Collection, now time.Time) (*CollectionResult, error) {
	res, err := Pay(ctx, db, inv, c, now)
	if err != nil {
		return nil, err
	}
	switch {
	case res.Unknown:
		inv.NextAttemptAt = now
	case !res.Success && !inv.DueDate.IsZero():
		if next := nextRetry(inv.DueDate, now); !next.IsZero() {
			inv.NextAttemptAt = next
		}
	}
	if err := inv.Update(); err != nil {
		return res, fmt.Errorf("record the payment on invoice %s: %w", inv.Id(), err)
	}
	return res, nil
}

// markPaid marks inv paid by method at now.
func markPaid(inv *billinginvoice.BillingInvoice, method, ref string, now time.Time, result *CollectionResult) error {
	if err := inv.MarkPaid(method, ref); err != nil {
		return err
	}
	inv.PaidAt = now
	result.Success = true
	return nil
}

func clearPending(inv *billinginvoice.BillingInvoice) {
	inv.PendingMethod, inv.PendingAmount, inv.PendingKey, inv.PendingRef = "", 0, "", ""
}

// The instruments a subscription's invoice is paid with, as Collection.Choose
// names them and the paid invoice records them.
const (
	PaidByCard    = "card"
	PaidByPrepaid = "prepaid"
)

// CardPayer is a Collection that pays by the subscription's card through charge.
func CardPayer(charge ProviderCharger) Collection {
	return Collection{
		Choose: func(context.Context, *datastore.Datastore, *billinginvoice.BillingInvoice, int64) (string, error) {
			return PaidByCard, nil
		},
		Pay: func(ctx context.Context, db *datastore.Datastore, inv *billinginvoice.BillingInvoice, _ string, amount int64) (string, error) {
			return charge(ctx, db, inv, amount)
		},
	}
}

// PrepaidPayer is a Collection that pays from the subscriber's prepaid money
// through p: its credit grants, then its balance, all or nothing. The draw's
// ref is the period's (drawRef), so a period is drawn at most once however many
// attempts or invoices ask for it, and a repeat of an attempt whose answer was
// lost gets the first draw back. A draw the money cannot cover is a decline;
// any other failure is ErrChargeUnknown.
func PrepaidPayer(p Prepaid) Collection {
	return Collection{
		Choose: func(ctx context.Context, _ *datastore.Datastore, inv *billinginvoice.BillingInvoice, amount int64) (string, error) {
			avail, err := p.Available(ctx, inv.UserId, invoiceCurrency(inv))
			if err != nil {
				return "", fmt.Errorf("%w: read the prepaid money: %v", ErrChargeUnknown, err)
			}
			if avail < amount {
				return "", fmt.Errorf("%w: prepaid money holds %d cents, the renewal is %d", ErrPrepaidShort, avail, amount)
			}
			return PaidByPrepaid, nil
		},
		Pay: func(ctx context.Context, _ *datastore.Datastore, inv *billinginvoice.BillingInvoice, _ string, amount int64) (string, error) {
			ref := drawRef(inv)
			d, err := p.Draw(ctx, inv.UserId, invoiceCurrency(inv), amount, ref)
			switch {
			case errors.Is(err, ErrPrepaidShort):
				return "", err
			case err != nil:
				return "", fmt.Errorf("%w: %v", ErrChargeUnknown, err)
			}
			inv.CreditApplied += d.Credit
			if d.Ref != "" {
				return d.Ref, nil
			}
			return ref, nil
		},
	}
}

// ErrPrepaidShort is a draw the subject's prepaid money cannot cover. It moved
// nothing. A Prepaid answers a short draw with an error wrapping it: the one
// ledger's own ErrShort.
var ErrPrepaidShort = creditledger.ErrShort

func invoiceCurrency(inv *billinginvoice.BillingInvoice) currency.Type {
	if inv.Currency == "" {
		return currency.USD
	}
	return inv.Currency
}
