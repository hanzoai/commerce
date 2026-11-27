package stripe

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hanzoai/commerce/payment/processor"
)

// agentVersion is the API version Shared Payment Tokens and stablecoin deposit mode are in
// (https://docs.stripe.com/agentic-commerce/concepts/shared-payment-tokens,
// https://docs.stripe.com/payments/accept-stablecoin-payments).
const agentVersion = "2026-10-28.preview"

// getVersion is get at another API version.
func (p *Provider) getVersion(ctx context.Context, path string, params url.Values, version string, result interface{}) error {
	u := baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Stripe-Version", version)
	return p.doRequest(req, result)
}

// isToken says a charge token is a shared payment token.
func isToken(t string) bool { return strings.HasPrefix(t, "spt_") }

// Grant reads a shared payment token granted to this account: its limits and whether it can
// still be used, with only what is safe to say of the card behind it.
func (p *Provider) Grant(ctx context.Context, id string) (*processor.Grant, error) {
	var t struct {
		ID                string `json:"id"`
		DeactivatedAt     int64  `json:"deactivated_at"`
		DeactivatedReason string `json:"deactivated_reason"`
		Livemode          bool   `json:"livemode"`
		UsageLimits       struct {
			Currency          string `json:"currency"`
			ExpiresAt         int64  `json:"expires_at"`
			MaxAmount         int64  `json:"max_amount"`
			RecurringInterval string `json:"recurring_interval"`
		} `json:"usage_limits"`
		UsageDetails struct {
			AmountCaptured struct {
				Value int64 `json:"value"`
			} `json:"amount_captured"`
		} `json:"usage_details"`
		PaymentMethodDetails struct {
			Card struct {
				Brand   string `json:"brand"`
				Country string `json:"country"`
				Last4   string `json:"last4"`
			} `json:"card"`
		} `json:"payment_method_details"`
	}
	if err := p.getVersion(ctx, "/shared_payment/granted_tokens/"+url.PathEscape(id), nil, agentVersion, &t); err != nil {
		return nil, err
	}
	g := &processor.Grant{
		ID: t.ID, Active: t.DeactivatedAt == 0, Why: t.DeactivatedReason,
		Currency: strings.ToLower(t.UsageLimits.Currency), MaxAmount: t.UsageLimits.MaxAmount, Captured: t.UsageDetails.AmountCaptured.Value, ExpiresAt: t.UsageLimits.ExpiresAt,
		RecurringInterval: t.UsageLimits.RecurringInterval,
		Country:           strings.ToUpper(t.PaymentMethodDetails.Card.Country), Brand: t.PaymentMethodDetails.Card.Brand, Last4: t.PaymentMethodDetails.Card.Last4,
		Test: !t.Livemode,
	}
	if !g.Active && g.Why == "" {
		g.Why = "deactivated"
	}
	return g, nil
}

var _ processor.GrantReader = (*Provider)(nil)

// chargeToken is Charge for a shared payment token: the grant is read first (a revoked,
// expired, over-limit, wrong-currency or sanctioned-country grant is refused here, before
// any intent exists), then a PaymentIntent is created on it and confirmed. The grant's own
// holder, this account, is the only one Stripe lets use it; Radar, refunds and disputes
// apply to the intent as to any card payment.
//
// A charge without the buyer present (a renewal) is allowed only on a token that carries a
// recurring interval: a one-time token pays once and is never charged again.
func (p *Provider) chargeToken(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	g, err := p.Grant(ctx, req.Token)
	if err != nil {
		return refusedGrant("token_unreadable", "the token could not be read: "+err.Error())
	}
	off, _ := req.Options["offSession"].(bool)
	if code, why := processor.GrantRefusal(g, int64(req.Amount), strings.ToLower(string(req.Currency)), off, time.Now()); why != "" {
		return refusedGrant(code, why)
	}
	if processor.Sanctioned(g.Country) {
		return &processor.PaymentResult{Success: false, ErrorMessage: errSanctioned.Error(), Error: errSanctioned}, errSanctioned
	}
	params := url.Values{}
	params.Set("amount", strconv.FormatInt(int64(req.Amount), 10))
	params.Set("currency", strings.ToLower(string(req.Currency)))
	params.Set("confirm", "true")
	params.Set("payment_method_data[shared_payment_granted_token]", req.Token)
	params.Set("automatic_payment_methods[enabled]", "true")
	params.Set("automatic_payment_methods[allow_redirects]", "never")
	if off {
		params.Set("off_session", "true")
	}
	if req.Description != "" {
		params.Set("description", req.Description)
	}
	params.Set("statement_descriptor_suffix", processor.StatementSuffix)
	for k, v := range req.Metadata {
		params.Set("metadata["+k+"]", fmt.Sprintf("%v", v))
	}
	var pi paymentIntent
	if err := p.postVersion(ctx, "/payment_intents", params, req.IdempotencyKey, agentVersion, &pi); err != nil {
		if pe := intentOf(err); pe != nil && pe.Status == "requires_action" {
			return actionRequired(pe)
		}
		return &processor.PaymentResult{Success: false, ErrorMessage: err.Error(), Error: err}, err
	}
	return resultOf(&pi)
}

// refusedGrant is a charge refused for the token's own reasons, with the code the caller
// reads: token_unreadable, token_revoked, token_expired, token_wrong_currency,
// token_over_limit or token_not_recurring.
func refusedGrant(code, why string) (*processor.PaymentResult, error) {
	e := &processor.Answer{Err: processor.NewPaymentError(processor.Stripe, code, why, nil)}
	return &processor.PaymentResult{Success: false, ErrorMessage: why, Error: e}, e
}

