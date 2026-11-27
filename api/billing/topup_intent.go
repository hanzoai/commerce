package billing

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/api/promo"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/checkoutsession"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment"
	"github.com/hanzoai/commerce/payment/fx"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/routing"
	types "github.com/hanzoai/commerce/types"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// TopupIntentIn is a signed-in buyer opening a Stripe checkout for a top-up of
// AmountCents US dollars (or a plan's first period when PlanID is non-empty),
// paying with whatever their country offers.
type TopupIntentIn struct {
	Subject        string
	Country        string
	AmountCents    int64
	IdempotencyKey string
	// Currency is the currency the buyer chose to pay in, if they chose one; empty takes
	// the country's default. Method is the method they chose, if they have: a currency
	// the method cannot be paid in is refused.
	Currency, Method string
	// Payer is who is paying; the intent is made for their customer, with their
	// receipt email.
	Payer PayerIn
	// Optional plan fields when paying a plan's first period via bank transfer (ACH).
	PlanID   string
	Quantity int
	Level    int
	Interval types.Interval
	Coupon   string
	Promo    *promo.Promo
}

// TopupIntentOut is what the browser mounts the Payment Element with, and what the
// buyer is shown: Amount of Currency buys UsdCents of credit, at Rate per dollar.
type TopupIntentOut struct {
	IntentID       string  `json:"intentId"`
	ClientSecret   string  `json:"clientSecret"`
	PublishableKey string  `json:"publishableKey"`
	Currency       string  `json:"currency"`
	Amount         int64   `json:"amount"`
	UsdCents       int64   `json:"usdCents"`
	Rate           float64 `json:"rate"`
	// Quote is the disclosure the buyer authorizes: the amount confirm will charge, its
	// currency, the rate, the markup and the dollars of credit it buys.
	Quote routing.Quote `json:"quote"`
	// PolicyVersion is the routing policy version the payment was opened under.
	PolicyVersion int `json:"policyVersion"`
}

