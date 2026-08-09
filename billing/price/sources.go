package price

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"
)

// The two venues this rail prices from. They are separate companies running
// separate books, which is the whole point of MinSources — an aggregator that
// reads both would look like a third opinion and be none.
//
// Both are public and unauthenticated. That is a real limit and it is stated
// here rather than discovered later: an unauthenticated endpoint can rate-limit
// or disappear without notice, and when it does the rail REFUSES to credit
// rather than guessing. Refusing is safe (the coin waits in custody) but it is
// not free — a customer's deposit sits uncredited — so a keyed feed is the
// upgrade path, not a rewrite.

// httpSource is the shape both venues share: one GET, one number out.
type httpSource struct {
	name   string
	url    func(symbol string) string
	parse  func([]byte) (string, error) // the decimal price as written by the venue
	client *http.Client
}

func (s httpSource) Name() string { return s.name }

func (s httpSource) MicroCents(ctx context.Context, symbol string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url(symbol), nil)
	if err != nil {
		return 0, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("price/%s: %w", s.name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("price/%s: HTTP %d", s.name, resp.StatusCode)
	}
	var body []byte
	if body, err = readAll(resp); err != nil {
		return 0, fmt.Errorf("price/%s: %w", s.name, err)
	}
	raw, err := s.parse(body)
	if err != nil {
		return 0, fmt.Errorf("price/%s: %w", s.name, err)
	}
	return decimalToMicroCents(raw)
}

// readAll bounds the read. A price is a few dozen bytes; anything else is not a
// price, and an unbounded read from a third party is a memory footgun.
func readAll(resp *http.Response) ([]byte, error) {
	const max = 1 << 16
	buf := make([]byte, max)
	n := 0
	for n < max {
		m, err := resp.Body.Read(buf[n:])
		n += m
		if err != nil {
			break
		}
	}
	return buf[:n], nil
}

// decimalToMicroCents converts a venue's decimal string to cents × Scale, EXACTLY.
//
// Parsing to float64 first is the bug this avoids: 65181.725 has no exact binary
// representation, and a rail that multiplies a customer's coin by a rounded float
// is off by an amount nobody can reproduce. The string is scaled AS TEXT and then
// read as an integer, so the only rounding is the deliberate one.
//
// The shift is 8 places — 2 to reach cents, 6 more for Scale — which keeps every
// digit any venue publishes. Truncating, always DOWN, the same direction as
// AmountCents and for the same reason: never credit value that was not sent.
func decimalToMicroCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty price")
	}
	neg := strings.HasPrefix(s, "-")
	if neg {
		return 0, fmt.Errorf("negative price %q", s)
	}
	intPart, frac, _ := strings.Cut(s, ".")
	// 2 places reaches cents, 6 more reach Scale. Pad or truncate to exactly 8.
	const shift = 8
	for len(frac) < shift {
		frac += "0"
	}
	frac = frac[:shift]
	n, ok := new(big.Int).SetString(intPart+frac, 10)
	if !ok {
		return 0, fmt.Errorf("unparseable price %q", s)
	}
	if !n.IsInt64() {
		return 0, fmt.Errorf("price %q does not fit in int64 cents", s)
	}
	c := n.Int64()
	if c <= 0 {
		return 0, fmt.Errorf("non-positive price %q", s)
	}
	return c, nil
}

// Coinbase prices from api.coinbase.com's public spot endpoint.
func Coinbase(client *http.Client) Source {
	return httpSource{
		name:   "coinbase",
		client: orDefault(client),
		url:    func(sym string) string { return "https://api.coinbase.com/v2/prices/" + sym + "-USD/spot" },
		parse: func(b []byte) (string, error) {
			var r struct {
				Data struct {
					Amount string `json:"amount"`
				} `json:"data"`
			}
			if err := json.Unmarshal(b, &r); err != nil {
				return "", err
			}
			if r.Data.Amount == "" {
				return "", fmt.Errorf("no amount in response")
			}
			return r.Data.Amount, nil
		},
	}
}

// krakenPair maps a symbol to Kraken's pair name.
//
// Kraken spells BTC as XBT and prefixes its oldest assets with X/Z, so the pair
// is NOT derivable from the symbol — "BTCUSD" and "XRPUSD" are not what it
// answers to. A wrong pair returns an error rather than a wrong price, but the
// table is what keeps the source usable at all.
var krakenPair = map[string]string{
	"BTC": "XXBTZUSD",
	"XRP": "XXRPZUSD",
	"TON": "TONUSD",
	"ETH": "XETHZUSD",
	"SOL": "SOLUSD",
}

// Kraken prices from api.kraken.com's public ticker.
func Kraken(client *http.Client) Source {
	return httpSource{
		name:   "kraken",
		client: orDefault(client),
		url: func(sym string) string {
			return "https://api.kraken.com/0/public/Ticker?pair=" + krakenPair[sym]
		},
		parse: func(b []byte) (string, error) {
			var r struct {
				Error  []string                   `json:"error"`
				Result map[string]json.RawMessage `json:"result"`
			}
			if err := json.Unmarshal(b, &r); err != nil {
				return "", err
			}
			if len(r.Error) > 0 {
				return "", fmt.Errorf("%s", strings.Join(r.Error, "; "))
			}
			// Exactly one pair, under a key Kraken chooses. More than one means
			// the query was ambiguous and neither answer is safe to pick.
			if len(r.Result) != 1 {
				return "", fmt.Errorf("%d pairs in result, want exactly 1", len(r.Result))
			}
			for _, v := range r.Result {
				var t struct {
					C []string `json:"c"` // last trade: [price, lot volume]
				}
				if err := json.Unmarshal(v, &t); err != nil {
					return "", err
				}
				if len(t.C) == 0 {
					return "", fmt.Errorf("no last-trade price")
				}
				return t.C[0], nil
			}
			return "", fmt.Errorf("no pair in result")
		},
	}
}

func orDefault(c *http.Client) *http.Client {
	if c != nil {
		return c
	}
	// A price read sits inside a deposit scan that runs on a timer. It must fail
	// fast rather than hold the pass open; a refusal costs one retry.
	return &http.Client{Timeout: 8 * time.Second}
}

// Default is the oracle the rail runs with.
func Default() (*Oracle, error) { return New(Coinbase(nil), Kraken(nil)) }
