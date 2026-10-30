// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/luxfi/crypto"
	"github.com/luxfi/geth/core/types"

	"github.com/hanzoai/commerce/billing/husdindex"
	"github.com/hanzoai/commerce/payment/eip3009"
)

// node is an EVM chain carrying one FiatTokenV2 (USDC), over JSON-RPC, as far
// as the rail can observe it. It keeps the token's own rules for
// transferWithAuthorization — the EIP-712 signature over the token's domain,
// the window, one use per (from, nonce), the balance — and the chain's for a
// relayer's transaction: an EIP-1559 transaction signed for this chain id,
// from an account that can pay its gas, at the next nonce. Anyone may submit
// an authorization (use), and blocks are made only when a test says so (mine),
// so depth is the test's to control.
type node struct {
	mu      sync.Mutex
	chainID int64
	usdc    string
	domain  eip3009.Domain
	now     func() time.Time

	head    uint64
	usdcOf  map[string]*big.Int // address → base units
	ethOf   map[string]*big.Int // address → wei
	nonceOf map[string]uint64
	used    map[string]bool // from|nonce
	mined   map[string]*mined
	pool    map[string]*authorization
	pooled  map[string]*pooled // from|nonce → the transaction holding that nonce in the pool

	// Knobs a test turns.
	safeLag   uint64 // how far the safe head trails the latest
	finalLag  uint64 // how far the finalized head trails the latest
	span      uint64 // the widest eth_getLogs range answered (Base Sepolia's 200)
	logCalls  int    // eth_getLogs calls answered
	separator string // a DOMAIN_SEPARATOR other than the domain's, when set
	drop      bool   // accept a relayer transaction and forget it
	hold      bool   // keep a relayer transaction in the pool, unmined
	down      bool   // answer every call with an error
	sends     int    // relayer transactions accepted
}

// pooled is a relayer transaction waiting in the pool, by its sender's nonce:
// what a replacement has to outbid.
type pooled struct {
	hash     string
	tip, cap *big.Int
}

// mined is a transaction in a block: its success and the logs it emitted.
type mined struct {
	block   uint64
	success bool
	logs    []map[string]any
	undo    func()
}

func newNode(chainID int64, usdc string, domain eip3009.Domain, now func() time.Time) *node {
	return &node{
		chainID: chainID, usdc: usdc, domain: domain, now: now, head: 1000, safeLag: 2, finalLag: 6, span: 200,
		usdcOf: map[string]*big.Int{}, ethOf: map[string]*big.Int{}, nonceOf: map[string]uint64{},
		used: map[string]bool{}, mined: map[string]*mined{}, pool: map[string]*authorization{}, pooled: map[string]*pooled{},
	}
}

func (n *node) set(fn func(*node)) {
	n.mu.Lock()
	defer n.mu.Unlock()
	fn(n)
}

func (n *node) fund(addr string, cents int64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	a := strings.ToLower(addr)
	if n.usdcOf[a] == nil {
		n.usdcOf[a] = new(big.Int)
	}
	n.usdcOf[a].Add(n.usdcOf[a], big.NewInt(cents*10_000))
}

func (n *node) gas(addr string, wei *big.Int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.ethOf[strings.ToLower(addr)] = new(big.Int).Set(wei)
}

func (n *node) usdcBalance(addr string) int64 {
	n.mu.Lock()
	defer n.mu.Unlock()
	b := n.usdcOf[strings.ToLower(addr)]
	if b == nil {
		return 0
	}
	return new(big.Int).Div(b, big.NewInt(10_000)).Int64()
}

// mine makes k more blocks.
func (n *node) mine(k uint64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.head += k
}

func (n *node) relayed() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.sends
}

// reorg takes a mined transaction out of the chain, undoing what it did.
func (n *node) reorg(hash string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if m, ok := n.mined[strings.ToLower(hash)]; ok {
		m.undo()
		delete(n.mined, strings.ToLower(hash))
	}
}

// authorization is transferWithAuthorization's arguments.
type authorization struct {
	from, to                [20]byte
	value                   *big.Int
	validAfter, validBefore *big.Int
	nonce, r, s             [32]byte
	v                       byte
}

func decodeCall(data []byte) (authorization, error) {
	var a authorization
	if len(data) != 4+9*32 || hex.EncodeToString(data[:4]) != transferSelector {
		return a, fmt.Errorf("not transferWithAuthorization")
	}
	w := func(i int) []byte { return data[4+32*i : 4+32*(i+1)] }
	copy(a.from[:], w(0)[12:])
	copy(a.to[:], w(1)[12:])
	a.value = new(big.Int).SetBytes(w(2))
	a.validAfter = new(big.Int).SetBytes(w(3))
	a.validBefore = new(big.Int).SetBytes(w(4))
	copy(a.nonce[:], w(5))
	vw := new(big.Int).SetBytes(w(6))
	if !vw.IsUint64() || vw.Uint64() > 255 {
		return a, fmt.Errorf("v out of range")
	}
	a.v = byte(vw.Uint64())
	copy(a.r[:], w(7))
	copy(a.s[:], w(8))
	return a, nil
}

