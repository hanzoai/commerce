package billing

import (
	"context"
	"fmt"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/payment"
	"github.com/hanzoai/commerce/payment/fx"
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
	d, f := Place(ctx, org, RouteIn{Subject: in.Subject, Country: in.Country})
	if f != nil {
		return nil, f
	}
	if d.Processor != processor.Stripe {
		return nil, fault(409, "this checkout is on "+string(d.Processor), nil)
	}
	pol, err := ReadRouting(ctx, org.Name)
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

	cur := routing.LocalCurrency(d.Country)
	paid, rate := in.AmountCents, 1.0
	if cur != "usd" {
		paid, rate, err = fx.Quote(ctx, in.AmountCents, cur)
		if err != nil {
			log.Error("topup intent: no quote for %s: %v", cur, err)
			return nil, fault(503, "local payment methods are unavailable right now; please retry", err)
		}
	}

	sp, err := processorsForOrg(org).Existing(ctx, processor.Stripe)
	maker, ok := sp.(processor.LocalIntentMaker)
	if err != nil || !ok {
		return nil, fault(503, "Stripe is not available right now", err)
	}
	key := in.IdempotencyKey
	if key == "" {
		key = windowKey(fmt.Sprintf("local:%d:%s:%s", in.AmountCents, cur, d.Country))
	}
	out, err := maker.CreateLocalIntent(ctx, processor.LocalIntent{
		Amount: paid, Currency: cur,
		Key:      gatewayKey("topup-local", in.Subject, fmt.Sprintf("%s:%d", key, paid)),
		Desc:     fmt.Sprintf("Top-up %d usd for %s", in.AmountCents, in.Subject),
		Metadata: map[string]interface{}{"hanzo_subject": in.Subject, "hanzo_usd_cents": in.AmountCents, "hanzo_org": org.Name},
	})
	if err != nil {
		return nil, fault(502, "the payment could not be opened", err)
	}
	paidCents, paidCur := paid, cur
	if cur == "usd" {
		paidCents, paidCur = 0, ""
	}
	if perr := pendingintent.PutQuoted(out.ID, org.Name, strings.ToLower(in.Subject), pendingintent.Topup, in.AmountCents, "usd", paidCents, paidCur, org.TestMode()); perr != nil {
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
	return &TopupIntentOut{
		IntentID: out.ID, ClientSecret: out.ClientSecret,
		PublishableKey: payment.StripePublicConfig(org).PublishableKey,
		Currency:       cur, Amount: paid, UsdCents: in.AmountCents, Rate: rate,
	}, nil
}

// TopupIntentState is where a payment is in its life, for the page that waits on it.
type TopupIntentState struct {
	// State is created, pending, succeeded or canceled. An attempt the bank or the
	// method refused leaves the payment created, for the buyer to try again.
	State string `json:"state"`
	// Credited says the wallet holds the money. A page shows "paid" only on this.
	Credited bool `json:"credited"`
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
	}
	return out, nil
}

type topupIntentRequest struct {
	AmountCents int64  `json:"amountCents"`
	Country     string `json:"country"`
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
