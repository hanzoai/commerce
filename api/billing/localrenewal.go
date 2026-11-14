package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/routing"
	types "github.com/hanzoai/commerce/types"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
	"github.com/zap-proto/zip"
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
func RequoteRenewal(ctx context.Context, reg *processor.Registry, pol routing.Policy, via processor.ProcessorType, usdCents int64, cur string, previous int64, subject, subscription, invoice string) (*RenewalQuote, *PaymentFault) {
	q, f := quoteVia(ctx, reg, pol, via, usdCents, "", cur, "", true)
	if f != nil {
		return nil, f
	}
	out := &RenewalQuote{Quote: q, Previous: previous, Changed: previous > 0 && previous != q.Amount}
	if out.Changed {
		if _, err := engine.EmitBillingEvent(webhookGuardDB(ctx), "renewal.amount_changed", "subscription", subscription, subject, renewalChange(subscription, invoice, previous, q), nil); err != nil {
			return out, fault(503, "the renewal notice could not be recorded", err)
		}
	}
	return out, nil
}

// invoicePath and invoicePayPath are where a renewal's invoice is read and paid, built
// from its id, so a notice can link to either.
func invoicePath(id string) string    { return "/v1/billing/invoices/" + id }
func invoicePayPath(id string) string { return invoicePath(id) + "/pay" }

// renewalChange is the data of renewal.amount_changed: the invoice and subscription,
// the amount the buyer paid last time and the amount due now in the same currency, the
// quote the new amount is bound to, and the paths to read or pay the invoice, so the
// notification renders from the event alone.
func renewalChange(subscription, invoice string, previous int64, q *routing.Quote) map[string]interface{} {
	return map[string]interface{}{
		"subscriptionId": subscription, "invoiceId": invoice, "currency": q.Currency, "decimals": q.Decimals,
		"previous": previous, "amount": q.Amount, "usdCents": q.UsdCents,
		"quote": q, "quoteId": q.QuoteID, "rate": q.Rate, "source": q.Source, "fxFee": q.FXFee, "lockExpiresAt": q.LockExpiresAt,
		"disclosure": q.Disclosure, "payPath": invoicePayPath(invoice), "invoicePath": invoicePath(invoice),
	}
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
			map[string]interface{}{"invoiceId": inv.Id(), "subscriptionId": sub.Id(), "payPath": invoicePayPath(inv.Id()), "invoicePath": invoicePath(inv.Id()), "currency": billCur, "usdCents": usdCents,
				"reason": "a card issued in India, or a payment in rupees, is charged only with the buyer present"}, nil); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: renewal of %s needs the buyer (card issued in %q, billed in %s)", errWaitingOnBuyer, sub.Id(), country, billCur)
	}
	if buyerPresent {
		// The buyer was sent to their bank for this invoice at a quote, and the quote still
		// holds: they finish at the amount they saw, not at a new one.
		if open := finishedInvoiceIntent(inv); open != nil && open.PaidCurrency == billCur && open.QuoteID != "" &&
			open.LockExpiresAt > time.Now().Unix()+30 {
			return &RenewalQuote{Quote: quoteOfIntent(open)}, nil
		}
	}
	pol, err := ReadRouting(ctx, org.Name)
	if err != nil {
		return nil, err
	}
	rq, f := RequoteRenewal(ctx, processorsForOrg(org), pol, providerOf(pm), usdCents, billCur, lastLocalAmount(db, sub, billCur), sub.UserId, sub.Id(), inv.Id())
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
	// The quote as its own JSON, so the invoice reads back as the same typed quote.
	var m map[string]interface{}
	if b, err := json.Marshal(q); err == nil && json.Unmarshal(b, &m) == nil {
		inv.Metadata["local"] = m
	}
}

// quoteOfInvoice is the quote bound to a PAID invoice, or nil.
func quoteOfInvoice(inv *billinginvoice.BillingInvoice) *routing.Quote {
	l, ok := inv.Metadata["local"].(map[string]interface{})
	if !ok {
		return nil
	}
	b, err := json.Marshal(l)
	if err != nil {
		return nil
	}
	var q routing.Quote
	if json.Unmarshal(b, &q) != nil || q.Currency == "" {
		return nil
	}
	return &q
}

