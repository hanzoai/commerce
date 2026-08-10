// Package husdindex is Step 3 of the chain-backed credit ledger: it makes the
// on-chain HUSD balance the source of truth and the commerce DB a cache.
//
// It has two decomplected halves:
//   - client.go: the repo's one minimal EVM JSON-RPC client (net/http,
//     encoding/json). It reads what the indexer needs — block height, ERC-20
//     balanceOf, Transfer logs — and it carries what a spend needs on top:
//     chain id, nonce, fees, gas estimate, and eth_sendRawTransaction. Still no
//     geth and no cgo, because none of that requires cryptography; building and
//     signing the bytes it carries is billing/custody/evm's job.
//   - index.go: the Sync projector — scan Transfer events INTO org addresses and
//     project each, idempotently, into the ledger tagged by its off-chain
//     issuance bucket. Pure logic over small interfaces, unit-tested with fakes.
//
// The invariant it upholds: an org's indexed balance == its on-chain
// balanceOf(address), reconciled to the cent (Σ projected credits − debits).
package husdindex

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// TransferTopic0 is keccak256("Transfer(address,address,uint256)") — the ERC-20
// Transfer event signature topic. Verified against luxfi/crypto.Keccak256 in the
// tests so a typo can never silently mis-filter logs.
const TransferTopic0 = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

// balanceOfSelector is the 4-byte selector for ERC-20 balanceOf(address).
const balanceOfSelector = "0x70a08231"

// decimalsSelector and symbolSelector are the ERC-20 metadata reads. They exist
// so a CONFIGURED token address is SELF-VERIFYING: the chain is asked what the
// contract at that address actually is, instead of a constant in our source
// being trusted. Getting decimals wrong by 12 credits 10^12 times too much, and
// a mistyped address is a token we would price at par — neither is survivable on
// a money path, and neither is detectable by reading our own config back.
const (
	decimalsSelector = "0x313ce567"
	symbolSelector   = "0x95d89b41"
)

// Client reads HUSD state over JSON-RPC. It is safe for concurrent use.
type Client struct {
	rpcURL    string
	tokenAddr string
	http      *http.Client
	id        atomic.Int64
}

// NewClient builds a read client for the given RPC endpoint + HUSD token.
func NewClient(rpcURL, tokenAddr string) *Client {
	return &Client{
		rpcURL:    rpcURL,
		tokenAddr: strings.ToLower(tokenAddr),
		http:      &http.Client{Timeout: 30 * time.Second},
	}
}

type rpcReq struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type rpcResp struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) call(ctx context.Context, method string, params []any) (json.RawMessage, error) {
	body, err := json.Marshal(rpcReq{JSONRPC: "2.0", ID: c.id.Add(1), Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.rpcURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("husdindex: rpc %s: %w", method, err)
	}
	defer resp.Body.Close()
	var out rpcResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("husdindex: rpc %s decode: %w", method, err)
	}
	if out.Error != nil {
		return nil, fmt.Errorf("husdindex: rpc %s error %d: %s", method, out.Error.Code, out.Error.Message)
	}
	return out.Result, nil
}

// BlockNumber returns the current chain head.
func (c *Client) BlockNumber(ctx context.Context) (uint64, error) {
	raw, err := c.call(ctx, "eth_blockNumber", []any{})
	if err != nil {
		return 0, err
	}
	return hexToUint64(rawString(raw))
}

// BalanceOf returns the balance of addr in the token this client is bound to,
// in base units.
func (c *Client) BalanceOf(ctx context.Context, addr string) (*big.Int, error) {
	return c.BalanceOfToken(ctx, c.tokenAddr, addr)
}

