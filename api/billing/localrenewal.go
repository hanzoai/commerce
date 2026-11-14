package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
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

// RenewalNeedsBuyer says a renewal cannot be charged without the buyer there. A card
// issued in India cannot be charged off-session without an RBI e-mandate, and Stripe
// registers those only on its own Subscriptions, which this service does not use; so a
// renewal of an Indian card, or in rupees, whatever the amount, runs on-session: the
// buyer is sent a link and pays by card with 3-D Secure or by UPI.
func RenewalNeedsBuyer(cardCountry, cur string) bool {
	return strings.EqualFold(strings.TrimSpace(cardCountry), "IN") || strings.EqualFold(cur, "inr")
}

// requoteLocalRenewal is the part of a renewal charge that converts: it prices the
// plan's USD amount in the subscription's billing currency from a fresh quote, or, when
// the renewal cannot run without the buyer, announces renewal.action_required with the
// invoice to pay and answers errWaitingOnBuyer, which leaves the invoice open and the
// subscription past due, in the grace the dunning gives it before it suspends.
func requoteLocalRenewal(ctx context.Context, db *datastore.Datastore, org *organization.Organization, sub *subscription.Subscription, pm *paymentmethod.PaymentMethod, inv *billinginvoice.BillingInvoice, usdCents int64, planCur currency.Type, billCur string, buyerPresent bool) (*RenewalQuote, error) {
	if planCur != currency.USD {
		return nil, fmt.Errorf("subscription %s is billed in %s but its plan is priced in %s, not dollars", sub.Id(), billCur, planCur)
	}
	country := ""
	if pm.Card != nil {
		country = pm.Card.Country
	}
	if !buyerPresent && RenewalNeedsBuyer(country, billCur) {
		if _, err := engine.EmitBillingEvent(webhookGuardDB(ctx), "renewal.action_required", "subscription", sub.Id(), sub.UserId,
			map[string]interface{}{"invoiceId": inv.Id(), "pay": "/v1/billing/invoices/" + inv.Id() + "/pay", "currency": billCur, "usdCents": usdCents,
				"reason": "a card issued in India, or a payment in rupees, is charged only with the buyer present"}, nil); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: renewal of %s needs the buyer (card issued in %q, billed in %s)", errWaitingOnBuyer, sub.Id(), country, billCur)
	}
	pol, err := ReadRouting(ctx, org.Name)
	if err != nil {
		return nil, err
	}
	rq, f := RequoteRenewal(ctx, processorsForOrg(org), pol, providerOf(pm), usdCents, billCur, lastLocalAmount(db, sub, billCur), sub.UserId, sub.Id())
	if f != nil {
		return nil, fmt.Errorf("%s", f.Message)
	}
	return rq, nil
}

// lastLocalAmount is the local amount the subscription's latest paid invoice in billCur
// was charged, in minor units, or 0 when it has none.
func lastLocalAmount(db *datastore.Datastore, sub *subscription.Subscription, billCur string) int64 {
	invs := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := billinginvoice.Query(db).Filter("SubscriptionId=", sub.Id()).Filter("UserId=", sub.UserId).GetAll(&invs); err != nil {
		return 0
	}
	var at int64
	var amount int64
	for _, i := range invs {
		if i.Status != billinginvoice.Paid || i.PeriodStart.Unix() <= at {
			continue
		}
		if l, ok := i.Metadata["local"].(map[string]interface{}); ok && l["currency"] == billCur {
			if n, ok := l["amount"].(float64); ok {
				at, amount = i.PeriodStart.Unix(), int64(n)
			} else if n, ok := l["amount"].(int64); ok {
				at, amount = i.PeriodStart.Unix(), n
			}
		}
	}
	return amount
}

