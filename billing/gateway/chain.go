// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package gateway

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/luxfi/crypto"

	"github.com/hanzoai/commerce/billing/custody"
	"github.com/hanzoai/commerce/billing/custody/evm"
	"github.com/hanzoai/commerce/billing/husdindex"
	"github.com/hanzoai/commerce/models/remittance"
	"github.com/hanzoai/commerce/payment/eip3009"
)

// What the rail asks of FiatTokenV2 (USDC), as four-byte selectors and event
// topics. Each is written as a constant and PROVEN in the tests by hashing its
// signature: one wrong nibble calls a different function.
const (
	// transferWithAuthorization(address,address,uint256,uint256,uint256,bytes32,uint8,bytes32,bytes32)
	transferSelector = "e3ee160e"
	// authorizationState(address,bytes32)
	stateSelector = "e94a0102"
	// DOMAIN_SEPARATOR()
	separatorSelector = "3644e515"
	// decimals()
	decimalsSelector = "313ce567"
	// balanceOf(address)
	balanceSelector = "70a08231"
	// AuthorizationUsed(address indexed authorizer, bytes32 indexed nonce)
	usedTopic = "0x98de503528ee59b575ef0c0a2576a82497bfc029a5685b209e9ec333479b10a5"
	// AuthorizationCanceled(address indexed authorizer, bytes32 indexed nonce)
	canceledTopic = "0x1cdd46ff242716cdaa72d159d339a485b3438398348d68f09d7c8c0a59353d81"
)

// logSpan is the widest block range one eth_getLogs asks for: Base Sepolia's
// public endpoint refuses more than 200, and the others rate-limit wide ones.
const logSpan = 200

// link is one network: its chain, dialled on first use and checked against the
// table before anything is offered or sent on it.
type link struct {
	chain Chain

	mu      sync.Mutex
	evm     *evm.Chain
	witness *husdindex.Client
	ok      bool
	checked time.Time
	err     error
}

func (l *link) rpc() *husdindex.Client { return l.evm.RPC() }

