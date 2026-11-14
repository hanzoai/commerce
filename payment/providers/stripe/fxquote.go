package stripe

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hanzoai/commerce/payment/processor"
)

// fxVersion is the API version Stripe's FX Quotes API is in
// (https://docs.stripe.com/payments/currencies/localize-prices/fx-quotes-api).
const fxVersion = "2026-10-28.preview"

// postVersion is postKey at another API version.
func (p *Provider) postVersion(ctx context.Context, path string, params url.Values, key, version string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	if key = strings.TrimSpace(key); key != "" {
		req.Header.Set("Idempotency-Key", fitKey(key))
	}
	req.Header.Set("Stripe-Version", version)
	return p.doRequest(req, result)
}

// QuoteRate asks Stripe for its rate from cur to USD, locked for lock (none, hour, day).
// A quote made with lock none is Stripe's current rate and costs nothing; a locked one
// holds for the payment it is bound to and carries a premium. It answers an error when
// the account cannot use the FX Quotes API, and the caller falls back to a market rate.
func (p *Provider) QuoteRate(ctx context.Context, cur, lock string) (*processor.RateQuote, error) {
	cur = strings.ToLower(cur)
	form := url.Values{}
	form.Set("to_currency", "usd")
	form.Add("from_currencies[]", cur)
	form.Set("lock_duration", lock)
	form.Set("usage[type]", "payment")
	var q struct {
		ID            string `json:"id"`
		LockDuration  string `json:"lock_duration"`
		LockExpiresAt int64  `json:"lock_expires_at"`
		Rates         map[string]struct {
			ExchangeRate float64 `json:"exchange_rate"`
			Details      struct {
				BaseRate float64 `json:"base_rate"`
				Ref      float64 `json:"reference_rate"`
				RefBy    string  `json:"reference_rate_provider"`
				Premium  float64 `json:"duration_premium"`
				FeeRate  float64 `json:"fx_fee_rate"`
			} `json:"rate_details"`
		} `json:"rates"`
	}
	if err := p.postVersion(ctx, "/fx_quotes", form, "", fxVersion, &q); err != nil {
		return nil, err
	}
	r, ok := q.Rates[cur]
	if !ok || r.ExchangeRate <= 0 || r.Details.BaseRate <= 0 {
		return nil, fmt.Errorf("stripe: no FX quote for %s", cur)
	}
	// Stripe states USD per one unit of cur; the price of a dollar is its inverse.
	return &processor.RateQuote{ID: q.ID, Currency: cur, PerDollar: 1 / r.ExchangeRate, BasePerDollar: 1 / r.Details.BaseRate,
		FeeRate: r.Details.FeeRate, Premium: r.Details.Premium, ExchangeRate: r.ExchangeRate, ReferenceRate: r.Details.Ref, ReferenceProvider: r.Details.RefBy, Lock: q.LockDuration, ExpiresAt: q.LockExpiresAt}, nil
}

var _ processor.RateQuoter = (*Provider)(nil)

// Settlement reads the balance transaction of a settled PaymentIntent's charge.
func (p *Provider) Settlement(ctx context.Context, intentID string) (*processor.Settlement, error) {
	q := url.Values{}
	q.Add("expand[]", "latest_charge.balance_transaction")
	var pi struct {
		LatestCharge struct {
			BT struct {
				ID           string  `json:"id"`
				Currency     string  `json:"currency"`
				Amount       int64   `json:"amount"`
				Fee          int64   `json:"fee"`
				Net          int64   `json:"net"`
				ExchangeRate float64 `json:"exchange_rate"`
				FeeDetails   []struct {
					Type   string `json:"type"`
					Amount int64  `json:"amount"`
				} `json:"fee_details"`
			} `json:"balance_transaction"`
		} `json:"latest_charge"`
	}
	if err := p.get(ctx, "/payment_intents/"+intentID, q, &pi); err != nil {
		return nil, err
	}
	b := pi.LatestCharge.BT
	if b.ID == "" {
		return nil, fmt.Errorf("stripe: %s has no balance transaction yet", intentID)
	}
	s := &processor.Settlement{BalanceTransaction: b.ID, Currency: b.Currency, Gross: b.Amount, Fee: b.Fee, Net: b.Net, ExchangeRate: b.ExchangeRate}
	for _, d := range b.FeeDetails {
		if d.Type == "tax" {
			s.Tax += d.Amount
		}
	}
	return s, nil
}

var _ processor.SettlementReader = (*Provider)(nil)