// BalanceOfToken returns the balance of addr in an ARBITRARY token.
//
// The indexer is bound to one token for its whole life; a sweep is not — it
// meets whichever token a payer chose — and a client per token would be a
// connection pool per token. Both go through the same eth_call so there is one
// piece of ABI encoding to get right.
func (c *Client) BalanceOfToken(ctx context.Context, token, addr string) (*big.Int, error) {
	if token == "" {
		return nil, fmt.Errorf("husdindex: balanceOf needs a token address")
	}
	raw, err := c.call(ctx, "eth_call", []any{
		map[string]string{"to": strings.ToLower(token), "data": balanceOfSelector + pad32(addr)}, "latest",
	})
	if err != nil {
		return nil, err
	}
	return hexToBig(rawString(raw))
}

// Decimals returns the token contract's own decimals(). An empty return is an
// ERROR, never zero: a contract with no decimals() (a wrong address, a
// self-destructed contract, a proxy pointing nowhere) answers "0x", and reading
// that as 0 decimals would value one base unit as one whole token — the single
// most expensive way to be wrong on this path.
func (c *Client) Decimals(ctx context.Context) (int, error) {
	raw, err := c.call(ctx, "eth_call", []any{
		map[string]string{"to": c.tokenAddr, "data": decimalsSelector}, "latest",
	})
	if err != nil {
		return 0, err
	}
	s := strings.TrimPrefix(strings.TrimPrefix(rawString(raw), "0x"), "0X")
	if s == "" {
		return 0, fmt.Errorf("husdindex: %s returned no data for decimals() — not an ERC-20 token", c.tokenAddr)
	}
	n, err := hexToBig(s)
	if err != nil {
		return 0, err
	}
	// decimals is a uint8 in the ABI; anything outside that is a contract we do
	// not understand well enough to price.
	if !n.IsInt64() || n.Int64() < 0 || n.Int64() > 255 {
		return 0, fmt.Errorf("husdindex: %s reported implausible decimals %s", c.tokenAddr, n)
	}
	return int(n.Int64()), nil
}

// Symbol returns the token contract's own symbol(), so a caller can assert the
// address it was configured with holds the token it believes it holds.
func (c *Client) Symbol(ctx context.Context) (string, error) {
	raw, err := c.call(ctx, "eth_call", []any{
		map[string]string{"to": c.tokenAddr, "data": symbolSelector}, "latest",
	})
	if err != nil {
		return "", err
	}
	sym, err := decodeABIString(rawString(raw))
	if err != nil {
		return "", fmt.Errorf("husdindex: %s symbol(): %w", c.tokenAddr, err)
	}
	return sym, nil
}

// decodeABIString decodes an eth_call return holding a token symbol. Two shapes
// exist in the wild and both are accepted: the ABI dynamic string
// (offset,length,bytes — USDC, USDT, and every modern token) and a raw bytes32
// (a handful of pre-standard tokens). Any other shape is an error — an
// undecodable symbol means the caller cannot verify the contract, which is
// exactly when it must refuse rather than assume.
func decodeABIString(hexData string) (string, error) {
	h := strings.TrimPrefix(strings.TrimPrefix(hexData, "0x"), "0X")
	b, err := hex.DecodeString(h)
	if err != nil {
		return "", fmt.Errorf("undecodable return data: %w", err)
	}
	switch {
	case len(b) == 32: // bytes32, NUL-padded
		return strings.TrimRight(string(b), "\x00"), nil
	case len(b) >= 64: // dynamic string: [offset][length][bytes...]
		off := new(big.Int).SetBytes(b[:32])
		if !off.IsUint64() || off.Uint64()+32 > uint64(len(b)) {
			return "", fmt.Errorf("string offset %s out of range (%d bytes)", off, len(b))
		}
		o := off.Uint64()
		length := new(big.Int).SetBytes(b[o : o+32])
		if !length.IsUint64() || o+32+length.Uint64() > uint64(len(b)) {
			return "", fmt.Errorf("string length %s out of range (%d bytes)", length, len(b))
		}
		return string(b[o+32 : o+32+length.Uint64()]), nil
	}
	return "", fmt.Errorf("return data is %d bytes, not a string", len(b))
}

