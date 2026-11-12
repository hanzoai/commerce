package stripe

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hanzoai/commerce/payment/processor"
)

// The card on file, Stripe's way: a PaymentMethod the browser made with
// Stripe.js (pm_…), attached to the buyer's Customer, and set up for charges
// made without the buyer (SetupIntent usage=off_session, or a first payment
// with setup_future_usage=off_session). The card number never reaches this
// process; the page holds it in Stripe's own iframe.

// Vault attaches the browser-made PaymentMethod pm to customerID and answers
// the card it is. A method already attached to customerID is answered as it
// is; one attached to another customer is refused, since a PaymentMethod
// belongs to one customer and charging it as somebody else's is a charge Stripe
// refuses anyway.
func (p *Provider) Vault(ctx context.Context, customerID, pm string) (processor.Card, error) {
	pm = strings.TrimSpace(pm)
	if !strings.HasPrefix(pm, "pm_") {
		return processor.Card{}, processor.NewPaymentError(processor.Stripe, "invalid_payment_method",
			"sourceId must be a Stripe PaymentMethod id (pm_…) made by Stripe.js", nil)
	}
	if customerID == "" {
		return processor.Card{}, fmt.Errorf("stripe: no customer to attach the card to")
	}
	var m paymentMethod
	if err := p.get(ctx, "/payment_methods/"+url.PathEscape(pm), nil, &m); err != nil {
		return processor.Card{}, err
	}
	switch {
	case m.Customer == customerID:
	case m.Customer != "":
		return processor.Card{}, processor.NewPaymentError(processor.Stripe, "payment_method_foreign",
			"this card is saved to another account; add it again", nil)
	default:
		params := url.Values{}
		params.Set("customer", customerID)
		if err := p.post(ctx, "/payment_methods/"+url.PathEscape(pm)+"/attach", params, &m); err != nil {
			return processor.Card{}, err
		}
	}
	if m.Type != "" && m.Type != "card" {
		return processor.Card{}, processor.NewPaymentError(processor.Stripe, "not_a_card",
			"only a card can be saved for plans and reloads", nil)
	}
	return processor.Card{
		ID:          m.ID,
		Brand:       m.Card.Brand,
		Last4:       m.Card.Last4,
		ExpMonth:    m.Card.ExpMonth,
		ExpYear:     m.Card.ExpYear,
		Fingerprint: m.Card.Fingerprint,
		CustomerID:  customerID,
	}, nil
}

// SetUp confirms that the attached card pm may be charged off session, through
// a SetupIntent with usage=off_session — the step Stripe's SCA rules require
// before a card saved without a payment is charged by a renewal. key is the
// request's idempotency key; resume names the SetupIntent a buyer has just
// authenticated (seti_…), which is read instead of making another.
//
// A bank that asks for authentication answers *processor.ActionRequired.
func (p *Provider) SetUp(ctx context.Context, customerID, pm, key, resume string) error {
	if resume = strings.TrimSpace(resume); resume != "" {
		if !strings.HasPrefix(resume, "seti_") {
			return processor.NewPaymentError(processor.Stripe, "invalid_intent", "setupIntent must be a Stripe SetupIntent id (seti_…)", nil)
		}
		var st setupIntent
		if err := p.get(ctx, "/setup_intents/"+url.PathEscape(resume), nil, &st); err != nil {
			return err
		}
		if st.Customer != customerID || st.PaymentMethod != pm || fmt.Sprint(st.Metadata[orderKey]) != fitKey(key) {
			return processor.NewPaymentError(processor.Stripe, "intent_mismatch", "the card setup named is not this card's", nil)
		}
		return setUpOutcome(&st)
	}
	params := url.Values{}
	params.Set("customer", customerID)
	params.Set("payment_method", pm)
	params.Set("confirm", "true")
	params.Set("usage", "off_session")
	params.Set("automatic_payment_methods[enabled]", "true")
	params.Set("automatic_payment_methods[allow_redirects]", "never")
	if key = strings.TrimSpace(key); key != "" {
		params.Set("metadata["+orderKey+"]", fitKey(key))
	}
	var st setupIntent
	if err := p.postKey(ctx, "/setup_intents", params, key, &st); err != nil {
		return err
	}
	return setUpOutcome(&st)
}

func setUpOutcome(st *setupIntent) error {
	switch st.Status {
	case "succeeded":
		return nil
	case "requires_action":
		return &processor.ActionRequired{Processor: processor.Stripe, Intent: st.ID, ClientSecret: st.ClientSecret}
	}
	return processor.NewPaymentError(processor.Stripe, "setup_incomplete",
		"the card could not be set up (status "+st.Status+"); try another card", nil)
}

type paymentMethod struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Customer string `json:"customer"`
	Card     struct {
		Brand       string `json:"brand"`
		Last4       string `json:"last4"`
		ExpMonth    int    `json:"exp_month"`
		ExpYear     int    `json:"exp_year"`
		Fingerprint string `json:"fingerprint"`
	} `json:"card"`
}

type setupIntent struct {
	ID            string                 `json:"id"`
	Status        string                 `json:"status"`
	Customer      string                 `json:"customer"`
	PaymentMethod string                 `json:"payment_method"`
	ClientSecret  string                 `json:"client_secret"`
	Metadata      map[string]interface{} `json:"metadata"`
}