// stablecoin deposit mode.

// cryptoCache is what a provider last learned of its account's crypto configuration.
type cryptoCache struct {
	sync.Mutex
	at  time.Time
	id  string
	why string
}

// cryptoConfiguration is the payment method configuration that offers crypto: the default
// one when it does, else any active one that does. Cached for a minute.
func (p *Provider) cryptoConfiguration(ctx context.Context) (string, string) {
	p.crypto.Lock()
	defer p.crypto.Unlock()
	if time.Since(p.crypto.at) < time.Minute {
		return p.crypto.id, p.crypto.why
	}
	var list struct {
		Data []struct {
			ID        string `json:"id"`
			Active    bool   `json:"active"`
			IsDefault bool   `json:"is_default"`
			Crypto    struct {
				Available         bool `json:"available"`
				DisplayPreference struct {
					Value string `json:"value"`
				} `json:"display_preference"`
			} `json:"crypto"`
		} `json:"data"`
	}
	id, why := "", "stablecoin payments are not enabled on this Stripe account"
	q := url.Values{}
	q.Set("limit", "100")
	// Use agentVersion: Stripe added crypto to payment_method_configurations in Oct 2025
	// (after the base 2024-12-18.acacia version). Do not cache transient API errors.
	if err := p.getVersion(ctx, "/payment_method_configurations", q, agentVersion, &list); err != nil {
		return "", "the payment method configurations could not be read: " + err.Error()
	}
	for _, c := range list.Data {
		if !c.Active || !c.Crypto.Available || c.Crypto.DisplayPreference.Value == "off" {
			continue
		}
		if id == "" || c.IsDefault {
			id = c.ID
		}
	}
	if id != "" {
		why = ""
	}
	p.crypto.at, p.crypto.id, p.crypto.why = time.Now(), id, why
	return id, why
}

// StablecoinReady says whether this account can take stablecoin payments by deposit now:
// a payment method configuration offers crypto. The networks are the deposit mode's
// (base, solana, tempo), and stablecoin payments are presented in USD only.
func (p *Provider) StablecoinReady(ctx context.Context) processor.StablecoinReady {
	if p.key() == "" {
		return processor.StablecoinReady{Why: "Stripe is not configured"}
	}
	id, why := p.cryptoConfiguration(ctx)
	if id == "" {
		return processor.StablecoinReady{Why: why}
	}
	return processor.StablecoinReady{On: true, Networks: []string{"base", "solana", "tempo"}, Currency: "usd"}
}

// OpenStablecoinDeposit creates a PaymentIntent for a stablecoin payment in deposit mode and
// confirms it, so Stripe answers the address to send to per network and the token contract:
// no page and no wallet connection are involved. The payment settles when the deposit is
// confirmed on chain, which Stripe reports as payment_intent.succeeded.
func (p *Provider) OpenStablecoinDeposit(ctx context.Context, in processor.StablecoinDepositIn) (*processor.StablecoinDeposit, error) {
	if strings.ToLower(in.Currency) != "usd" {
		return nil, fmt.Errorf("stablecoin payments are presented in USD only, not %s", strings.ToUpper(in.Currency))
	}
	cfg, why := p.cryptoConfiguration(ctx)
	if cfg == "" {
		return nil, fmt.Errorf("%s", why)
	}
	params := url.Values{}
	params.Set("amount", strconv.FormatInt(in.Amount, 10))
	params.Set("currency", "usd")
	params.Set("payment_method_configuration", cfg)
	params.Set("confirm", "true")
	params.Set("return_url", "https://hanzo.ai/pay/return") // required by Stripe; no buyer is sent there
	params.Set("payment_method_data[type]", "crypto")
	params.Set("payment_method_options[crypto][mode]", "deposit")
	params.Set("payment_method_options[crypto][deposit_options][networks][]", in.Network)
	if in.Desc != "" {
		params.Set("description", in.Desc)
	}
	for k, v := range in.Metadata {
		params.Set("metadata["+k+"]", fmt.Sprintf("%v", v))
	}
	var pi struct {
		ID         string `json:"id"`
		Amount     int64  `json:"amount"`
		Currency   string `json:"currency"`
		NextAction struct {
			Type    string `json:"type"`
			Details struct {
				Addresses map[string]struct {
					Address string `json:"address"`
					Tokens  []struct {
						Contract string `json:"token_contract_address"`
						Currency string `json:"token_currency"`
					} `json:"supported_tokens"`
				} `json:"deposit_addresses"`
			} `json:"crypto_display_details"`
		} `json:"next_action"`
	}
	if err := p.postVersion(ctx, "/payment_intents", params, in.Key, agentVersion, &pi); err != nil {
		return nil, err
	}
	out := &processor.StablecoinDeposit{ID: pi.ID, Amount: pi.Amount, Currency: pi.Currency}
	for _, net := range []string{"base", "solana", "tempo"} {
		a, ok := pi.NextAction.Details.Addresses[net]
		if !ok {
			continue
		}
		d := processor.DepositAddress{Network: net, Address: a.Address}
		for _, t := range a.Tokens {
			d.Tokens = append(d.Tokens, processor.DepositToken{Currency: t.Currency, Contract: t.Contract})
		}
		out.Addresses = append(out.Addresses, d)
	}
	if len(out.Addresses) == 0 {
		return nil, fmt.Errorf("Stripe returned no deposit address for payment %s", pi.ID)
	}
	return out, nil
}

var _ processor.StablecoinDepositor = (*Provider)(nil)
