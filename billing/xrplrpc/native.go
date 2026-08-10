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

// DropDecimals is 6: 1 XRP is 10^6 drops. NOT Scale — an issued currency has no
// base unit and is rendered at 15 places, so Scale here credits 10^9 too much.
const DropDecimals = 6

// NewNative builds a reader for native XRP.
//
// Shares everything with the issued reader except what a delivered_amount means:
// marker paging, validated-ledger checks, tesSUCCESS and the destination TAG are
// the same code — on a pooled chain the tag is what says whose money arrived.
//
// Simpler than RLUSD: no trust line to set up, and no issuer to check, since XRP
// cannot be counterfeited.
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

// nativeAmount reads a delivered_amount as drops.
//
// THE SHAPE IS THE DISCRIMINATOR: native is a bare JSON STRING of drops, issued
// is an OBJECT. Each reader refuses the other's shape.
//
// delivered_amount, never tx.Amount: a Payment may deliver LESS than it says it
// sends, and crediting the stated amount is how an exchange gets drained.
func nativeAmount(m *txMeta, hash string) (*big.Int, bool, error) {
	if len(m.DeliveredAmount) == 0 || string(m.DeliveredAmount) == "null" {
		return nil, false, nil // not a delivering payment
	}
	var s string
	if err := json.Unmarshal(m.DeliveredAmount, &s); err != nil {
		return nil, false, nil // an object is an issued currency, not this asset
	}
	if s == "unavailable" {
		// The one case where the amount is unknowable. Crediting anything would
		// be inventing it.
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

const nativeSymbol = "XRP"

// symbolFor answers without a ledger round-trip when the asset is native: there
// is no issuer to ask.
func (c *Client) symbolFor(ctx context.Context) (string, bool) {
	if c.native {
		return nativeSymbol, true
	}
	_ = ctx
	return "", false
}
