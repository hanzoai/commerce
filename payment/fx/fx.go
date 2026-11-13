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
)

// Spread is the margin over the mid-market rate, in basis points: what converting
// the local money back to dollars costs us, so a top-up of $5 is never worth less
// than $5 once it settles.
const Spread = 200

// Rate is the mid-market price of one USD in cur (a lower-case ISO 4217 code).
// It is a variable so the rate source is one place, and a test names its own.
var Rate = frankfurter

var (
	mu    sync.Mutex
	cache = map[string]quote{}
)

type quote struct {
	rate float64
	at   time.Time
}

const ttl = 15 * time.Minute

func frankfurter(ctx context.Context, cur string) (float64, error) {
	cur = strings.ToUpper(cur)
	mu.Lock()
	if q, ok := cache[cur]; ok && time.Since(q.at) < ttl {
		mu.Unlock()
		return q.rate, nil
	}
	mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.frankfurter.dev/v1/latest?base=USD&symbols="+cur, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fx: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
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
	mu.Lock()
	cache[cur] = quote{r, time.Now()}
	mu.Unlock()
	return r, nil
}

// Quote is what usdCents costs in cur, in cur's minor units, at the quoted rate
// plus Spread, rounded UP so the buyer's payment always covers the credit.
// cur must be a two-decimal currency.
func Quote(ctx context.Context, usdCents int64, cur string) (localCents int64, rate float64, err error) {
	if usdCents <= 0 {
		return 0, 0, errors.New("fx: nothing to quote")
	}
	rate, err = Rate(ctx, cur)
	if err != nil {
		return 0, 0, err
	}
	marked := rate * (1 + float64(Spread)/10000)
	return int64(math.Ceil(float64(usdCents) * marked)), marked, nil
}