// rawLog is one eth_getLogs entry.
type rawLog struct {
	Address     string   `json:"address"`
	Topics      []string `json:"topics"`
	Data        string   `json:"data"`
	BlockNumber string   `json:"blockNumber"`
	TxHash      string   `json:"transactionHash"`
	LogIndex    string   `json:"logIndex"`
	Removed     bool     `json:"removed"`
}

// TransfersTo returns HUSD Transfer events whose recipient (topic2) is one of
// addrs, between fromBlock and toBlock (inclusive). addrs are matched
// case-insensitively.
func (c *Client) TransfersTo(ctx context.Context, addrs []string, fromBlock, toBlock uint64) ([]Transfer, error) {
	if len(addrs) == 0 || toBlock < fromBlock {
		return nil, nil
	}
	toTopics := make([]any, 0, len(addrs))
	for _, a := range addrs {
		toTopics = append(toTopics, "0x"+pad32(a))
	}
	filter := map[string]any{
		"address":   c.tokenAddr,
		"fromBlock": uintToHex(fromBlock),
		"toBlock":   uintToHex(toBlock),
		// [Transfer, from=any, to ∈ addrs]
		"topics": []any{TransferTopic0, nil, toTopics},
	}
	raw, err := c.call(ctx, "eth_getLogs", []any{filter})
	if err != nil {
		return nil, err
	}
	var logs []rawLog
	if err := json.Unmarshal(raw, &logs); err != nil {
		return nil, fmt.Errorf("husdindex: decode logs: %w", err)
	}
	out := make([]Transfer, 0, len(logs))
	for _, l := range logs {
		if l.Removed {
			continue // reorged-out
		}
		tr, err := decodeTransfer(l)
		if err != nil {
			return nil, err
		}
		out = append(out, tr)
	}
	return out, nil
}

// TransfersInTx returns the HUSD Transfer events emitted by ONE mined tx whose
// recipient is one of addrs. It reads eth_getTransactionReceipt (not eth_getLogs)
// so a just-submitted mint can be projected the instant it is mined, without
// waiting for a block-range scan. addrs match case-insensitively; only logs from
// the HUSD token contract are considered. Returns nil if the tx is not yet mined
// (no receipt) — the caller falls back to the background Sync.
func (c *Client) TransfersInTx(ctx context.Context, txHash string, addrs []string) ([]Transfer, error) {
	want := make(map[string]bool, len(addrs))
	for _, a := range addrs {
		want["0x"+pad32(a)] = true // 32-byte topic form, lowercased by pad32
	}
	raw, err := c.call(ctx, "eth_getTransactionReceipt", []any{txHash})
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil // not mined yet
	}
	var receipt struct {
		Logs []rawLog `json:"logs"`
	}
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return nil, fmt.Errorf("husdindex: decode receipt: %w", err)
	}
	out := make([]Transfer, 0, len(receipt.Logs))
	for _, l := range receipt.Logs {
		if l.Removed || strings.ToLower(l.Address) != c.tokenAddr {
			continue
		}
		if len(l.Topics) != 3 || strings.ToLower(l.Topics[0]) != TransferTopic0 {
			continue // not an ERC-20 Transfer
		}
		if !want[strings.ToLower(l.Topics[2])] {
			continue // not to one of our addresses
		}
		tr, err := decodeTransfer(l)
		if err != nil {
			return nil, err
		}
		out = append(out, tr)
	}
	return out, nil
}

// Transfer is a decoded ERC-20 Transfer event.
type Transfer struct {
	From     string // 0x, lowercased
	To       string // 0x, lowercased
	ValueWei *big.Int
	TxHash   string
	LogIndex uint64
	Block    uint64
}

// DedupKey is the idempotency key for projecting this transfer: unique per log.
func (t Transfer) DedupKey() string { return fmt.Sprintf("%s:%d", t.TxHash, t.LogIndex) }

