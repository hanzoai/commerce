// Package bitcoinrpc reads Bitcoin for the deposit rail.
//
// Bitcoin has no accounts: a transaction creates OUTPUTS, so "our address was
// paid" means "an output pays our script". Two consequences drive this file —
// one transaction may pay us twice (see outputsTo), and an unconfirmed
// transaction has no height (see paidTo).
//
// Esplora rather than Bitcoin Core: Core cannot answer "which transactions paid
// this address" without an address index most operators have not enabled.
package bitcoinrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hanzoai/commerce/billing/depositwatch"
)

// Decimals is 8 by protocol. A native coin has no contract to read it from.
const Decimals = 8

// maxAddressPages bounds the walk. Exceeding it errors rather than truncating:
// a partial scan silently misses deposits.
const maxAddressPages = 100

// Client reads one Esplora endpoint.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient builds a read client against an Esplora API root, e.g.
// "https://blockstream.info/api" or a self-hosted instance.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

var _ depositwatch.Reader = (*Client)(nil)

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("bitcoinrpc: GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("bitcoinrpc: GET %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return fmt.Errorf("bitcoinrpc: GET %s: HTTP %d: %s", path, resp.StatusCode, msg)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("bitcoinrpc: GET %s: unreadable response: %w", path, err)
	}
	return nil
}

// BlockNumber is the height of the chain tip.
func (c *Client) BlockNumber(ctx context.Context) (uint64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/blocks/tip/height", nil)
	if err != nil {
		return 0, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("bitcoinrpc: tip height: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
	if err != nil {
		return 0, fmt.Errorf("bitcoinrpc: tip height: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("bitcoinrpc: tip height: HTTP %d", resp.StatusCode)
	}
	// The endpoint answers a bare integer, not JSON.
	h, err := strconv.ParseUint(strings.TrimSpace(string(body)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bitcoinrpc: tip height %q is not a number: %w", strings.TrimSpace(string(body)), err)
	}
	if h == 0 {
		return 0, fmt.Errorf("bitcoinrpc: tip height is 0 — the endpoint is not synced")
	}
	return h, nil
}

// Decimals is 8, by protocol. See the package constant.
func (c *Client) Decimals(context.Context) (int, error) { return Decimals, nil }

// Symbol is BTC. There is no contract to ask and nothing that could disagree.
func (c *Client) Symbol(context.Context) (string, error) { return "BTC", nil }

// tx is the shape Esplora answers with, reduced to what a deposit needs.
type tx struct {
	TxID   string `json:"txid"`
	Status struct {
		Confirmed   bool   `json:"confirmed"`
		BlockHeight uint64 `json:"block_height"`
	} `json:"status"`
	Vout []struct {
		ScriptPubKeyAddress string `json:"scriptpubkey_address"`
		Value               int64  `json:"value"` // satoshis
	} `json:"vout"`
}

// TransfersTo returns every output that paid a watched address inside the window.
func (c *Client) TransfersTo(ctx context.Context, addrs []string, fromHeight, toHeight uint64) ([]depositwatch.Transfer, error) {
	if len(addrs) == 0 || toHeight < fromHeight {
		return nil, nil
	}
	// Two watched entries for one address have no safe answer to "which intent
	// owns this?", so they are refused.
	seen := make(map[string]bool, len(addrs))
	for _, a := range addrs {
		k := strings.TrimSpace(a)
		if k == "" {
			return nil, fmt.Errorf("bitcoinrpc: a watched deposit address is empty")
		}
		if seen[k] {
			return nil, fmt.Errorf("bitcoinrpc: address %q is watched twice", k)
		}
		seen[k] = true
	}

	var out []depositwatch.Transfer
	for addr := range seen {
		got, err := c.paidTo(ctx, addr, fromHeight, toHeight)
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return out, nil
}

// paidTo walks one address's history back through the window, newest first.
//
// Pages by LAST SEEN TXID, never by offset: history grows at the head, so an
// offset walk skips whatever confirms mid-walk, and a skipped row is a missed
// deposit.
func (c *Client) paidTo(ctx context.Context, addr string, fromHeight, toHeight uint64) ([]depositwatch.Transfer, error) {
	var out []depositwatch.Transfer
	path := "/address/" + url.PathEscape(addr) + "/txs"

	for page := 0; ; page++ {
		if page >= maxAddressPages {
			return nil, fmt.Errorf("bitcoinrpc: address %s has more history than %d pages in heights %d..%d — refusing to scan a partial window",
				addr, maxAddressPages, fromHeight, toHeight)
		}
		var txs []tx
		if err := c.get(ctx, path, &txs); err != nil {
			return nil, err
		}
		if len(txs) == 0 {
			return out, nil
		}

		lastSeen := ""
		for i := range txs {
			t := &txs[i]
			lastSeen = t.TxID
			if !t.Status.Confirmed {
				// The mempool is a claim that can be replaced, not a shallow
				// confirmation. Skip without stopping — older rows still count.
				continue
			}
			switch {
			case t.Status.BlockHeight > toHeight:
				continue // newer than the window; the next pass takes it
			case t.Status.BlockHeight < fromHeight:
				return out, nil // and so is everything behind it
			}
			out = append(out, outputsTo(t, addr)...)
		}
		if lastSeen == "" {
			return out, nil
		}
		// Esplora's confirmed-history cursor.
		path = "/address/" + url.PathEscape(addr) + "/txs/chain/" + url.PathEscape(lastSeen)
	}
}

// outputsTo turns one transaction into the credits it produced for addr.
//
// EVERY matching output is its own Transfer: two outputs paying one address are
// two pieces of value, and summing them would credit half when a wallet splits a
// payment. EventIndex is the output's OWN position, never a counter over matches
// — a counter renumbers, and renumbering a dedup key is a double credit.
func outputsTo(t *tx, addr string) []depositwatch.Transfer {
	var out []depositwatch.Transfer
	for i := range t.Vout {
		v := &t.Vout[i]
		if v.ScriptPubKeyAddress != addr || v.Value <= 0 {
			continue
		}
		out = append(out, depositwatch.Transfer{
			To:         addr,
			Units:      big.NewInt(v.Value), // satoshis
			TxHash:     t.TxID,
			EventIndex: uint64(i),
			Block:      t.Status.BlockHeight,
		})
	}
	return out
}