// OpenTopupIntent opens a PaymentIntent for the Payment Element, in the currency
// the buyer's country needs, and records BEFORE it answers whose it is, what it is
// for, and the quote: the buyer pays the local amount and the wallet is credited
// the dollars. Nothing is credited here. The credit is posted when Stripe reports
// the intent succeeded (the verified webhook, or the reconciler for one that
// missed it), once, for exactly the recorded quote.
func OpenTopupIntent(ctx context.Context, org *organization.Organization, in TopupIntentIn) (*TopupIntentOut, *PaymentFault) {
	if org == nil || in.Subject == "" {
		return nil, fault(401, "sign in to add funds", nil)
	}
	// A caller may name the bank debit "ach": the core speaks the policy's name for it.
	in.Method = policyMethod(in.Method)
	planSlug := strings.TrimSpace(in.PlanID)
	planInterval := types.Monthly
	if planSlug != "" {
		db := datastore.New(org.Namespaced(ctx))
		subIn := SubscribeIn{
			SourceID: "ach-plan-quote",
			PlanID:   planSlug,
			Subject:  in.Subject,
			Quantity: in.Quantity,
			Level:    in.Level,
			Interval: in.Interval,
			Wallet:   true,
			Promo:    in.Promo,
		}
		q, perr := price(db, org, subIn, planSlug, subIn.SourceID)
		if perr != nil {
			return nil, saleErrToFault(perr)
		}
		planSlug = q.slug
		planInterval = q.plan.Interval
		chargeCents := q.charge
		if raw := strings.TrimSpace(in.Coupon); raw != "" {
			couponCode := promo.CouponCode(raw)
			if couponCode == "" || !engine.BillsMonthly(q.plan) {
				return nil, fault(400, "coupon is not valid for this plan", nil)
			}
			offer, reason := promo.FindCoupon(ctx, couponCode, time.Now())
			if reason == "" && !everyLine(q.slug, q.lines, offer.AppliesTo) {
				reason = promo.CouponNotApplicable
			}
			if reason != "" {
				return nil, fault(400, couponRefusal(couponCode, q.slug, reason), nil)
			}
			if promo.Redeemed(ctx, couponCode, org.Name) {
				return nil, fault(409, fmt.Sprintf("%v (%s)", promo.ErrCouponRedeemedByOrg, couponCode), nil)
			}
			chargeCents = engine.Discounted(chargeCents, offer.Coupon().Percent)
			if chargeCents <= 0 {
				return nil, fault(400, "coupon leaves nothing to charge", nil)
			}
		}
		in.AmountCents = chargeCents
	} else {
		minCents, maxCents := topupBounds()
		if in.AmountCents < minCents || in.AmountCents > maxCents {
			return nil, fault(400, fmt.Sprintf("amountCents must be between %d and %d", minCents, maxCents), nil)
		}
	}
	d, f := Place(ctx, org, RouteIn{Subject: in.Subject, Country: in.Country, Method: in.Method, Currency: in.Currency})
	if f != nil {
		return nil, f
	}
	if d.Processor != processor.Stripe {
		return nil, fault(409, "this checkout is on "+string(d.Processor), nil)
	}
	pol, polVersion, err := ReadRoutingVersioned(ctx, org.Name)
	if err != nil {
		return nil, fault(503, "routing is temporarily unavailable; please retry", err)
	}
	if !pol.LocalOn || (d.Reason == "saved" && (in.Method == "" || strings.EqualFold(in.Method, "card"))) {
		// A saved card is charged through the saved-card top-up; the Payment Element
		// is for a buyer paying anew or choosing a non-card method (ACH, crypto).
		return nil, fault(422, "local payment methods are not offered for this checkout", nil)
	}
	if cerr := atCap(in.Subject); cerr != nil {
		return nil, fault(429, cerr.Error(), cerr)
	}

	reg := processorsForOrg(org)
	quote, qf := quoteFor(ctx, reg, pol, in.AmountCents, d.Country, in.Currency, in.Method, true)
	if qf != nil {
		return nil, qf
	}
	cur, paid, rate := quote.Currency, quote.Amount, quote.Rate

	sp, err := processorsForOrg(org).Existing(ctx, processor.Stripe)
	maker, ok := sp.(processor.LocalIntentMaker)
	if err != nil || !ok {
		return nil, fault(503, "Stripe is not available right now", err)
	}
	key := in.IdempotencyKey
	if key == "" {
		key = windowKey(fmt.Sprintf("local:%d:%s:%s:%s", in.AmountCents, cur, d.Country, strings.ToLower(in.Method)))
		// A derived key replays an intent that can still be paid, and only that. A
		// buyer who pressed twice gets the one unpaid intent back; one who comes back
		// after paying (or after it was cancelled or is processing) gets a new one, not
		// the finished payment their browser cannot confirm. The intents already opened
		// for this amount in the window number the key.
		prior, perr := pendingintent.Since(strings.ToLower(in.Subject), pendingintent.Topup, time.Now().Add(-idemWindowSec*time.Second))
		if perr != nil {
			return nil, fault(503, "billing is temporarily unavailable; please retry", perr)
		}
		paidCur := cur
		if cur == "usd" {
			paidCur = ""
		}
		n := 0
		for _, p := range prior {
			if p.Cents != in.AmountCents || p.PaidCurrency != paidCur || in.Method != "" {
				// (a checkout for one method is its own intent, never an all-methods one replayed)
				continue
			}
			n++
			if p.Done {
				continue
			}
			if p.LockExpiresAt > 0 && time.Now().Unix() >= p.LockExpiresAt-30 {
				// The rate lock has lapsed (or is about to): the old intent is not replayed at
				// a rate Stripe will refuse. A new one is opened below on a fresh quote, and the
				// buyer is shown its amount before they authorize.
				continue
			}
			tx, terr := sp.GetTransaction(ctx, p.Id())
			if terr != nil {
				continue
			}
			switch tx.Status {
			case "requires_payment_method", "requires_confirmation", "requires_action":
				secret, _ := tx.Metadata["client_secret"].(string)
				if secret == "" {
					continue
				}
				amount, dec, rate := p.Cents, int32(2), 1.0
				if p.PaidCurrency != "" {
					// The rate is per major unit: yen paid per dollar, not yen-minor per cent.
					dec = currency.Type(p.PaidCurrency).Decimals()
					amount, rate = p.PaidCents, (float64(p.PaidCents)/math.Pow10(int(dec)))/(float64(p.Cents)/100)
					if p.Rate > 0 {
						rate = p.Rate // the rate bound when the attempt was opened
					}
				}
				return &TopupIntentOut{
					IntentID: p.Id(), ClientSecret: secret, PublishableKey: payment.StripePublicConfig(org).PublishableKey,
					Currency: cur, Amount: amount, UsdCents: in.AmountCents, Rate: rate,
					Quote: routing.Quote{Amount: amount, Decimals: int(dec), Currency: cur, UsdCents: in.AmountCents, Rate: rate,
						Source: p.RateSource, FXFee: p.FXFee, ExchangeRate: p.ExchangeRate, LockExpiresAt: p.LockExpiresAt, QuoteID: p.QuoteID, BaseRate: p.BaseRate, FeeRate: p.FeeRate, Premium: p.Premium, Disclosure: routing.Disclosure},
				}, nil
			}
		}
		if n > 0 {
			key = fmt.Sprintf("%s:%d", key, n)
		}
	}
	// The payer's one customer at Stripe, named and addressed, so the issuer and Radar
	// see a person and not an anonymous buyer — and required when vaulting a bank debit
	// for recurring plan renewals.
	payer := payerFor(org, in.Subject, in.Payer)
	customer := ""
	if cv, ok := sp.(cardVault); ok && (payer.Known() || planSlug != "") {
		if id, cerr := vaultCustomer(ctx, datastore.New(org.Namespaced(ctx)), cv, in.Subject, payer); cerr != nil {
			log.Warn("topup intent: no customer for %s: %v", in.Subject, cerr)
		} else {
			customer = id
		}
	}
	// Apple Pay and Google Pay cannot be excluded by type on an intent; the switch is
	// enforced through a commerce-owned configuration that has them off.
	walletConfig := ""
	if !pol.MethodOn("apple_pay") || !pol.MethodOn("google_pay") {
		if wc, ok := sp.(processor.WalletConfigurer); ok {
			id, werr := wc.WalletConfiguration(ctx, pol.MethodOn("apple_pay"), pol.MethodOn("google_pay"))
			if werr != nil {
				log.Error("topup intent: no wallet configuration: %v", werr)
				return nil, fault(503, "local payment methods are unavailable right now; please retry", werr)
			}
			walletConfig = id
		}
	}
	excluded, ach := bankExclusions(ctx, sp, pol, cur, in.AmountCents)
	excluded = withoutCrypto(ctx, reg, excluded)
	verification := ""
	if ach {
		verification = "automatic"
	}
	var only []string
	if strings.EqualFold(in.Method, "us_bank_account") {
		// A checkout for the bank debit alone: refused where the debit is not on offer.
		if !ach {
			return nil, fault(422, "bank transfer (ACH) is not available for this payment", nil)
		}
		only = []string{"us_bank_account"}
	}
	desc := fmt.Sprintf("Top-up $%.2f USD for %s", float64(in.AmountCents)/100, in.Subject)
	if planSlug != "" {
		desc = fmt.Sprintf("Subscription %s — first period for %s", planSlug, in.Subject)
	}
	out, err := maker.CreateLocalIntent(ctx, processor.LocalIntent{
		Only:              only,
		Customer:          customer,
		ReceiptEmail:      payer.Email,
		Verification:      verification,
		SaveForOffSession: planSlug != "" && customer != "",
		Amount:            paid,
		Currency:          cur,
		Excluded:          excluded,
		Configuration:     walletConfig,
		QuoteID:           quote.QuoteID,
		Key:               gatewayKey("topup-local", in.Subject, fmt.Sprintf("%s:%d:%s", key, paid, planSlug)),
		Desc:              desc,
		Metadata:          map[string]interface{}{"hanzo_subject": in.Subject, "hanzo_usd_cents": in.AmountCents, "hanzo_org": org.Name, "org": org.Name, "user": strings.ToLower(in.Subject)},
	})
	if err != nil {
		return nil, fault(502, "the payment could not be opened", err)
	}
	paidCents, paidCur := paid, cur
	if cur == "usd" {
		paidCents, paidCur = 0, ""
	}
	if planSlug != "" {
		sess := new(checkoutsession.Session)
		sess.SetId(newSessionID())
		sess.Org, sess.Subject, sess.Test, sess.Status = org.Name, strings.ToLower(in.Subject), org.TestMode(), checkoutsession.Pending
		sess.Line, sess.Plan, sess.Level, sess.Quantity, sess.Interval, sess.Coupon, sess.TotalCents, sess.Intent =
			checkoutsession.Plan, planSlug, in.Level, in.Quantity, string(planInterval), strings.TrimSpace(in.Coupon), in.AmountCents, out.ID
		if serr := checkoutsession.Put(sess); serr != nil {
			if c, ok := sp.(interface{ CancelIntent(context.Context, string) error }); ok {
				_ = c.CancelIntent(ctx, out.ID)
			}
			return nil, fault(503, "billing is temporarily unavailable; please retry", serr)
		}
		if perr := pendingintent.PutRef(out.ID, org.Name, strings.ToLower(in.Subject), pendingintent.Plan, sess.Id(), in.AmountCents, "usd", org.TestMode()); perr != nil {
			if c, ok := sp.(interface{ CancelIntent(context.Context, string) error }); ok {
				_ = c.CancelIntent(ctx, out.ID)
			}
			return nil, fault(503, "billing is temporarily unavailable; please retry", perr)
		}
		_ = pendingintent.Bind(out.ID, pendingintent.Terms{Delayed: true})
	} else if perr := pendingintent.PutQuoted(out.ID, org.Name, strings.ToLower(in.Subject), pendingintent.Topup, in.AmountCents, "usd", paidCents, paidCur, polVersion, org.TestMode()); perr != nil {
		// Without the record the payment could not be attributed. Cancel it rather than
		// hand the buyer a secret whose money nobody could place.
		if c, ok := sp.(interface {
			CancelIntent(context.Context, string) error
		}); ok {
			_ = c.CancelIntent(ctx, out.ID)
		}
		log.Error("RECONCILE: intent %s could not be recorded: %v", out.ID, perr)
		return nil, fault(503, "billing is temporarily unavailable; please retry", perr)
	}
	if cur == "usd" && ach {
		// A bank debit settles days after the buyer left: the attempt waits for it.
		if berr := pendingintent.Bind(out.ID, pendingintent.Terms{Delayed: true}); berr != nil {
			log.Warn("topup intent: the terms of %s were not bound: %v", out.ID, berr)
		}
	}
	if cur != "usd" {
		// The terms the quote was made on are bound to this attempt, once: the rate
		// disclosed is the rate charged, and a refund or a record reads it back.
		if berr := pendingintent.Bind(out.ID, pendingintent.Terms{Rate: quote.Rate, RateSource: quote.Source, QuoteID: quote.QuoteID, BaseRate: quote.BaseRate, FeeRate: quote.FeeRate, Premium: quote.Premium,
			ExchangeRate: quote.ExchangeRate, ReferenceRate: quote.ReferenceRate, ReferenceProvider: quote.ReferenceProvider, FXFee: quote.FXFee, LockExpiresAt: quote.LockExpiresAt, Delayed: lockFor(in.Method) == fx.LockDay || ach}); berr != nil {
			log.Warn("topup intent: the terms of %s were not bound: %v", out.ID, berr)
		}
	}
	if _, eerr := engine.EmitBillingEvent(webhookGuardDB(ctx), "topup.intent.opened", "payment_intent", out.ID, in.Subject,
		map[string]interface{}{"policyVersion": polVersion, "currency": cur, "amount": paid, "usdCents": in.AmountCents, "source": quote.Source, "fxFee": quote.FXFee, "rate": rate, "fxQuote": quote.QuoteID, "org": org.Name}, nil); eerr != nil {
		log.Warn("topup intent: the opening was not recorded: %v", eerr)
	}
	return &TopupIntentOut{
		IntentID: out.ID, ClientSecret: out.ClientSecret,
		PublishableKey: payment.StripePublicConfig(org).PublishableKey,
		Currency:       cur, Amount: paid, UsdCents: in.AmountCents, Rate: rate, Quote: *quote, PolicyVersion: polVersion,
	}, nil
}