func decodeTransfer(l rawLog) (Transfer, error) {
	if len(l.Topics) != 3 {
		return Transfer{}, fmt.Errorf("husdindex: transfer log has %d topics, want 3", len(l.Topics))
	}
	value, err := hexToBig(l.Data)
	if err != nil {
		return Transfer{}, fmt.Errorf("husdindex: transfer value: %w", err)
	}
	block, err := hexToUint64(l.BlockNumber)
	if err != nil {
		return Transfer{}, err
	}
	logIdx, err := hexToUint64(l.LogIndex)
	if err != nil {
		return Transfer{}, err
	}
	return Transfer{
		From:     topicToAddr(l.Topics[1]),
		To:       topicToAddr(l.Topics[2]),
		ValueWei: value,
		TxHash:   strings.ToLower(l.TxHash),
		LogIndex: logIdx,
		Block:    block,
	}, nil
}

// --- hex helpers (no external deps) ---

func rawString(r json.RawMessage) string { return strings.Trim(string(r), `"`) }

func hexToUint64(h string) (uint64, error) {
	h = strings.TrimPrefix(strings.TrimPrefix(h, "0x"), "0X")
	if h == "" {
		return 0, nil
	}
	n := new(big.Int)
	if _, ok := n.SetString(h, 16); !ok {
		return 0, fmt.Errorf("husdindex: bad hex uint %q", h)
	}
	return n.Uint64(), nil
}

func hexToBig(h string) (*big.Int, error) {
	h = strings.TrimPrefix(strings.TrimPrefix(h, "0x"), "0X")
	if h == "" {
		return big.NewInt(0), nil
	}
	n := new(big.Int)
	if _, ok := n.SetString(h, 16); !ok {
		return nil, fmt.Errorf("husdindex: bad hex bigint %q", h)
	}
	return n, nil
}

func uintToHex(n uint64) string { return "0x" + new(big.Int).SetUint64(n).Text(16) }

// pad32 left-pads a 20-byte address to a 32-byte (64-hex) word.
func pad32(addr string) string {
	a := strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(addr, "0x"), "0X"))
	if len(a) < 64 {
		a = strings.Repeat("0", 64-len(a)) + a
	}
	return a
}

// topicToAddr extracts the low 20 bytes of a 32-byte topic word as a 0x address.
func topicToAddr(topic string) string {
	t := strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(topic, "0x"), "0X"))
	if len(t) >= 40 {
		t = t[len(t)-40:]
	}
	return "0x" + t
}

// --- The spend path -----------------------------------------------------
//
// Everything above reads. What follows is what a SWEEP needs on top: the state
// that goes into an unsigned transaction (chain id, nonce, fees, gas), and the
// one call that submits a signed one.
//
// They live here rather than in a second client because this is the repo's one
// EVM JSON-RPC client, and two of those would be two places to fix a header, a
// timeout or an error shape. The property that made this file worth keeping
// separate is untouched: it is still net/http and encoding/json and nothing
// else. No geth, no cgo — because none of this needs cryptography. Building and
// signing the bytes these methods carry is billing/custody/evm's job, and that
// is where luxfi/geth is linked.

// ChainID asks the node which chain it is.
//
// It is asked rather than configured because the chain id is what makes a
// signature unreplayable on any OTHER chain (EIP-155), and the failure mode of
// getting it from a constant is that a signature valid on Base is also valid on
// Ethereum. A node cannot be wrong about its own id; our config can.
func (c *Client) ChainID(ctx context.Context) (*big.Int, error) {
	raw, err := c.call(ctx, "eth_chainId", []any{})
	if err != nil {
		return nil, err
	}
	return hexToBig(rawString(raw))
}