// check is FiatTokenV2's own order of checks for one authorization, with the
// lock held. "" means it would succeed.
func (n *node) check(a authorization) string {
	t := big.NewInt(n.now().Unix())
	key := eip3009.Hex(a.from[:]) + "|" + eip3009.Hex(a.nonce[:])
	switch {
	case t.Cmp(a.validAfter) <= 0:
		return "FiatTokenV2: authorization is not yet valid"
	case t.Cmp(a.validBefore) >= 0:
		return "FiatTokenV2: authorization is expired"
	case n.used[key]:
		return "FiatTokenV2: authorization is used or canceled"
	}
	p, err := eip3009.Authorization{
		From: eip3009.Hex(a.from[:]), To: eip3009.Hex(a.to[:]), Value: a.value.String(),
		ValidAfter: a.validAfter.String(), ValidBefore: a.validBefore.String(), Nonce: eip3009.Hex(a.nonce[:]),
	}.Parse()
	if err != nil {
		return "FiatTokenV2: invalid signature"
	}
	digest, err := n.domain.Digest(p)
	if err != nil {
		return "FiatTokenV2: invalid signature"
	}
	sig := append(append(append([]byte{}, a.r[:]...), a.s[:]...), a.v)
	signer, err := eip3009.Recover(digest, eip3009.Hex(sig))
	if err != nil || signer != a.from {
		return "FiatTokenV2: invalid signature"
	}
	held := n.usdcOf[eip3009.Hex(a.from[:])]
	if held == nil || held.Cmp(a.value) < 0 {
		return "ERC20: transfer amount exceeds balance"
	}
	return ""
}

// execute applies an authorization in a new block, from submitter, and
// returns what was mined. The lock is held.
func (n *node) execute(hash string, a authorization) *mined {
	n.head++
	m := &mined{block: n.head}
	if why := n.check(a); why != "" {
		m.undo = func() {}
		n.mined[hash] = m
		return m
	}
	from, to := eip3009.Hex(a.from[:]), eip3009.Hex(a.to[:])
	key := from + "|" + eip3009.Hex(a.nonce[:])
	n.used[key] = true
	n.usdcOf[from].Sub(n.usdcOf[from], a.value)
	if n.usdcOf[to] == nil {
		n.usdcOf[to] = new(big.Int)
	}
	n.usdcOf[to].Add(n.usdcOf[to], a.value)
	value := a.value
	m.success = true
	m.undo = func() {
		delete(n.used, key)
		n.usdcOf[from].Add(n.usdcOf[from], value)
		n.usdcOf[to].Sub(n.usdcOf[to], value)
	}
	word := func(b []byte) string { return eip3009.Hex(pad(b)) }
	m.logs = []map[string]any{
		{"address": n.usdc, "topics": []string{usedTopic, word(a.from[:]), eip3009.Hex(a.nonce[:])}, "data": "0x"},
		{"address": n.usdc, "topics": []string{husdindex.TransferTopic0, word(a.from[:]), word(a.to[:])}, "data": eip3009.Hex(pad(a.value.Bytes()))},
	}
	n.mined[hash] = m
	return m
}

// use submits an authorization from outside the rail — the payer, or anyone
// holding the signature — and mines it.
func (n *node) use(sub Submission) string {
	n.mu.Lock()
	defer n.mu.Unlock()
	p, _ := sub.Authorization.Parse()
	sig, _ := hex.DecodeString(strings.TrimPrefix(sub.Signature, "0x"))
	a := authorization{from: p.From, to: p.To, value: p.Value,
		validAfter: new(big.Int).SetUint64(p.ValidAfter), validBefore: new(big.Int).SetUint64(p.ValidBefore), nonce: p.Nonce}
	copy(a.r[:], sig[:32])
	copy(a.s[:], sig[32:64])
	a.v = sig[64]
	if a.v < 27 {
		a.v += 27
	}
	hash := eip3009.Hex(crypto.Keccak256([]byte(fmt.Sprintf("outside-%d", len(n.mined)))))
	n.execute(hash, a)
	return hash
}