// TopupIntentState is where a payment is in its life, for the page that waits on it.
type TopupIntentState struct {
	// State is created, pending, succeeded or canceled. An attempt the bank or the
	// method refused leaves the payment created, for the buyer to try again.
	State string `json:"state"`
	// Credited says the wallet holds the money. A page shows "paid" only on this.
	Credited bool `json:"credited"`
	// PaymentID is the receipt of the credit, set once Credited: the transaction the
	// wallet's credit was written as.
	PaymentID string `json:"paymentId,omitempty"`
	// SubscriptionID is set when a plan-bound bank transfer intent settles and opens a plan.
	SubscriptionID string `json:"subscriptionId,omitempty"`
}

// ReadTopupIntent answers where one of the caller's own payments is. A payment that
// is not theirs is not found.
func ReadTopupIntent(ctx context.Context, org *organization.Organization, subject, id string) (*TopupIntentState, *PaymentFault) {
	p, found, err := pendingintent.Get(id)
	if err != nil {
		return nil, fault(503, "billing is temporarily unavailable; please retry", err)
	}
	if !found || !strings.EqualFold(p.Subject, subject) || !strings.EqualFold(p.Org, org.Name) {
		return nil, fault(404, "payment not found", nil)
	}
	sp, err := processorsForOrg(org).Existing(ctx, processor.Stripe)
	if err != nil {
		return nil, fault(503, "Stripe is not available right now", err)
	}
	tx, err := sp.GetTransaction(ctx, id)
	if err != nil {
		return nil, fault(502, "the payment could not be read", err)
	}
	out := &TopupIntentState{State: processor.IntentState(tx.Status)}
	if p.Purpose == pendingintent.Plan {
		if s, ok, _ := checkoutsession.Get(p.Ref); ok && s.SubscriptionID != "" {
			out.Credited = true
			out.SubscriptionID = s.SubscriptionID
			out.PaymentID = s.PaymentID
			return out, nil
		}
	}
	if tx.ProcessorRef != "" {
		_, out.Credited, _ = paymentorg.Get(tx.ProcessorRef)
		if out.Credited {
			db := datastore.New(org.Namespaced(ctx))
			stamped := transaction.New(db)
			if found, _ := stamped.Query().Filter("SourceKind=", chargeSourceKind(processor.Stripe)).Filter("SourceId=", tx.ProcessorRef).Get(); found {
				out.PaymentID = stamped.Id()
			}
		}
	}
	return out, nil
}

