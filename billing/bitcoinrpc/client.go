// Package bitcoinrpc reads Bitcoin for the crypto deposit rail.
//
// It is the rail's first UTXO chain, and that is the whole of what makes it
// different. Everywhere else a deposit is a change to an ACCOUNT — an ERC-20
// balance, a Solana token account, a TON message value, an XRPL delivery — and
// the reader's job is to notice the account moved. Bitcoin has no accounts: a
// transaction creates OUTPUTS, each locked to a script, and "our address
// received money" means "an output in this transaction pays our script".
//
// TWO CONSEQUENCES THAT ARE NOT OPTIONAL:
//
//	ONE TRANSACTION MAY PAY US TWICE. Two outputs in one transaction can both
//	pay the same address, and they are two separate pieces of value. Each is its
//	own Transfer with the OUTPUT INDEX as its EventIndex, so the ledger's dedup
//	key names the output rather than the transaction. Summing them into one
//	credit would work until the day a wallet splits a payment, and then it would
//	silently credit half.
//
//	AN UNCONFIRMED TRANSACTION HAS NO HEIGHT. Bitcoin's mempool is not a shorter
//	confirmation — it is a claim that has not happened yet, and it can be
//	replaced. Those are skipped entirely rather than credited shallowly; the
//	watcher's own depth rule then applies to the height, the same as every other
//	chain.
//
// The wire is Esplora (blockstream.info, mempool.space, or a self-hosted
// instance) rather than Bitcoin Core's JSON-RPC, because Core cannot answer
// "which transactions paid this address" without an address index the node
// operator must have enabled and most have not. Esplora exists to answer exactly
// that question.
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

// Decimals is Bitcoin's, by protocol: 1 BTC is 10^8 satoshis.
//
// Like TON's 9 and XRP's 6, this is a constant because there is no contract to
// ask — the unit is fixed by consensus, not by a deployment. That is the one
// circumstance where this rail accepts a decimals constant instead of reading
// one from the chain.
const Decimals = 8

// maxAddressPages bounds the walk back through one address's history.
//
// An address this rail mints is used by ONE payer, so its history is short; a
// hundred pages is far past any real deposit address and is a backstop against
// paging forever on a hot address somebody configured by hand. Exceeding it is
// an ERROR rather than a truncation — a partial scan silently misses deposits.
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
	// A Bitcoin address has ONE canonical spelling per script, but the same
	// script can be written as different address types and case matters for
	// bech32. Two watched entries resolving to the same string is a
	// configuration mistake with no safe answer — "which intent owns this?" —
	// so it is refused rather than resolved.
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

// paidTo walks ONE address's history back through the window.
//
// The walk is newest-first and pages by the LAST SEEN TXID, which is Esplora's
// cursor for confirmed history. It is not an offset: an address's history grows
// at the head, so an offset-paged walk skips rows whenever a new transaction
// confirms mid-walk — and a skipped row is a missed deposit.
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
				// The mempool is not a shallower confirmation — it is a claim
				// that has not happened and can be replaced. Skip WITHOUT
				// stopping: the confirmed rows behind it are still wanted.
				continue
			}
			switch {
			case t.Status.BlockHeight > toHeight:
				continue // newer than the window; the next pass takes it
			case t.Status.BlockHeight < fromHeight:
				// Older than the window, and so is everything behind it —
				// Esplora returns confirmed history in descending height.
				return out, nil
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

// outputsTo turns ONE transaction into the credits it produced for addr.
//
// EVERY matching output is its own Transfer, indexed by its position in the
// transaction. That is what makes the ledger's dedup key name the OUTPUT: two
// outputs paying the same address in one transaction are two pieces of value,
// and collapsing them into one credit would silently pay half the day a wallet
// splits a payment.
//
// The index is the OUTPUT'S OWN position, never a counter over matches — a
// counter would renumber if the first output stopped matching, and renumbering a
// dedup key is a double credit.
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
