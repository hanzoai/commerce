// Package fx quotes what a USD amount costs in a buyer's own currency, for the
// payment methods that only take local money (Pix in BRL, OXXO in MXN, ...).
//
// The quote is made once, when the checkout opens, and recorded with the payment:
// the buyer pays the quoted local amount and the wallet is credited the USD they
// agreed to buy, whatever the rate does afterwards. Commerce never credits an
// amount the buyer did not pay.
package fx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hanzoai/commerce/models/types/currency"
)

const (
	// MinChargeUSDCents is Stripe's minimum charge for an account that settles in USD
	// (0.50 USD, https://docs.stripe.com/currencies#minimum-and-maximum-charge-amounts).
	// A charge in another currency must meet the equivalent after conversion, and a
	// quote only rounds up, so a credit of at least this clears it in every currency.
	MinChargeUSDCents = 50
)

// Rate is the mid-market price of one USD in cur (a lower-case ISO 4217 code), in
// major units: 150 for JPY means one dollar is 150 yen. It is a variable so the rate
// source is one place, and a test names its own.
var Rate = sources

var (
	mu    sync.Mutex
	cache = map[string]quote{}
)

type quote struct {
	rate float64
	at   time.Time
}

const ttl = 15 * time.Minute

// sources asks the ECB reference rates (Frankfurter, about thirty currencies) first and
// the wider table (open.er-api.com, every currency Stripe presents) when the first has
// no rate for cur, so a buyer in the UAE or Vietnam is quoted and not turned away.
func sources(ctx context.Context, cur string) (float64, error) {
	cur = strings.ToUpper(cur)
	mu.Lock()
	if q, ok := cache[cur]; ok && time.Since(q.at) < ttl {
		mu.Unlock()
		return q.rate, nil
	}
	mu.Unlock()
	r, err := frankfurter(ctx, cur)
	if err != nil {
		var werr error
		if r, werr = wide(ctx, cur); werr != nil {
			return 0, fmt.Errorf("%w; %v", err, werr)
		}
	}
	mu.Lock()
	cache[cur] = quote{r, time.Now()}
	mu.Unlock()
	return r, nil
}

func frankfurter(ctx context.Context, cur string) (float64, error) {
	return fetch(ctx, "https://api.frankfurter.dev/v1/latest?base=USD&symbols="+cur, cur)
}

func wide(ctx context.Context, cur string) (float64, error) {
	return fetch(ctx, "https://open.er-api.com/v6/latest/USD", cur)
}

// fetch reads the rates object at url and answers cur's.
func fetch(ctx context.Context, url, cur string) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fx: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<18))
	if err != nil || resp.StatusCode != 200 {
		return 0, fmt.Errorf("fx: rate source answered %d", resp.StatusCode)
	}
	var out struct {
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, fmt.Errorf("fx: %w", err)
	}
	r := out.Rates[cur]
	if r <= 0 || math.IsNaN(r) || math.IsInf(r, 0) {
		return 0, fmt.Errorf("fx: no rate for %s", cur)
	}
	return r, nil
}

// Granule is the smallest amount Stripe takes in cur, in cur's minor units
// (https://docs.stripe.com/currencies#special-cases): 1 for a two-decimal and a
// zero-decimal currency, 10 for a three-decimal one (the amount ends in 0), and 100 for
// ISK and UGX, stated in hundredths with the fraction always 00.
func Granule(cur string) int64 {
	switch cur = strings.ToLower(cur); cur {
	case "isk", "ugx":
		return 100
	}
	if currency.Type(cur).Decimals() == 3 {
		return 10
	}
	return 1
}

// Minor is inc major units of cur in cur's minor units (0.01 KWD is 10 fils).
func Minor(inc float64, cur string) int64 {
	return int64(math.Round(inc * math.Pow10(int(currency.Type(strings.ToLower(cur)).Decimals()))))
}

// Convert is what usdCents costs in cur, in cur's minor units, at rate (cur per
// dollar, in major units), rounded UP to a multiple of step minor units, so the buyer's
// payment always covers the credit: P_local = ceilToIncrement(P_usd x rate). The wallet
// is credited usdCents exactly, whatever the rounding. step must be a multiple of
// Granule(cur).
func Convert(usdCents int64, cur string, rate float64, step int64) (int64, error) {
	if usdCents <= 0 || rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return 0, errors.New("fx: nothing to quote")
	}
	if step <= 0 || step%Granule(cur) != 0 {
		return 0, fmt.Errorf("fx: a step of %d is not an amount Stripe takes in %s", step, cur)
	}
	// usdCents is hundredths of a dollar; the answer is in cur's minor unit, which is
	// 10^decimals of cur's major unit, so the scale is 10^(decimals-2) and not always 1.
	scale := math.Pow10(int(currency.Type(strings.ToLower(cur)).Decimals()) - 2)
	// Float error must not cost the buyer a unit: 945.0000000000001 is 945.
	v := math.Round(float64(usdCents)*rate*scale*1e6) / 1e6
	n := int64(math.Ceil(v/float64(step))) * step
	if n <= 0 || n > 999999999999 {
		return 0, fmt.Errorf("fx: %d usd cents is %d %s, outside what Stripe takes", usdCents, n, strings.ToLower(cur))
	}
	return n, nil
}

// Quote is Convert at the market rate (Frankfurter's, the wider table when it has no
// rate): what usdCents costs in cur, rounded up to step, and the rate applied. No margin
// is added; the cost of converting the money back is Hanzo's.
func Quote(ctx context.Context, usdCents int64, cur string, step int64) (minor int64, rate float64, err error) {
	rate, err = Rate(ctx, cur)
	if err != nil {
		return 0, 0, err
	}
	if minor, err = Convert(usdCents, cur, rate, step); err != nil {
		return 0, 0, err
	}
	return minor, rate, nil
}