type topupIntentRequest struct {
	AmountCents int64          `json:"amountCents"`
	Country     string         `json:"country"`
	Currency    string         `json:"currency"`
	Method      string         `json:"method"`
	PlanID      string         `json:"planId,omitempty"`
	Quantity    int            `json:"quantity,omitempty"`
	Level       int            `json:"level,omitempty"`
	Interval    types.Interval `json:"interval,omitempty"`
	Coupon      string         `json:"coupon,omitempty"`
}

// CreateTopupIntent opens a Stripe checkout for a top-up.
//
//	POST /v1/billing/topup/intent   { amountCents, country? }
func CreateTopupIntent(c *zip.Ctx) error {
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return jsonhttp.Fail(c, 401, "sign in to add funds", nil)
	}
	var req topupIntentRequest
	if err := c.Bind(&req); err != nil {
		return jsonhttp.Fail(c, 400, "invalid request body", err)
	}
	country := strings.TrimSpace(req.Country)
	if country == "" {
		country = requestCountry(c)
	}
	out, f := OpenTopupIntent(c.Context(), org, TopupIntentIn{
		Subject: userBillingKey(c), Country: country, AmountCents: req.AmountCents,
		Currency: strings.TrimSpace(req.Currency), Method: req.Method,
		IdempotencyKey: strings.TrimSpace(c.Header("X-Idempotency-Key")),
		Payer:          PayerIn{Email: iamEmail(c)},
		PlanID:         req.PlanID, Quantity: req.Quantity, Level: req.Level, Interval: req.Interval, Coupon: req.Coupon,
		Promo: promo.Active(c),
	})
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, out)
}