// cancel is the payer calling cancelAuthorization: the nonce is spent, and the
// chain records AuthorizationCanceled rather than a use.
func (n *node) cancel(sub Submission) string {
	n.mu.Lock()
	defer n.mu.Unlock()
	p, _ := sub.Authorization.Parse()
	n.head++
	n.used[eip3009.Hex(p.From[:])+"|"+eip3009.Hex(p.Nonce[:])] = true
	hash := eip3009.Hex(crypto.Keccak256([]byte(fmt.Sprintf("cancel-%d", len(n.mined)))))
	n.mined[hash] = &mined{block: n.head, success: true, undo: func() {}, logs: []map[string]any{
		{"address": n.usdc, "topics": []string{canceledTopic, eip3009.Hex(pad(p.From[:])), eip3009.Hex(p.Nonce[:])}, "data": "0x"},
	}}
	return hash
}

func (n *node) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     json.RawMessage   `json:"id"`
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	result, rpcErr := n.answer(req.Method, req.Params)
	w.Header().Set("Content-Type", "application/json")
	out := map[string]any{"jsonrpc": "2.0", "id": req.ID}
	if rpcErr != "" {
		out["error"] = map[string]any{"code": 3, "message": rpcErr}
	} else {
		out["result"] = result
	}
	_ = json.NewEncoder(w).Encode(out)
}

func hexOf(v uint64) string { return "0x" + new(big.Int).SetUint64(v).Text(16) }

func (n *node) answer(method string, params []json.RawMessage) (any, string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.down {
		return nil, "node unavailable"
	}
	str := func(i int) string {
		var s string
		_ = json.Unmarshal(params[i], &s)
		return strings.ToLower(s)
	}
	switch method {
	case "eth_chainId":
		return hexOf(uint64(n.chainID)), ""
	case "eth_blockNumber":
		return hexOf(n.head), ""
	case "eth_getTransactionCount":
		return hexOf(n.nonceOf[str(0)]), ""
	case "eth_maxPriorityFeePerGas":
		return hexOf(1_000_000), ""
	case "eth_getBlockByNumber":
		at := n.head
		switch str(0) {
		case "safe":
			at -= min(at, n.safeLag)
		case "finalized":
			at -= min(at, n.finalLag)
		}
		return map[string]string{"number": hexOf(at), "baseFeePerGas": hexOf(10_000_000)}, ""
	case "eth_getBalance":
		b := n.ethOf[str(0)]
		if b == nil {
			b = new(big.Int)
		}
		return "0x" + b.Text(16), ""
	case "eth_estimateGas":
		return hexOf(80_000), ""
	case "eth_call":
		var arg struct{ To, Data string }
		_ = json.Unmarshal(params[0], &arg)
		if strings.ToLower(arg.To) != n.usdc {
			return "0x", ""
		}
		data, _ := hex.DecodeString(strings.TrimPrefix(arg.Data, "0x"))
		return n.view(data)
	case "eth_sendRawTransaction":
		return n.send(str(0))
	case "eth_getTransactionReceipt":
		m, ok := n.mined[str(0)]
		if !ok {
			return nil, ""
		}
		status := "0x0"
		if m.success {
			status = "0x1"
		}
		logs := []map[string]any{}
		for i, l := range m.logs {
			c := map[string]any{"blockNumber": hexOf(m.block), "transactionHash": str(0), "logIndex": hexOf(uint64(i))}
			for k, v := range l {
				c[k] = v
			}
			logs = append(logs, c)
		}
		return map[string]any{"status": status, "blockNumber": hexOf(m.block), "logs": logs}, ""
	case "eth_getTransactionByHash":
		h := str(0)
		if _, ok := n.mined[h]; ok || n.pool[h] != nil {
			return map[string]string{"hash": h}, ""
		}
		return nil, ""
	case "eth_getLogs":
		var f struct {
			Address   string
			FromBlock string
			ToBlock   string
			Topics    []json.RawMessage
		}
		_ = json.Unmarshal(params[0], &f)
		from, _ := new(big.Int).SetString(strings.TrimPrefix(f.FromBlock, "0x"), 16)
		to, _ := new(big.Int).SetString(strings.TrimPrefix(f.ToBlock, "0x"), 16)
		if to.Uint64()-from.Uint64()+1 > n.span {
			return nil, fmt.Sprintf("block range is too wide (maximum %d)", n.span)
		}
		n.logCalls++
		// Each topic position is null (any), one word, or a list of words (any of).
		want := make([][]string, len(f.Topics))
		for j, raw := range f.Topics {
			var one string
			var many []string
			switch {
			case string(raw) == "null":
			case json.Unmarshal(raw, &one) == nil:
				want[j] = []string{strings.ToLower(one)}
			case json.Unmarshal(raw, &many) == nil:
				for _, w := range many {
					want[j] = append(want[j], strings.ToLower(w))
				}
			}
		}
		out := []map[string]any{}
		for hash, m := range n.mined {
			if m.block < from.Uint64() || m.block > to.Uint64() || !m.success {
				continue
			}
		next:
			for i, l := range m.logs {
				if l["address"] != strings.ToLower(f.Address) {
					continue
				}
				topics := l["topics"].([]string)
				for j, any := range want {
					if any == nil {
						continue
					}
					if j >= len(topics) || !slices.Contains(any, topics[j]) {
						continue next
					}
				}
				c := map[string]any{"blockNumber": hexOf(m.block), "transactionHash": hash, "logIndex": hexOf(uint64(i))}
				for k, v := range l {
					c[k] = v
				}
				out = append(out, c)
			}
		}
		return out, ""
	}
	return nil, "method not found: " + method
}