// bindRenewalTerms records on the invoice the terms of the attempt that paid it: the
// local amount and currency, the USD it bought, the rate and its source, and the quote
// the charge was bound to.
func bindRenewalTerms(inv *billinginvoice.BillingInvoice, q *routing.Quote) {
	if inv.Metadata == nil {
		inv.Metadata = map[string]interface{}{}
	}
	inv.Metadata["local"] = map[string]interface{}{
		"amount": q.Amount, "currency": q.Currency, "usdCents": q.UsdCents, "rate": q.Rate, "source": q.Source, "fxFee": q.FXFee,
		"quoteId": q.QuoteID, "baseRate": q.BaseRate, "feeRate": q.FeeRate, "premium": q.Premium, "exchangeRate": q.ExchangeRate,
		"lockExpiresAt": q.LockExpiresAt, "decimals": q.Decimals,
	}
}

// saleQuote is the locked quote a first charge is made at, when the sale is to be billed
// in the buyer's currency, or nil to keep the plan's. It is nil when the policy resolves
// dollars for the country, when local currency is off, or when Stripe cannot present
// the currency; an error only when the buyer was shown a price the locked quote does
// not match, which they must see and confirm before anything is charged.
func saleQuote(ctx context.Context, org *organization.Organization, in SubscribeIn, usdCents int64) (*routing.Quote, error) {
	pol, err := ReadRouting(ctx, org.Name)
	if err != nil || !pol.LocalOn || pol.LocalCurrency(in.Country) == "usd" {
		return nil, nil
	}
	q, f := quoteVia(ctx, processorsForOrg(org), pol, processor.Stripe, usdCents, in.Country, "", "", true)
	if f != nil || q.Currency == "usd" {
		return nil, nil
	}
	if in.QuotedCurrency != "" && (!strings.EqualFold(in.QuotedCurrency, q.Currency) || in.QuotedAmount != q.Amount) {
		return nil, fmt.Errorf("the price is now %d %s (minor units, %d decimals); confirm it to continue", q.Amount, q.Currency, q.Decimals)
	}
	return q, nil
}

// quoteOfIntent is the quote an attempt was opened on, as its record keeps it.
func quoteOfIntent(p *pendingintent.Intent) *routing.Quote {
	dec := int(currency.Type(p.PaidCurrency).Decimals())
	return &routing.Quote{Amount: p.PaidCents, Decimals: dec, Currency: p.PaidCurrency, UsdCents: p.Cents, Rate: p.Rate, Source: p.RateSource, FXFee: p.FXFee,
		ExchangeRate: p.ExchangeRate, BaseRate: p.BaseRate, FeeRate: p.FeeRate, Premium: p.Premium, LockExpiresAt: p.LockExpiresAt,
		QuoteID: p.QuoteID, ReferenceRate: p.ReferenceRate, ReferenceProvider: p.ReferenceProvider, Disclosure: routing.Disclosure}
}

// registerQuotedAction records the intent a buyer is sent to their bank for when the
// charge was made in their own currency: the credit is the plan's dollars, the amount
// paid the quote, and the terms the quote was made on are bound to it.
func registerQuotedAction(result *processor.PaymentResult, err error, org *organization.Organization, subject, purpose string, usdCents int64, q *routing.Quote) error {
	ar := actionOf(result, err)
	if ar == nil || !strings.HasPrefix(ar.ID, "pi_") {
		return nil
	}
	if cerr := atCap(subject); cerr != nil {
		return cerr
	}
	if perr := pendingintent.PutQuoted(ar.ID, org.Name, strings.ToLower(subject), purpose, usdCents, "usd", q.Amount, q.Currency, 0, org.TestMode()); perr != nil {
		log.Error("RECONCILE: the 3-D Secure intent %s for %s could not be recorded: %v", ar.ID, subject, perr)
		return errors.New("billing is temporarily unavailable; please retry")
	}
	return pendingintent.Bind(ar.ID, pendingintent.Terms{Rate: q.Rate, RateSource: q.Source, QuoteID: q.QuoteID, BaseRate: q.BaseRate, FeeRate: q.FeeRate, Premium: q.Premium,
		ExchangeRate: q.ExchangeRate, ReferenceRate: q.ReferenceRate, ReferenceProvider: q.ReferenceProvider, FXFee: q.FXFee, LockExpiresAt: q.LockExpiresAt})
}