// ready returns the network, checked against its chain within readyAge: the
// node is the chain the table names (its own chain id), the contract is USDC as
// the table describes it (decimals 6), and the token's own DOMAIN_SEPARATOR()
// is the separator of the domain the rail hands payers. The last is the one
// that matters most: a payer signs whatever domain the terms state, and a
// domain that is not the token's would collect signatures the token refuses.
func (s *Service) ready(ctx context.Context, id string) (*link, error) {
	l, ok := s.links[id]
	if !ok {
		return nil, invalid(ReasonNetwork, "network %q is not accepted; GET the terms for the networks that are", id)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := s.now()
	switch {
	case l.ok && now.Sub(l.checked) < readyAge:
		return l, nil
	case l.err != nil && now.Sub(l.checked) < failAge:
		return nil, l.err
	}
	err := l.check(ctx)
	l.ok, l.err, l.checked = err == nil, nil, now
	if err != nil {
		l.err = shut(ReasonUnavailable, "The "+l.chain.Name+" network is not answering as expected; try again shortly or use a deposit address, a card or a wire.", err)
		return nil, l.err
	}
	return l, nil
}

func (l *link) check(ctx context.Context) error {
	if l.evm == nil {
		c, err := evm.Dial(ctx, l.chain.RPC)
		if err != nil {
			return err
		}
		l.evm = c
	}
	if got := l.evm.ChainID(); got.Cmp(big.NewInt(l.chain.ChainID)) != 0 {
		return fmt.Errorf("%s: the node is chain %s, not %d", l.chain.ID, got, l.chain.ChainID)
	}
	if l.witness == nil {
		l.witness = husdindex.NewClient(l.chain.Witness, "")
	}
	if got, err := l.witness.ChainID(ctx); err != nil {
		return fmt.Errorf("%s: the witness: %w", l.chain.ID, err)
	} else if got.Cmp(big.NewInt(l.chain.ChainID)) != 0 {
		return fmt.Errorf("%s: the witness is chain %s, not %d", l.chain.ID, got, l.chain.ChainID)
	}
	dec, err := l.word(ctx, "", decimalsSelector)
	if err != nil {
		return fmt.Errorf("%s: decimals(): %w", l.chain.ID, err)
	}
	if dec.Cmp(big.NewInt(6)) != 0 {
		return fmt.Errorf("%s: %s reports %s decimals, not 6", l.chain.ID, l.chain.USDC, dec)
	}
	want, err := l.chain.Domain().Separator()
	if err != nil {
		return err
	}
	got, err := l.rpc().Call(ctx, "", l.chain.USDC, mustHex(separatorSelector))
	if err != nil {
		return fmt.Errorf("%s: DOMAIN_SEPARATOR(): %w", l.chain.ID, err)
	}
	if !bytes.Equal(got, want[:]) {
		return fmt.Errorf("%s: the token's DOMAIN_SEPARATOR() is 0x%x, the configured domain's is 0x%x", l.chain.ID, got, want)
	}
	return nil
}

// word calls a no-argument (or pre-encoded) view on the token and reads one
// uint256 word back. An empty answer is an error, never zero.
func (l *link) word(ctx context.Context, from, selector string, args ...[]byte) (*big.Int, error) {
	data := mustHex(selector)
	for _, a := range args {
		data = append(data, a...)
	}
	out, err := l.rpc().Call(ctx, from, l.chain.USDC, data)
	if err != nil {
		return nil, err
	}
	if len(out) != 32 {
		return nil, fmt.Errorf("answered %d bytes, want one word", len(out))
	}
	return new(big.Int).SetBytes(out), nil
}

// used reports whether the token has spent (from, nonce): authorizationState.
func (l *link) used(ctx context.Context, from, nonce string) (bool, error) {
	f, err := eip3009.Address(from)
	if err != nil {
		return false, err
	}
	n, err := eip3009.Nonce(nonce)
	if err != nil {
		return false, err
	}
	w, err := l.word(ctx, "", stateSelector, pad(f[:]), n[:])
	if err != nil {
		return false, err
	}
	return w.Sign() != 0, nil
}

// balance is addr's USDC balance in base units.
func (l *link) balance(ctx context.Context, addr string) (*big.Int, error) {
	a, err := eip3009.Address(addr)
	if err != nil {
		return nil, err
	}
	return l.word(ctx, "", balanceSelector, pad(a[:]))
}

// transferCall is the calldata of transferWithAuthorization for r, with r's
// canonical signature split into v, r and s.
func transferCall(r *remittance.Remittance) ([]byte, error) {
	p, err := eip3009.Authorization{
		From: r.From, To: r.To, Value: r.Value,
		ValidAfter: r.ValidAfter, ValidBefore: r.ValidBefore, Nonce: r.Nonce,
	}.Parse()
	if err != nil {
		return nil, err
	}
	sig, err := hex.DecodeString(strings.TrimPrefix(r.Signature, "0x"))
	if err != nil || len(sig) != 65 {
		return nil, fmt.Errorf("stored signature is not 65 bytes of hex")
	}
	out := mustHex(transferSelector)
	out = append(out, pad(p.From[:])...)
	out = append(out, pad(p.To[:])...)
	out = append(out, word(p.Value)...)
	out = append(out, word(new(big.Int).SetUint64(p.ValidAfter))...)
	out = append(out, word(new(big.Int).SetUint64(p.ValidBefore))...)
	out = append(out, p.Nonce[:]...)
	out = append(out, pad([]byte{sig[64]})...) // v, 27 or 28 (Canonical)
	out = append(out, sig[:32]...)             // r
	out = append(out, sig[32:64]...)           // s
	return out, nil
}

// errNoFee is the relayer out of gas money: nothing the payer did, and nothing
// the rail can send until the relayer is funded.
var errNoFee = errors.New("relayer cannot pay the gas")

// sign drafts the relayer's transferWithAuthorization for r and signs it with
// the relayer key. It returns the transaction's bytes, its hash and the nonce
// and fees it pays, so the record names the transaction before it is
// broadcast. With replace, it is the replacement of r's last send: the same
// nonce, higher fees.
func (s *Service) sign(ctx context.Context, l *link, r *remittance.Remittance, replace *evm.Fees) (raw []byte, hash string, fees evm.Fees, err error) {
	data, err := transferCall(r)
	if err != nil {
		return nil, "", fees, err
	}
	d, fees, err := l.evm.DraftCall(ctx, s.cfg.RelayerAddress(), l.chain.USDC, data, replace)
	var short *custody.ErrNoFee
	if errors.As(err, &short) {
		return nil, "", fees, fmt.Errorf("%w: has %s wei, needs %s", errNoFee, short.Have, short.Need)
	}
	if err != nil {
		return nil, "", fees, err
	}
	if len(d.Digests) != 1 {
		return nil, "", fees, fmt.Errorf("draft wants %d signatures, want 1", len(d.Digests))
	}
	sig, err := crypto.Sign(d.Digests[0], s.cfg.Relayer)
	if err != nil {
		return nil, "", fees, err
	}
	raw, err = d.Seal([][]byte{sig})
	if err != nil {
		return nil, "", fees, err
	}
	return raw, eip3009.Hex(crypto.Keccak256(raw)), fees, nil
}

// use is what the chain says became of a remittance's authorization.
type use struct {
	tx    string
	block uint64
	// paid reports the use carried the remittance itself: the Transfer the USDC
	// contract emits right after this authorization's AuthorizationUsed moved
	// exactly Value from From to the treasury. False means another
	// authorization under the same nonce was used.
	paid bool
	// canceled reports the payer canceled the authorization
	// (cancelAuthorization): it can never pay.
	canceled bool
}

// locate finds what became of r's authorization: the transaction the rail
// sent, when its receipt shows the use; else a search of the authorization's
// window, FromBlock to ToBlock, for AuthorizationUsed or AuthorizationCanceled
// of (From, Nonce), logSpan blocks at a time from where the last search
// stopped (Scanned, kept on the row). Nil with no error means the chain shows
// nothing yet in the blocks searched.
func (s *Service) locate(ctx context.Context, l *link, r *remittance.Remittance) (*use, error) {
	for _, tx := range r.Sends {
		rc, err := l.rpc().Receipt(ctx, tx)
		if err != nil {
			return nil, err
		}
		if u := s.spent(l, r, tx, rc); u != nil {
			return u, nil
		}
	}
	head, err := l.rpc().BlockNumber(ctx)
	if err != nil {
		return nil, err
	}
	end := min(head, uint64(max(r.ToBlock, 0)))
	start := max(r.Scanned+1, r.FromBlock, 0)
	scanned := start - 1 // the last block searched, as the row will record it
	defer func() {
		if scanned > r.Scanned {
			_, _ = update(r.Id(), func(x *remittance.Remittance) bool {
				if scanned <= x.Scanned {
					return false
				}
				x.Scanned = scanned
				return true
			})
			r.Scanned = scanned
		}
	}()
	topics := []any{[]string{usedTopic, canceledTopic}, eip3009.Hex(pad(mustAddr(r.From))), strings.ToLower(r.Nonce)}
	for from := uint64(start); from <= end; from += logSpan {
		to := min(from+logSpan-1, end)
		logs, err := l.rpc().Logs(ctx, l.chain.USDC, topics, from, to)
		if err != nil {
			return nil, err
		}
		for _, lg := range logs {
			if lg.Topics[0] == canceledTopic {
				return &use{tx: lg.TxHash, block: lg.Block, canceled: true}, nil
			}
			rc, err := l.rpc().Receipt(ctx, lg.TxHash)
			if err != nil {
				return nil, err
			}
			if u := s.spent(l, r, lg.TxHash, rc); u != nil {
				return u, nil
			}
		}
		scanned = int64(to)
	}
	return nil, nil
}

// searched reports whether the whole window a use of r could be in has been
// searched: validBefore is past (with grace, for a lagging node) and the
// search has reached the head the chain stood at then.
func (s *Service) searched(r *remittance.Remittance, head uint64) bool {
	return !s.now().Before(before(r).Add(grace)) && uint64(max(r.Scanned, 0)) >= min(head, uint64(max(r.ToBlock, 0)))
}

// spent reads a receipt for the use of r's authorization. Only a SUCCESSFUL
// transaction whose logs, from the USDC contract, include AuthorizationUsed of
// (From, Nonce) is a use. It paid r when the very next log — FiatTokenV2 marks
// the authorization used and then transfers, so its Transfer is the next event
// — is a Transfer from the USDC contract of exactly Value from From to the
// treasury. Any other Transfer in the transaction is some other
// authorization's, however alike: one transfer pays one payment.
func (s *Service) spent(l *link, r *remittance.Remittance, tx string, rc *husdindex.Receipt) *use {
	if rc == nil || !rc.Success {
		return nil
	}
	from := eip3009.Hex(pad(mustAddr(r.From)))
	to := eip3009.Hex(pad(mustAddr(s.cfg.Treasury)))
	value, _ := new(big.Int).SetString(r.Value, 10)
	for i, lg := range rc.Logs {
		if lg.Address != l.chain.USDC || len(lg.Topics) != 3 || lg.Topics[0] != usedTopic ||
			lg.Topics[1] != from || lg.Topics[2] != strings.ToLower(r.Nonce) {
			continue
		}
		paid := false
		if i+1 < len(rc.Logs) {
			t := rc.Logs[i+1]
			if t.Address == l.chain.USDC && t.Index == lg.Index+1 && len(t.Topics) == 3 &&
				t.Topics[0] == husdindex.TransferTopic0 && t.Topics[1] == from && t.Topics[2] == to {
				got, err := hexWord(t.Data)
				paid = err == nil && value != nil && got.Cmp(value) == 0
			}
		}
		return &use{tx: strings.ToLower(tx), block: rc.Block, paid: paid}
	}
	return nil
}

// revert names what a simulated or mined transferWithAuthorization was refused
// for, from FiatTokenV2's own revert strings.
func revert(err error) (reason string, final bool) {
	m := strings.ToLower(err.Error())
	switch {
	case strings.Contains(m, "authorization is used"):
		return "used", false
	case strings.Contains(m, "exceeds balance"):
		return ReasonBalance, false
	case strings.Contains(m, "invalid signature"):
		return ReasonSignature, true
	case strings.Contains(m, "authorization is expired"):
		return ReasonExpired, true
	case strings.Contains(m, "not yet valid"):
		return ReasonEarly, false
	case strings.Contains(m, "blacklisted"):
		return ReasonBlocked, false
	case strings.Contains(m, "paused"):
		return "paused", false
	case strings.Contains(m, "revert"):
		return ReasonRejected, false
	}
	return "", false // not a revert: the node did not answer
}

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

// mustAddr reads an address the rail stored itself, already parsed once.
func mustAddr(s string) []byte {
	a, err := eip3009.Address(s)
	if err != nil {
		panic(fmt.Sprintf("gateway: stored address %q: %v", s, err))
	}
	return a[:]
}

func pad(b []byte) []byte {
	out := make([]byte, 32)
	copy(out[32-len(b):], b)
	return out
}

func word(v *big.Int) []byte {
	out := make([]byte, 32)
	v.FillBytes(out)
	return out
}

func hexWord(s string) (*big.Int, error) {
	b, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	if err != nil || len(b) != 32 {
		return nil, fmt.Errorf("not one word: %q", s)
	}
	return new(big.Int).SetBytes(b), nil
}