// view answers a read-only call on the token, or simulates a
// transferWithAuthorization the way eth_call does: the revert, or nothing.
func (n *node) view(data []byte) (any, string) {
	if len(data) < 4 {
		return "0x", ""
	}
	word := func(v *big.Int) string { return eip3009.Hex(pad(v.Bytes())) }
	switch hex.EncodeToString(data[:4]) {
	case decimalsSelector:
		return word(big.NewInt(6)), ""
	case separatorSelector:
		if n.separator != "" {
			return n.separator, ""
		}
		sep, _ := n.domain.Separator()
		return eip3009.Hex(sep[:]), ""
	case balanceSelector:
		b := n.usdcOf[eip3009.Hex(data[4+12:4+32])]
		if b == nil {
			b = new(big.Int)
		}
		return word(b), ""
	case stateSelector:
		key := eip3009.Hex(data[4+12:4+32]) + "|" + eip3009.Hex(data[4+32:4+64])
		if n.used[key] {
			return word(big.NewInt(1)), ""
		}
		return word(new(big.Int)), ""
	case transferSelector:
		a, err := decodeCall(data)
		if err != nil {
			return nil, "execution reverted"
		}
		if why := n.check(a); why != "" {
			return nil, "execution reverted: " + why
		}
		return "0x", ""
	}
	return "0x", ""
}

// send takes a relayer's raw transaction: signed for this chain, at the
// sender's next nonce, from an account that can pay its gas. Unless a knob
// says otherwise it is mined at once, in the next block.
func (n *node) send(raw string) (any, string) {
	b, err := hex.DecodeString(strings.TrimPrefix(raw, "0x"))
	if err != nil {
		return nil, "invalid raw transaction"
	}
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(b); err != nil {
		return nil, "rlp: " + err.Error()
	}
	if tx.ChainId().Int64() != n.chainID {
		return nil, "invalid chain id"
	}
	sender, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		return nil, "invalid sender"
	}
	from := strings.ToLower(sender.Hex())
	slot := fmt.Sprintf("%s|%d", from, tx.Nonce())
	old := n.pooled[slot]
	switch {
	case old != nil:
		// A replacement: geth takes it only at 10% more on both fees.
		ten := func(v *big.Int) *big.Int {
			return new(big.Int).Div(new(big.Int).Mul(v, big.NewInt(110)), big.NewInt(100))
		}
		if tx.GasTipCap().Cmp(ten(old.tip)) < 0 || tx.GasFeeCap().Cmp(ten(old.cap)) < 0 {
			return nil, "replacement transaction underpriced"
		}
	case tx.Nonce() != n.nonceOf[from]:
		return nil, "nonce too low"
	}
	cost := new(big.Int).Mul(new(big.Int).SetUint64(tx.Gas()), tx.GasFeeCap())
	if eth := n.ethOf[from]; eth == nil || eth.Cmp(cost) < 0 {
		return nil, "insufficient funds for gas * price + value"
	}
	if tx.To() == nil || strings.ToLower(tx.To().Hex()) != n.usdc {
		return nil, "not the token"
	}
	a, err := decodeCall(tx.Data())
	if err != nil {
		return nil, err.Error()
	}
	hash := strings.ToLower(tx.Hash().Hex())
	if eip3009.Hex(crypto.Keccak256(b)) != hash {
		return nil, "hash mismatch"
	}
	if old != nil {
		delete(n.pool, old.hash)
		delete(n.pooled, slot)
	} else {
		n.nonceOf[from]++
	}
	n.sends++
	switch {
	case n.drop:
		return hash, ""
	case n.hold:
		n.pool[hash] = &a
		n.pooled[slot] = &pooled{hash: hash, tip: tx.GasTipCap(), cap: tx.GasFeeCap()}
		return hash, ""
	}
	n.execute(hash, a)
	return hash, ""
}

// release mines every transaction held in the pool.
func (n *node) release() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.hold = false
	for h, a := range n.pool {
		n.execute(h, *a)
		delete(n.pool, h)
	}
	n.pooled = map[string]*pooled{}
}
