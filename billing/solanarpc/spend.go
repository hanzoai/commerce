package solanarpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
)

// The spend path.
//
// client.go reads what a DEPOSIT needs. A sweep needs the state that goes into
// a transaction — a recent blockhash, the balances, whether the destination's
// token account exists yet — plus the two calls that hand one to the cluster.
//
// Reads here are at `processed`, not the `finalized` the deposit path uses, and
// the difference is deliberate in both directions. A deposit must be final
// before it is credited, because a credit cannot be taken back. A spend must be
// built against the LATEST state, because a blockhash more than about 150 slots
// old is expired and the transaction is rejected outright — and because a nonce
// or balance read too far behind builds a transaction the cluster has already
// moved past.

// Blockhash returns a recent blockhash and the last slot at which a transaction
// carrying it will still be accepted.
//
// Solana has no nonce. A transaction is made non-replayable by naming a recent
// blockhash, which the cluster remembers for roughly 150 slots — about a minute.
// That is the whole expiry mechanism, and it is why a signed Solana transaction
// cannot be held: sign it late, or sign it again.
func (c *Client) Blockhash(ctx context.Context) (string, uint64, error) {
	raw, err := c.call(ctx, "getLatestBlockhash", []any{map[string]any{"commitment": "finalized"}})
	if err != nil {
		return "", 0, err
	}
	var out struct {
		Value struct {
			Blockhash            string `json:"blockhash"`
			LastValidBlockHeight uint64 `json:"lastValidBlockHeight"`
		} `json:"value"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", 0, fmt.Errorf("solanarpc: decode blockhash: %w", err)
	}
	if out.Value.Blockhash == "" {
		return "", 0, fmt.Errorf("solanarpc: getLatestBlockhash returned no blockhash")
	}
	return out.Value.Blockhash, out.Value.LastValidBlockHeight, nil
}

// Lamports returns an account's balance of the native coin, which is what pays
// the fee. An SPL token cannot pay for its own transfer.
func (c *Client) Lamports(ctx context.Context, addr PublicKey) (uint64, error) {
	raw, err := c.call(ctx, "getBalance", []any{addr.String(), map[string]any{"commitment": "processed"}})
	if err != nil {
		return 0, err
	}
	var out struct {
		Value uint64 `json:"value"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return 0, fmt.Errorf("solanarpc: decode balance: %w", err)
	}
	return out.Value, nil
}

// TokenBalance returns the amount held in one token account, in base units.
//
// A missing account answers zero rather than an error, because "the deposit
// address has no token account yet" and "it has one and it is empty" are the
// same thing to a sweep: there is nothing to move.
func (c *Client) TokenBalance(ctx context.Context, account PublicKey) (*big.Int, error) {
	raw, err := c.call(ctx, "getTokenAccountBalance", []any{account.String(), map[string]any{"commitment": "processed"}})
	if err != nil {
		// The RPC reports an absent account as an invalid-param error rather
		// than a null value, so the message is what distinguishes it.
		return new(big.Int), nil
	}
	var out struct {
		Value struct {
			Amount string `json:"amount"`
		} `json:"value"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("solanarpc: decode token balance: %w", err)
	}
	n, ok := new(big.Int).SetString(out.Value.Amount, 10)
	if !ok {
		return nil, fmt.Errorf("solanarpc: token amount %q is not a number", out.Value.Amount)
	}
	return n, nil
}

// Exists reports whether an account is present on chain.
//
// It decides whether a sweep must also CREATE the destination's token account.
// Sending an SPL token to an owner whose associated account does not exist yet
// fails; the transfer has to carry the creation with it, and that costs rent
// the sender pays.
func (c *Client) Exists(ctx context.Context, addr PublicKey) (bool, error) {
	raw, err := c.call(ctx, "getAccountInfo", []any{addr.String(), map[string]any{"commitment": "processed", "encoding": "base64"}})
	if err != nil {
		return false, err
	}
	var out struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return false, fmt.Errorf("solanarpc: decode account info: %w", err)
	}
	return len(out.Value) > 0 && string(out.Value) != "null", nil
}

// Simulate runs a signed transaction against the cluster's current state
// WITHOUT submitting it, verifying the signature as it goes.
//
// This is the closest thing on any chain here to asking "would this work"
// without paying to find out. It catches everything a local check cannot: an
// expired blockhash, an account that is not what we assumed, an instruction the
// program rejects, a signature over the wrong bytes. A sweep runs it before
// every broadcast.
func (c *Client) Simulate(ctx context.Context, raw []byte) error {
	res, err := c.call(ctx, "simulateTransaction", []any{
		base64.StdEncoding.EncodeToString(raw),
		map[string]any{"commitment": "processed", "encoding": "base64", "sigVerify": true, "replaceRecentBlockhash": false},
	})
	if err != nil {
		return err
	}
	var out struct {
		Value struct {
			Err  json.RawMessage `json:"err"`
			Logs []string        `json:"logs"`
		} `json:"value"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return fmt.Errorf("solanarpc: decode simulation: %w", err)
	}
	if len(out.Value.Err) > 0 && string(out.Value.Err) != "null" {
		return fmt.Errorf("solanarpc: simulation failed: %s (logs: %v)", out.Value.Err, out.Value.Logs)
	}
	return nil
}

// Send submits a signed transaction and returns its signature, which is the id
// the cluster knows it by.
//
// preflightCommitment is left at the default so the cluster runs its own
// simulation first: a transaction rejected in preflight costs nothing, and one
// that is not rejected has already been checked twice.
func (c *Client) Send(ctx context.Context, raw []byte) (string, error) {
	res, err := c.call(ctx, "sendTransaction", []any{
		base64.StdEncoding.EncodeToString(raw),
		map[string]any{"encoding": "base64"},
	})
	if err != nil {
		return "", err
	}
	var sig string
	if err := json.Unmarshal(res, &sig); err != nil {
		return "", fmt.Errorf("solanarpc: decode send result: %w", err)
	}
	if sig == "" {
		return "", fmt.Errorf("solanarpc: sendTransaction returned no signature")
	}
	return sig, nil
}

// FeeFor asks the cluster what a message will cost, in lamports.
//
// The base fee is 5000 lamports per signature, but that is a default and not a
// rule: a cluster charges prioritisation fees on top, and the number is a
// consensus parameter rather than a constant. Asking costs one round trip and
// removes a hard-coded number from the one calculation that decides how much of
// a balance is actually movable.
func (c *Client) FeeFor(ctx context.Context, message []byte) (uint64, error) {
	raw, err := c.call(ctx, "getFeeForMessage", []any{
		base64.StdEncoding.EncodeToString(message),
		map[string]any{"commitment": "processed"},
	})
	if err != nil {
		return 0, err
	}
	var out struct {
		Value *uint64 `json:"value"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return 0, fmt.Errorf("solanarpc: decode fee: %w", err)
	}
	// A null value means the blockhash in the message has already expired, so
	// the cluster will not price it. That is a real answer and not a missing
	// one: the transaction must be rebuilt.
	if out.Value == nil {
		return 0, fmt.Errorf("solanarpc: the cluster will not price this message; its blockhash has expired")
	}
	return *out.Value, nil
}

// DecimalsOf returns an arbitrary mint's decimals.
//
// The indexer is bound to one mint for its whole life; a sweep is not — it
// meets whichever token a payer chose.
func (c *Client) DecimalsOf(ctx context.Context, mint PublicKey) (int, error) {
	if mint == c.mint {
		return c.Decimals(ctx)
	}
	return NewClient(c.rpcURL, mint).Decimals(ctx)
}
