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
	params := url.Values{}
	params.Set("amount", strconv.FormatInt(in.Amount, 10))
	params.Set("currency", strings.ToLower(in.Currency))
	params.Set("automatic_payment_methods[enabled]", "true")
	// A voucher lives three days and a Pix code one, so an unpaid one ends by itself.
	params.Set("payment_method_options[boleto][expires_after_days]", "3")
	params.Set("payment_method_options[oxxo][expires_after_days]", "3")
	params.Set("payment_method_options[pix][expires_after_seconds]", "86400")
	if in.Configuration != "" {
		params.Set("payment_method_configuration", in.Configuration)
	}
	for i, m := range in.Excluded {
		params.Set(fmt.Sprintf("excluded_payment_method_types[%d]", i), m)
	}
	if in.Desc != "" {
		params.Set("description", in.Desc)
	}
	if in.Customer != "" {
		params.Set("customer", in.Customer)
	}
	if e := strings.TrimSpace(in.ReceiptEmail); e != "" {
		params.Set("receipt_email", e)
	}
	params.Set("statement_descriptor_suffix", processor.StatementSuffix)
	for k, v := range in.Metadata {
		params.Set("metadata["+k+"]", fmt.Sprintf("%v", v))
	}
	var pi paymentIntent
	if err := p.postKey(ctx, "/payment_intents", params, in.Key, &pi); err != nil {
		return nil, err
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
