package billing

import (
	"context"
	"fmt"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/routing"
)

// RenewalQuote is a renewal priced afresh: the plan's USD price converted at a rate
// quoted now and bound to this attempt, and whether the buyer's local amount moved.
type RenewalQuote struct {
	Quote    *routing.Quote
	Previous int64 // the local amount last charged, in minor units; 0 when none
	Changed  bool
}

// RequoteRenewal prices a renewal of usdCents in cur from a fresh, locked quote, so the
// local amount follows the exchange rate from one period to the next while the credit
// stays the plan's USD price. When the amount differs from the last charge it emits
// renewal.amount_changed, with both amounts and the rate, for the notification to show.
func RequoteRenewal(ctx context.Context, reg *processor.Registry, pol routing.Policy, via processor.ProcessorType, usdCents int64, cur string, previous int64, subject, subscription string) (*RenewalQuote, *PaymentFault) {
	q, f := quoteVia(ctx, reg, pol, via, usdCents, "", cur, "", true)
	if f != nil {
		return nil, f
	}
	out := &RenewalQuote{Quote: q, Previous: previous, Changed: previous > 0 && previous != q.Amount}
	if out.Changed {
		if _, err := engine.EmitBillingEvent(webhookGuardDB(ctx), "renewal.amount_changed", "subscription", subscription, subject,
			map[string]interface{}{"currency": q.Currency, "previous": previous, "amount": q.Amount, "usdCents": usdCents, "rate": q.Rate, "source": q.Source, "quoteId": q.QuoteID, "decimals": q.Decimals}, nil); err != nil {
			return out, fault(503, "the renewal notice could not be recorded", err)
		}
	}
	return out, nil
}

// indiaNoAuthLimit is the RBI ceiling, in paise, for a recurring charge that may run
// without the cardholder authenticating it (Rs 15,000).
const indiaNoAuthLimit int64 = 15000 * 100

// mandateHeadroomBps is the room a mandate's maximum leaves over the renewal's local
// amount for the exchange rate to move before the next period: a ceiling the bank holds
// the merchant to, never part of a price.
const mandateHeadroomBps = 1000

// RenewalMode says how a renewal of localMinor in cur may be charged: off-session on the
// mandate, or only with the buyer present, who is then sent a pay link. Under the RBI's
// rules a recurring charge above Rs 15,000 needs the buyer's authentication each time.
func RenewalMode(cur string, localMinor int64) (offSession bool) {
	return !(cur == "inr" && localMinor > indiaNoAuthLimit)
}

// MandateMax is the maximum a card mandate is registered for: the renewal's local
// amount plus headroom, rounded up to the increment.
func MandateMax(localMinor, step int64) int64 {
	m := (localMinor*(10000+mandateHeadroomBps) + 9999) / 10000 // exact, rounded up
	return (m + step - 1) / step * step
}

// PayLinkEvent is emitted instead of an off-session attempt when the renewal needs the
// buyer: it carries what the notification needs to ask them to pay.
func PayLinkEvent(ctx context.Context, subject, subscription, cur string, localMinor int64) error {
	_, err := engine.EmitBillingEvent(webhookGuardDB(ctx), "renewal.pay_link", "subscription", subscription, subject,
		map[string]interface{}{"currency": cur, "amount": localMinor, "reason": fmt.Sprintf("a renewal over %d paise needs the buyer's authentication", indiaNoAuthLimit)}, nil)
	return err
}
