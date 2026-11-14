package billing

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/routing"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

// TopupIntentIn is a signed-in buyer opening a Stripe checkout for a top-up of
// AmountCents US dollars, paying with whatever their country offers.
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
	// SpreadBps is the margin over the mid-market rate the quote carries.
	SpreadBps int `json:"spreadBps"`
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
	minCents, maxCents := topupBounds()
	if in.AmountCents < minCents || in.AmountCents > maxCents {
		return nil, fault(400, fmt.Sprintf("amountCents must be between %d and %d", minCents, maxCents), nil)
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
	if !pol.LocalOn || d.Reason == "saved" {
		// A saved card is charged through the saved-card top-up; the Payment Element
		// is for a buyer paying anew.
		return nil, fault(422, "local payment methods are not offered for this checkout", nil)
	}
	if cerr := atCap(in.Subject); cerr != nil {
		return nil, fault(429, cerr.Error(), cerr)
	}

	reg := processorsForOrg(org)
	quote, qf := quoteFor(ctx, reg, pol, in.AmountCents, d.Country, in.Currency, in.Method)
	if qf != nil {
		return nil, qf
	}
	cur, paid, rate, spread := quote.Currency, quote.Amount, quote.Rate, quote.MarkupBps

	sp, err := processorsForOrg(org).Existing(ctx, processor.Stripe)
	maker, ok := sp.(processor.LocalIntentMaker)
	if err != nil || !ok {
		return nil, fault(503, "Stripe is not available right now", err)
	}
	key := in.IdempotencyKey
	if key == "" {
		key = windowKey(fmt.Sprintf("local:%d:%s:%s", in.AmountCents, cur, d.Country))
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
			if p.Cents != in.AmountCents || p.PaidCurrency != paidCur {
				continue
			}
			n++
			if p.Done {
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
				}
				return &TopupIntentOut{
					IntentID: p.Id(), ClientSecret: secret, PublishableKey: payment.StripePublicConfig(org).PublishableKey,
					Currency: cur, Amount: amount, UsdCents: in.AmountCents, Rate: rate,
					Quote: routing.Quote{Amount: amount, Decimals: int(dec), Currency: cur, UsdCents: in.AmountCents, Rate: rate},
				}, nil
			}
		}
		if n > 0 {
			key = fmt.Sprintf("%s:%d", key, n)
		}
	}
	// The payer's one customer at Stripe, named and addressed, so the issuer and Radar
	// see a person and not an anonymous buyer.
	payer := payerFor(org, in.Subject, in.Payer)
	customer := ""
	if cv, ok := sp.(cardVault); ok && payer.Known() {
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
	out, err := maker.CreateLocalIntent(ctx, processor.LocalIntent{
		Customer: customer, ReceiptEmail: payer.Email,
		Amount: paid, Currency: cur, Excluded: pol.Off(), Configuration: walletConfig,
		Key:      gatewayKey("topup-local", in.Subject, fmt.Sprintf("%s:%d", key, paid)),
		Desc:     fmt.Sprintf("Top-up %d usd for %s", in.AmountCents, in.Subject),
		Metadata: map[string]interface{}{"hanzo_subject": in.Subject, "hanzo_usd_cents": in.AmountCents, "hanzo_org": org.Name, "org": org.Name, "user": strings.ToLower(in.Subject)},
	})
	if err != nil {
		return nil, fault(502, "the payment could not be opened", err)
	}
	paidCents, paidCur := paid, cur
	if cur == "usd" {
		paidCents, paidCur = 0, ""
	}
	if perr := pendingintent.PutQuoted(out.ID, org.Name, strings.ToLower(in.Subject), pendingintent.Topup, in.AmountCents, "usd", paidCents, paidCur, polVersion, org.TestMode()); perr != nil {
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
	if _, eerr := engine.EmitBillingEvent(webhookGuardDB(ctx), "topup.intent.opened", "payment_intent", out.ID, in.Subject,
		map[string]interface{}{"policyVersion": polVersion, "currency": cur, "amount": paid, "usdCents": in.AmountCents, "markupBps": spread, "org": org.Name}, nil); eerr != nil {
		log.Warn("topup intent: the opening was not recorded: %v", eerr)
	}
	return &TopupIntentOut{
		IntentID: out.ID, ClientSecret: out.ClientSecret,
		PublishableKey: payment.StripePublicConfig(org).PublishableKey,
		Currency:       cur, Amount: paid, UsdCents: in.AmountCents, Rate: rate, SpreadBps: spread, Quote: *quote, PolicyVersion: polVersion,
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
	AmountCents int64  `json:"amountCents"`
	Country     string `json:"country"`
	Currency    string `json:"currency"`
	Method      string `json:"method"`
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
		IdempotencyKey: strings.TrimSpace(c.Header("X-Idempotency-Key")),
		Payer:          PayerIn{Email: iamEmail(c)},
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
