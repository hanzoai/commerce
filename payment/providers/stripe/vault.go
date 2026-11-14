package stripe

import (
	"context"
	"net/url"
	"strings"

	"github.com/hanzoai/commerce/payment/processor"
)

// Vault saves a card the browser's Stripe.js minted (a PaymentMethod, "pm_…") to
// a customer for later off-session charges, and reports the card.
//
// It is a SetupIntent, not a bare attach: Stripe verifies the card with its bank
// and records the buyer's consent to be charged without being present, which is
// what a renewal or an auto-reload relies on. A card whose bank wants the buyer's
// authentication is refused with *processor.ActionRequired, nothing saved.
func (p *Provider) Vault(ctx context.Context, customerID, paymentMethodID string) (processor.Card, error) {
	// A SetupIntent the buyer authenticated in the browser ("seti_…") is finished,
	// not made again: it must have succeeded FOR THIS CUSTOMER.
	if strings.HasPrefix(paymentMethodID, "seti_") {
		return p.finishSetup(ctx, customerID, paymentMethodID)
	}
	params := url.Values{}
	params.Set("customer", customerID)
	params.Set("payment_method", paymentMethodID)
	params.Set("confirm", "true")
	params.Set("usage", "off_session")
	params.Set("automatic_payment_methods[enabled]", "true")
	params.Set("automatic_payment_methods[allow_redirects]", "never")

	var si struct {
		ID            string `json:"id"`
		Status        string `json:"status"`
		ClientSecret  string `json:"client_secret"`
		PaymentMethod string `json:"payment_method"`
	}
	if err := p.postKey(ctx, "/setup_intents", params, "", &si); err != nil {
		return processor.Card{}, err
	}
	if si.Status == "requires_action" {
		return processor.Card{}, &processor.ActionRequired{ID: si.ID, ClientSecret: si.ClientSecret}
	}
	if si.Status != "succeeded" {
		return processor.Card{}, &processor.Answer{Err: processor.NewPaymentError(processor.Stripe, "setup_not_completed",
			"the card could not be saved: "+si.Status, nil)}
	}
	// The card on file is the PaymentMethod the SetupIntent saved, by the id it
	// answers with: the one attached to the customer, which is what a later charge
	// must name.
	card, err := p.saved(ctx, customerID, si.PaymentMethod)
	if err == nil {
		p.completeBilling(ctx, customerID, si.PaymentMethod)
	}
	return card, err
}

// finishSetup is the card a SetupIntent the buyer authenticated saved, for
// customerID only.
func (p *Provider) finishSetup(ctx context.Context, customerID, id string) (processor.Card, error) {
	var si struct {
		Status        string `json:"status"`
		Customer      string `json:"customer"`
		PaymentMethod string `json:"payment_method"`
	}
	if err := p.get(ctx, "/setup_intents/"+url.PathEscape(id), nil, &si); err != nil {
		return processor.Card{}, err
	}
	if si.Status != "succeeded" || si.Customer != customerID || si.PaymentMethod == "" {
		return processor.Card{}, &processor.Answer{Err: processor.NewPaymentError(processor.Stripe, "setup_not_completed",
			"the card was not confirmed for this account", nil)}
	}
	card, err := p.saved(ctx, customerID, si.PaymentMethod)
	if err == nil {
		p.completeBilling(ctx, customerID, si.PaymentMethod)
	}
	return card, err
}

// saved reads the card a PaymentMethod is and refuses one issued in a sanctioned
// country, detaching it again.
func (p *Provider) saved(ctx context.Context, customerID, pm string) (processor.Card, error) {
	card, err := p.card(ctx, pm)
	if err != nil {
		return card, err
	}
	if processor.Sanctioned(card.Country) {
		_ = p.RemovePaymentMethod(ctx, customerID, pm)
		return processor.Card{}, errSanctioned
	}
	return card, nil
}

// errSanctioned is the refusal of a card issued where no payment can be taken.
var errSanctioned = &processor.Answer{Err: processor.NewPaymentError(processor.Stripe, "card_country_unsupported",
	"cards issued in this country cannot be used here", nil)}

// card reads a PaymentMethod's card facts.
func (p *Provider) card(ctx context.Context, id string) (processor.Card, error) {
	var pm stripePaymentMethod
	if err := p.get(ctx, "/payment_methods/"+url.PathEscape(id), nil, &pm); err != nil {
		return processor.Card{}, err
	}
	return pm.card(), nil
}

// Cards lists the cards saved to a customer.
func (p *Provider) Cards(ctx context.Context, customerID string) ([]processor.Card, error) {
	var list struct {
		Data []stripePaymentMethod `json:"data"`
	}
	if err := p.get(ctx, "/payment_methods", url.Values{"customer": {customerID}, "type": {"card"}, "limit": {"100"}}, &list); err != nil {
		return nil, err
	}
	out := make([]processor.Card, 0, len(list.Data))
	for _, pm := range list.Data {
		out = append(out, pm.card())
	}
	return out, nil
}

// FindCustomer finds the customer made under reference, "" when there is none.
func (p *Provider) FindCustomer(ctx context.Context, reference string) (string, error) {
	q := "metadata['reference']:'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(reference) + "'"
	var list struct {
		Data []stripeCustomer `json:"data"`
	}
	if err := p.get(ctx, "/customers/search", url.Values{"query": {q}, "limit": {"1"}}, &list); err != nil {
		return "", err
	}
	if len(list.Data) == 0 {
		return "", nil
	}
	return list.Data[0].ID, nil
}

type stripePaymentMethod struct {
	ID   string `json:"id"`
	Card struct {
		Brand       string `json:"brand"`
		Last4       string `json:"last4"`
		ExpMonth    int    `json:"exp_month"`
		ExpYear     int    `json:"exp_year"`
		Fingerprint string `json:"fingerprint"`
		Country     string `json:"country"`
	} `json:"card"`
	Customer string `json:"customer"`
}

func (m stripePaymentMethod) card() processor.Card {
	return processor.Card{
		ID: m.ID, Brand: m.Card.Brand, Last4: m.Card.Last4,
		ExpMonth: m.Card.ExpMonth, ExpYear: m.Card.ExpYear,
		Fingerprint: m.Card.Fingerprint, CustomerID: m.Customer, Country: m.Card.Country,
	}
}