// GetTopupIntent says where one of the caller's payments is.
//
//	GET /v1/billing/topup/intent/:id
func GetTopupIntent(c *zip.Ctx) error {
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return jsonhttp.Fail(c, 401, "sign in to add funds", nil)
	}
	out, f := ReadTopupIntent(c.Context(), org, userBillingKey(c), c.Param("id"))
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, out)
}

// iamEmail is the verified identity's email, "" when the request carries none.
func iamEmail(c *zip.Ctx) string {
	e, _ := c.Locals("iam_email").(string)
	return strings.TrimSpace(e)
}

// bankExclusions are the payment method types the Payment Element must leave out for a
// payment of usdCents in cur: the policy's switched-off methods; every method whose
// account capability is not active (a method is shown only where Stripe can take it); and
// ACH unless the payment is in dollars and within the per-attempt limit. ach says the
// bank debit stays on offer.
func bankExclusions(ctx context.Context, sp processor.PaymentProcessor, pol routing.Policy, cur string, usdCents int64) (excluded []string, ach bool) {
	excluded = pol.Off()
	skip := map[string]bool{}
	for _, m := range excluded {
		skip[m] = true
	}
	drop := func(m string) {
		if !skip[m] {
			skip[m] = true
			excluded = append(excluded, m)
		}
	}
	if c, ok := sp.(processor.Capable); ok {
		have := c.Methods(ctx)
		for _, m := range routing.BankMethodNames() {
			if !have[m] {
				drop(m)
			}
		}
	}
	if cur != "usd" || usdCents > routing.ACHMaxCents {
		drop("us_bank_account")
	}
	return excluded, !skip["us_bank_account"]
}

