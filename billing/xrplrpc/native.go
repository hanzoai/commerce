package xrplrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"
)

// DropDecimals is XRP's, by protocol: 1 XRP is 10^6 drops.
//
// It is NOT Scale. An issued currency has no base unit at all — the ledger
// states a decimal number, which this package renders at 15 places — while
// native XRP is an integer count of drops and always has been. Using Scale here
// would credit 10^9 times too much.
const DropDecimals = 6

// NewNative builds a reader for NATIVE XRP — the coin itself, not an issued
// currency.
//
// It shares everything with the issued-currency reader except what a
// delivered_amount MEANS: the paging by marker, the validated-ledger checks, the
// tesSUCCESS rule and the destination tag are the same code, because on a POOLED
// chain the tag is what says whose money arrived and getting that right once is
// the whole point.
//
// WHY THIS IS SIMPLER THAN RLUSD: an issued currency needs a TRUST LINE on the
// receiving account before it can arrive at all, and a currency code is a name
// anybody may issue, so the reader must check both the code and the issuer.
// Native XRP needs neither — it is the ledger's own unit, it cannot be
// counterfeited, and any funded account can receive it. What it needs instead is
// a price, which the oracle now provides.
func NewNative(rpcURL string) *Client {
	c := &Client{
		rpcURL: rpcURL,
		// token stays zero and is never consulted: nativeAmount ignores it, and
		// Symbol below answers without asking the ledger what an issuer issues.
		http:     &http.Client{Timeout: 30 * time.Second},
		decimals: DropDecimals,
		native:   true,
	}
	c.amount = nativeAmount
	return c
}

// nativeAmount reads a delivered_amount as DROPS of native XRP.
//
// The shape IS the discriminator, and it is the whole of it: XRPL renders a
// native amount as a bare JSON STRING of drops and an issued amount as an OBJECT
// {currency, issuer, value}. So a reader that wants one simply refuses the
// other, and neither can be mistaken for the other by a malformed field.
//
// delivered_amount, never tx.Amount — the partial-payment defence the issued
// reader relies on for the same reason. A Payment may deliver LESS than it says
// it sends, and crediting the stated amount is how an exchange gets drained.
func nativeAmount(m *txMeta, hash string) (*big.Int, bool, error) {
	if len(m.DeliveredAmount) == 0 || string(m.DeliveredAmount) == "null" {
		return nil, false, nil // not a delivering payment
	}
	var s string
	if err := json.Unmarshal(m.DeliveredAmount, &s); err != nil {
		// An object is an ISSUED currency — RLUSD, USDC, somebody's token. Not
		// this reader's asset, and not an error.
		return nil, false, nil
	}
	if s == "unavailable" {
		// The ledger says it cannot report what was delivered. Refused rather
		// than guessed: this is the one case where the amount is unknowable and
		// crediting anything would be inventing it.
		return nil, false, fmt.Errorf("xrplrpc: transaction %s does not report a delivered amount", hash)
	}
	drops, ok := new(big.Int).SetString(strings.TrimSpace(s), 10)
	if !ok || drops.Sign() < 0 {
		return nil, false, fmt.Errorf("xrplrpc: transaction %s delivered %q, which is not an amount in drops", hash, s)
	}
	if drops.Sign() == 0 {
		return nil, false, nil
	}
	return drops, true, nil
}

// nativeSymbol is what Symbol answers for a native reader. There is no issuer to
// ask and nothing that could disagree.
const nativeSymbol = "XRP"

// symbolFor returns this reader's ticker without a ledger round-trip when the
// asset is native.
func (c *Client) symbolFor(ctx context.Context) (string, bool) {
	if c.native {
		return nativeSymbol, true
	}
	_ = ctx
	return "", false
}
