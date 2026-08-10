package bitcoinrpc

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// The spend path.
//
// client.go reads what a DEPOSIT needs — which transactions paid us. A sweep
// needs three more things, and Esplora already serves all three; the read
// client simply never asked for them. Nothing new is dialled and no second
// backend appears: the same endpoint that told us money arrived tells us which
// outputs are still unspent, what the network is charging, and takes the signed
// transaction back.

// UTXO is one unspent output paying a custody address.
//
// Value is satoshis. Height is 0 for an output still in the mempool, which is
// how Confirmed reports it — an unconfirmed output is real value but it is not
// value a sweep should spend, because the transaction that created it can still
// be replaced.
type UTXO struct {
	TxID   string
	Vout   uint32
	Value  int64
	Height uint64
}

// Confirmed reports whether the output is in a block.
func (u UTXO) Confirmed() bool { return u.Height > 0 }

// Unspent lists the outputs still available to addr, oldest first.
//
// The order is fixed rather than left to the server because it becomes the
// INPUT ORDER of the transaction we sign, and an input order that varies
// between two calls makes two different transactions out of one intent — which
// on a retry means two signatures over two payloads, both valid, spending the
// same coins.
func (c *Client) Unspent(ctx context.Context, addr string) ([]UTXO, error) {
	var raw []struct {
		TxID   string `json:"txid"`
		Vout   uint32 `json:"vout"`
		Value  int64  `json:"value"`
		Status struct {
			Confirmed   bool   `json:"confirmed"`
			BlockHeight uint64 `json:"block_height"`
		} `json:"status"`
	}
	if err := c.get(ctx, "/address/"+url.PathEscape(addr)+"/utxo", &raw); err != nil {
		return nil, err
	}
	out := make([]UTXO, 0, len(raw))
	for _, u := range raw {
		if u.Value <= 0 {
			continue
		}
		h := uint64(0)
		if u.Status.Confirmed {
			h = u.Status.BlockHeight
		}
		out = append(out, UTXO{TxID: u.TxID, Vout: u.Vout, Value: u.Value, Height: h})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Height != out[j].Height {
			// Unconfirmed (height 0) sorts last, not first.
			if out[i].Height == 0 {
				return false
			}
			if out[j].Height == 0 {
				return true
			}
			return out[i].Height < out[j].Height
		}
		if out[i].TxID != out[j].TxID {
			return out[i].TxID < out[j].TxID
		}
		return out[i].Vout < out[j].Vout
	})
	return out, nil
}

// FeeRate is what the network is charging, in satoshis per virtual byte, for
// confirmation within blocks.
//
// Esplora answers a map of target → rate and does not promise every target is
// present, so the nearest target at or beyond the one asked for is used. A
// missing answer is an error and never a default: a fee rate invented locally
// is either a transaction that never confirms or one that overpays without
// limit, and neither is a decision this package should make silently.
func (c *Client) FeeRate(ctx context.Context, blocks int) (float64, error) {
	if blocks < 1 {
		return 0, fmt.Errorf("bitcoinrpc: fee target must be at least 1 block, got %d", blocks)
	}
	var est map[string]float64
	if err := c.get(ctx, "/fee-estimates", &est); err != nil {
		return 0, err
	}
	best, bestTarget := 0.0, 0
	for k, v := range est {
		var target int
		if _, err := fmt.Sscanf(k, "%d", &target); err != nil || v <= 0 {
			continue
		}
		if target < blocks {
			continue
		}
		if bestTarget == 0 || target < bestTarget {
			best, bestTarget = v, target
		}
	}
	if bestTarget == 0 {
		return 0, fmt.Errorf("bitcoinrpc: no fee estimate for %d blocks or sooner in %v", blocks, est)
	}
	// Below one satoshi per vbyte a transaction is under the default relay
	// minimum and most nodes will not forward it at all.
	if best < 1 {
		best = 1
	}
	return best, nil
}

// Send submits a signed transaction and returns the txid the endpoint computed.
//
// Esplora takes the raw hex as the request BODY and answers the txid as bare
// text, not JSON.
func (c *Client) Send(ctx context.Context, raw []byte) (string, error) {
	body := strings.NewReader(hex.EncodeToString(raw))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/tx", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "text/plain")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("bitcoinrpc: POST /tx: %w", err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return "", fmt.Errorf("bitcoinrpc: POST /tx: %w", err)
	}
	txid := strings.TrimSpace(string(out))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bitcoinrpc: POST /tx: HTTP %d: %s", resp.StatusCode, txid)
	}
	// A txid is 32 bytes of hex and nothing else. Esplora reports some
	// rejections with a 200 and a prose body, so the SHAPE is what confirms the
	// transaction was accepted rather than the status code.
	if len(txid) != 64 {
		return "", fmt.Errorf("bitcoinrpc: POST /tx did not return a txid: %s", txid)
	}
	if _, err := hex.DecodeString(txid); err != nil {
		return "", fmt.Errorf("bitcoinrpc: POST /tx returned %q, which is not a txid", txid)
	}
	return txid, nil
}