// policyMethod is the policy's name for a method a client asks for: "ach" is Stripe's bank debit.
func policyMethod(m string) string {
	m = strings.ToLower(strings.TrimSpace(m))
	if m == "ach" {
		return "us_bank_account"
	}
	return m
}

// withoutCrypto adds crypto to the excluded types when Stripe cannot actually take
// stablecoins now: a crypto method is shown only when Stripe can take it.
func withoutCrypto(ctx context.Context, reg *processor.Registry, excluded []string) []string {
	for _, m := range excluded {
		if m == "crypto" {
			return excluded
		}
	}
	if !stripeStablecoins(ctx, reg).On {
		excluded = append(excluded, "crypto")
	}
	return excluded
}

// TopupIntentCancelled is the answer to cancelling a top-up.
type TopupIntentCancelled struct {
	IntentID string `json:"intentId"`
	// State is canceled, whether this call cancelled it or an earlier one did.
	State string `json:"state"`
}

// CancelTopupIntent cancels one of the caller's own top-up payments that has not been
// confirmed, at the processor, and releases its place under the cap of open payments. It is
// idempotent: a payment already cancelled answers cancelled again. A payment that has
// succeeded, is being processed by the bank, or has been credited is 409, and one that is
// not the caller's (or is not a top-up) is 404. Every cancellation is recorded as an event.
func CancelTopupIntent(ctx context.Context, org *organization.Organization, subject, id string) (*TopupIntentCancelled, *PaymentFault) {
	p, found, err := pendingintent.Get(id)
	if err != nil {
		return nil, fault(503, "billing is temporarily unavailable; please retry", err)
	}
	if !found || !strings.EqualFold(p.Subject, subject) || !strings.EqualFold(p.Org, org.Name) || p.Purpose != pendingintent.Topup {
		return nil, fault(404, "payment not found", nil)
	}
	sp, err := processorsForOrg(org).Existing(ctx, processor.Stripe)
	if err != nil {
		return nil, fault(503, "Stripe is not available right now", err)
	}
	read := func() (*processor.Transaction, *PaymentFault) {
		tx, terr := sp.GetTransaction(ctx, id)
		if terr != nil {
			return nil, fault(502, "the payment could not be read", terr)
		}
		return tx, nil
	}
	tx, f := read()
	if f != nil {
		return nil, f
	}
	refuse := func(tx *processor.Transaction) *PaymentFault {
		switch tx.Status {
		case "succeeded":
			return fault(409, "this payment has succeeded and cannot be cancelled", nil)
		case "processing":
			return fault(409, "this payment is being processed by the bank and cannot be cancelled", nil)
		}
		if tx.ProcessorRef != "" {
			if _, credited, _ := paymentorg.Get(tx.ProcessorRef); credited {
				return fault(409, "this payment has been credited and cannot be cancelled", nil)
			}
		}
		return nil
	}
	if f := refuse(tx); f != nil {
		return nil, f
	}
	if tx.Status != "canceled" {
		c, ok := sp.(interface {
			CancelIntent(context.Context, string) error
		})
		if !ok {
			return nil, fault(503, "this payment cannot be cancelled here", nil)
		}
		if cerr := c.CancelIntent(ctx, id); cerr != nil {
			// It may have succeeded between the read and the cancel.
			if again, rf := read(); rf == nil {
				if f := refuse(again); f != nil {
					return nil, f
				}
			}
			return nil, fault(502, "the payment could not be cancelled", cerr)
		}
	}
	first, serr := pendingintent.Settle(id)
	if serr != nil {
		return nil, fault(503, "billing is temporarily unavailable; please retry", serr)
	}
	if first {
		if _, eerr := engine.EmitBillingEvent(webhookGuardDB(ctx), "topup.intent.canceled", "payment_intent", id, subject,
			map[string]interface{}{"org": org.Name, "usdCents": p.Cents, "by": subject}, nil); eerr != nil {
			log.Warn("topup intent %s: the cancellation was not recorded: %v", id, eerr)
		}
	}
	return &TopupIntentCancelled{IntentID: id, State: "canceled"}, nil
}

// CancelTopupIntentHandler cancels one of the caller's own unconfirmed top-ups.
//
//	POST /v1/billing/topup/intent/:id/cancel
func CancelTopupIntentHandler(c *zip.Ctx) error {
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return jsonhttp.Fail(c, 401, "sign in to add funds", nil)
	}
	out, f := CancelTopupIntent(c.Context(), org, userBillingKey(c), c.Param("id"))
	if f != nil {
		return jsonhttp.Fail(c, f.Status, f.Message, f.Err)
	}
	return c.JSON(200, out)
}