// invoiceQuote is the invoice's price in the buyer's currency, for the page that shows
// it: the quote the payment was bound to once paid; the one an attempt waiting on the
// buyer's bank is bound to; else, for an open invoice of a subscription billed in a
// currency of its own, the current displayed price of what is due. Nil when the invoice
// is in the plan's currency.
func invoiceQuote(ctx context.Context, org *organization.Organization, db *datastore.Datastore, inv *billinginvoice.BillingInvoice) *routing.Quote {
	if q := quoteOfInvoice(inv); q != nil {
		return q
	}
	if open := finishedInvoiceIntent(inv); open != nil && open.PaidCurrency != "" {
		q := quoteOfIntent(open)
		q.Disclosure = routing.Disclosure
		return q
	}
	if inv.Status != billinginvoice.Open || inv.SubscriptionId == "" || inv.Currency != currency.USD || inv.AmountDue <= 0 {
		return nil
	}
	sub := subscription.New(db)
	if err := sub.GetById(inv.SubscriptionId); err != nil || sub.BillingCurrency == "" {
		return nil
	}
	pol, err := ReadRouting(ctx, org.Name)
	if err != nil {
		return nil
	}
	q, f := quoteVia(ctx, processorsForOrg(org), pol, processor.Stripe, inv.AmountDue, "", sub.BillingCurrency, "", false)
	if f != nil {
		return nil
	}
	return q
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
func registerQuotedAction(result *processor.PaymentResult, err error, org *organization.Organization, subject, purpose, ref string, usdCents int64, q *routing.Quote) error {
	ar := actionOf(result, err)
	if ar == nil || !strings.HasPrefix(ar.ID, "pi_") {
		return nil
	}
	if cerr := atCap(subject); cerr != nil {
		return cerr
	}
	if perr := pendingintent.PutQuotedRef(ar.ID, org.Name, strings.ToLower(subject), purpose, ref, usdCents, "usd", q.Amount, q.Currency, 0, org.TestMode()); perr != nil {
		log.Error("RECONCILE: the 3-D Secure intent %s for %s could not be recorded: %v", ar.ID, subject, perr)
		return errors.New("billing is temporarily unavailable; please retry")
	}
	return pendingintent.Bind(ar.ID, pendingintent.Terms{Rate: q.Rate, RateSource: q.Source, QuoteID: q.QuoteID, BaseRate: q.BaseRate, FeeRate: q.FeeRate, Premium: q.Premium,
		ExchangeRate: q.ExchangeRate, ReferenceRate: q.ReferenceRate, ReferenceProvider: q.ReferenceProvider, FXFee: q.FXFee, LockExpiresAt: q.LockExpiresAt})
}

// QuoteIn is a price asked for before any sale: a plan's first charge, or an amount of
// credit.
type QuoteIn struct {
	Subject string
	// Plan is the plan to price (as subscribe takes it, with Level and Interval); empty
	// when UsdCents asks for a top-up.
	Plan     string
	Level    int
	Interval types.Interval
	UsdCents int64
	Country  string
	// Currency is the currency the buyer chose; empty takes the country's default.
	Currency string
	Method   string
}

// QuoteBeforeSale is the neutral quote for what the buyer is about to buy, made with the
// same policy, rounding and source as subscribe and top-up: the plan's first charge
// priced as the sale prices it (promotion and coupon rules included), or the amount of
// credit asked for, in the buyer's currency on the processor the route picks. It uses
// the displayed rate (cached); the sale locks its own quote and refuses if the buyer
// was shown an amount it no longer matches.
func QuoteBeforeSale(ctx context.Context, org *organization.Organization, in QuoteIn) (*routing.Quote, *PaymentFault) {
	usd := in.UsdCents
	if in.Plan != "" {
		pr, err := price(datastore.New(org.Namespaced(ctx)), org, SubscribeIn{PlanID: in.Plan, Level: in.Level, Interval: in.Interval, Subject: in.Subject}, in.Plan, "")
		if err != nil {
			return nil, fault(400, err.Error(), err)
		}
		usd = pr.charge
	}
	if usd <= 0 {
		return nil, fault(400, "name a plan, or an amount in usdCents", nil)
	}
	d, f := place(ctx, org, RouteIn{Subject: in.Subject, Country: in.Country, Currency: in.Currency, QuoteCurrency: in.Currency, AmountCents: 0, Method: in.Method}, false)
	if f != nil {
		return nil, f
	}
	pol, err := ReadRouting(ctx, org.Name)
	if err != nil {
		return nil, fault(503, "routing is temporarily unavailable; please retry", err)
	}
	if !pol.LocalOn {
		return nativeQuote(usd), nil
	}
	return quoteVia(ctx, processorsForOrg(org), pol, d.Processor, usd, d.Country, in.Currency, in.Method, false)
}

// QuoteHandler answers the quote before a sale.
//
//	GET /v1/billing/quote?plan=<id>[&level=&interval=]&country=<cc>[&currency=&method=]
//	GET /v1/billing/quote?usdCents=<n>&country=<cc>
func QuoteHandler(c *zip.Ctx) error {
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return jsonhttp.Fail(c, 401, "sign in to see a price", nil)
	}
	country := strings.TrimSpace(c.Query("country"))
	if country == "" {
		country = requestCountry(c)
	}
	cents, _ := strconv.ParseInt(strings.TrimSpace(c.Query("usdCents")), 10, 64)
	level, _ := strconv.Atoi(strings.TrimSpace(c.Query("level")))
	q, f := QuoteBeforeSale(c.Context(), org, QuoteIn{Subject: userBillingKey(c), Plan: strings.TrimSpace(c.Query("plan")), Level: level,
		Interval: types.Interval(strings.TrimSpace(c.Query("interval"))), UsdCents: cents, Country: country,
		Currency: strings.TrimSpace(c.Query("currency")), Method: strings.TrimSpace(c.Query("method"))})
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, q)
}
