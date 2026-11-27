package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/hanzoai/commerce/payment/processor"
)

// CreateLocalIntent opens the intent. Nothing is credited by it: the money is
// credited only when Stripe reports the intent SUCCEEDED, which for a delayed
// method (Pix, boleto, OXXO, SEPA debit) is by webhook, minutes to days after the
// buyer left. A voucher that is never paid expires and credits nothing.
func (p *Provider) CreateLocalIntent(ctx context.Context, in processor.LocalIntent) (*processor.LocalIntentOut, error) {
	if in.Amount <= 0 || in.Currency == "" {
		return nil, fmt.Errorf("stripe local intent: amount and currency are required")
	}
	cur := strings.ToLower(in.Currency)
	params := url.Values{}
	params.Set("amount", strconv.FormatInt(in.Amount, 10))
	params.Set("currency", cur)
	if len(in.Only) > 0 {
		for i, m := range in.Only {
			params.Set(fmt.Sprintf("payment_method_types[%d]", i), m)
		}
	} else {
		params.Set("automatic_payment_methods[enabled]", "true")
	}
	// A voucher lives three days and a Pix code one, so an unpaid one ends by itself.
	// Only attach regional options to intents in the currencies that support them.
	if cur == "brl" {
		params.Set("payment_method_options[boleto][expires_after_days]", "3")
		params.Set("payment_method_options[pix][expires_after_seconds]", "86400")
	} else if cur == "mxn" {
		params.Set("payment_method_options[oxxo][expires_after_days]", "3")
	}
	if in.Verification != "" {
		// A bank debit is verified instantly through the bank's login (Financial
		// Connections) and by microdeposits when the bank is not covered.
		params.Set("payment_method_options[us_bank_account][verification_method]", in.Verification)
		params.Set("payment_method_options[us_bank_account][financial_connections][permissions][0]", "payment_method")
	}
	if len(in.Only) == 0 {
		if in.Configuration != "" {
			params.Set("payment_method_configuration", in.Configuration)
		}
		for i, m := range in.Excluded {
			params.Set(fmt.Sprintf("excluded_payment_method_types[%d]", i), m)
		}
	}
	if in.Desc != "" {
		params.Set("description", in.Desc)
	}
	if in.Customer != "" {
		params.Set("customer", in.Customer)
	}
	if in.SaveForOffSession {
		params.Set("setup_future_usage", "off_session")
	}
	if e := strings.TrimSpace(in.ReceiptEmail); e != "" {
		params.Set("receipt_email", e)
	}
	params.Set("statement_descriptor_suffix", processor.StatementSuffix)
	for k, v := range in.Metadata {
		params.Set("metadata["+k+"]", fmt.Sprintf("%v", v))
	}
	var pi paymentIntent
	var err error
	if in.QuoteID != "" {
		// A payment bound to a locked FX quote is converted at the quoted rate; Stripe
		// takes the quote on the PaymentIntent in its preview API version.
		params.Set("fx_quote", in.QuoteID)
		err = p.postVersion(ctx, "/payment_intents", params, in.Key, fxVersion, &pi)
	} else {
		err = p.postKey(ctx, "/payment_intents", params, in.Key, &pi)
	}
	if err != nil {
		return nil, err
	}
	// If Stripe returns an explicit empty payment_method_types list for this currency/config,
	// cancel the unusable intent immediately so the buyer never lands on a broken checkout.
	if len(in.Only) == 0 && pi.PaymentMethodTypes != nil && len(pi.PaymentMethodTypes) == 0 {
		if pi.ID != "" {
			_ = p.CancelIntent(ctx, pi.ID)
		}
		return nil, processor.ErrNoPaymentMethods
	}
	return &processor.LocalIntentOut{ID: pi.ID, ClientSecret: pi.ClientSecret}, nil
}

// WalletConfiguration answers the id of the commerce-owned Payment Method Configuration
// that offers what the account's default configuration offers, with Apple Pay and Google
// Pay set as asked. Stripe cannot exclude a wallet by method type on an intent, but it
// takes a configuration per intent (payment_method_configuration), and a configuration
// can turn a wallet off. It is found by name or made once, copying the default's
// preferences so nothing else changes.
func (p *Provider) WalletConfiguration(ctx context.Context, applePay, googlePay bool) (string, error) {
	pref := func(on bool) string {
		if on {
			return "on"
		}
		return "off"
	}
	name := "hanzo-wallets-apple-" + pref(applePay) + "-google-" + pref(googlePay)
	var list struct {
		Data []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Active    bool   `json:"active"`
			IsDefault bool   `json:"is_default"`
		} `json:"data"`
	}
	q := url.Values{}
	q.Set("limit", "100")
	if err := p.get(ctx, "/payment_method_configurations", q, &list); err != nil {
		return "", err
	}
	var def string
	for _, c := range list.Data {
		if c.Name == name && c.Active {
			return c.ID, nil
		}
		if c.IsDefault {
			def = c.ID
		}
	}
	// Copy the default's preference for every method, then set the wallets.
	var base map[string]json.RawMessage
	if def == "" {
		return "", fmt.Errorf("stripe: the account has no default payment method configuration")
	}
	if err := p.get(ctx, "/payment_method_configurations/"+def, nil, &base); err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("name", name)
	for k, raw := range base {
		var m struct {
			DisplayPreference struct {
				Preference string `json:"preference"`
			} `json:"display_preference"`
		}
		if json.Unmarshal(raw, &m) != nil || m.DisplayPreference.Preference == "" || m.DisplayPreference.Preference == "none" {
			continue
		}
		form.Set(k+"[display_preference][preference]", m.DisplayPreference.Preference)
	}
	form.Set("apple_pay[display_preference][preference]", pref(applePay))
	form.Set("google_pay[display_preference][preference]", pref(googlePay))
	var made struct {
		ID string `json:"id"`
	}
	if err := p.post(ctx, "/payment_method_configurations", form, &made); err != nil {
		return "", err
	}
	return made.ID, nil
}

var _ processor.WalletConfigurer = (*Provider)(nil)