// Nonce returns the next nonce for addr, counting transactions already in the
// mempool ("pending") rather than only mined ones. A sweep that used "latest"
// would reuse the nonce of a transfer it had just broadcast and replace it.
func (c *Client) Nonce(ctx context.Context, addr string) (uint64, error) {
	raw, err := c.call(ctx, "eth_getTransactionCount", []any{strings.ToLower(addr), "pending"})
	if err != nil {
		return 0, err
	}
	return hexToUint64(rawString(raw))
}

// NativeBalance returns addr's balance of the chain's own coin, in wei. This is
// the gas budget: on an EVM chain a token cannot pay for its own transfer.
func (c *Client) NativeBalance(ctx context.Context, addr string) (*big.Int, error) {
	raw, err := c.call(ctx, "eth_getBalance", []any{strings.ToLower(addr), "latest"})
	if err != nil {
		return nil, err
	}
	return hexToBig(rawString(raw))
}

// BaseFee returns the pending block's base fee per gas.
//
// It reads the PENDING block, not the latest one: base fee is set per block, so
// by the time a transaction built against the latest block is mined the real
// base fee has already moved, by up to 12.5% per block. The pending block's
// value is the one the transaction will actually meet.
//
// A chain with no EIP-1559 base fee (a pre-London network) reports none; that
// is returned as zero, and the caller decides whether it can proceed.
func (c *Client) BaseFee(ctx context.Context) (*big.Int, error) {
	raw, err := c.call(ctx, "eth_getBlockByNumber", []any{"pending", false})
	if err != nil {
		return nil, err
	}
	var head struct {
		BaseFeePerGas string `json:"baseFeePerGas"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, fmt.Errorf("husdindex: decode pending block: %w", err)
	}
	if head.BaseFeePerGas == "" {
		return new(big.Int), nil
	}
	return hexToBig(head.BaseFeePerGas)
}

// Tip returns the node's suggested priority fee per gas — what the miner keeps.
//
// Not every node implements eth_maxPriorityFeePerGas, so a failure is reported
// as such and left to the caller rather than silently defaulting to zero: a
// zero tip is a transaction that is valid, cheap, and may never be mined, which
// is a worse outcome for a sweep than not sending one.
func (c *Client) Tip(ctx context.Context) (*big.Int, error) {
	raw, err := c.call(ctx, "eth_maxPriorityFeePerGas", []any{})
	if err != nil {
		return nil, err
	}
	return hexToBig(rawString(raw))
}

// EstimateGas asks the node what the transfer will cost to execute.
//
// It is asked per transfer and never assumed, because "an ERC-20 transfer costs
// 65000 gas" is false often enough to matter: the first transfer into an
// address writes a fresh storage slot and costs far more than the second, and
// tokens with hooks, fees or blocklists cost more again. An estimate that is
// too low does not fail cheaply — the transaction is mined, reverts, and the
// gas is spent anyway.
func (c *Client) EstimateGas(ctx context.Context, from, to string, value *big.Int, data []byte) (uint64, error) {
	arg := map[string]string{"from": strings.ToLower(from), "to": strings.ToLower(to)}
	if value != nil && value.Sign() > 0 {
		arg["value"] = "0x" + value.Text(16)
	}
	if len(data) > 0 {
		arg["data"] = "0x" + hex.EncodeToString(data)
	}
	raw, err := c.call(ctx, "eth_estimateGas", []any{arg})
	if err != nil {
		return 0, err
	}
	return hexToUint64(rawString(raw))
}

// Send submits a signed transaction and returns the hash the node computed for
// it.
//
// The node's hash is returned rather than one computed locally so that the two
// are compared by the caller: if they disagree, the bytes that went out are not
// the bytes that were built, and that is worth knowing before a sweep is
// recorded as done.
func (c *Client) Send(ctx context.Context, raw []byte) (string, error) {
	res, err := c.call(ctx, "eth_sendRawTransaction", []any{"0x" + hex.EncodeToString(raw)})
	if err != nil {
		return "", err
	}
	return rawString(res), nil
}
