// Package evm sweeps an EVM custody address: native coin or ERC-20, drafted
// against the chain, signed by the MPC fleet, broadcast by us.
//
// It is the only place in commerce that links luxfi/geth, and it links it for
// one reason: transaction encoding. An EIP-1559 transaction is
// keccak256(0x02 ‖ rlp([chainId, nonce, tip, cap, gas, to, value, data,
// accessList])) and it is perfectly possible to write that by hand — right up
// until an empty access list, a zero value or a leading-zero-byte amount
// encodes one byte differently and the signature covers a payload no node will
// accept. types.DynamicFeeTx is the reference implementation of that encoding
// in Go, it is the fork this estate mandates, and it was already in the module
// graph. The tests below prove the choice byte for byte against a signature
// this code did not produce.
package evm

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/luxfi/crypto"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/types"

	"github.com/hanzoai/commerce/billing/custody"
	"github.com/hanzoai/commerce/billing/husdindex"
)

// transferSelector is the first four bytes of keccak256("transfer(address,uint256)").
//
// It is written as a constant and PROVEN in the tests by hashing the signature
// string, rather than hashed at init: a selector is four bytes that decide
// which function of a contract runs, and one wrong nibble calls something else
// — on many tokens, something that succeeds and moves nothing.
const transferSelector = "a9059cbb"

// gasMargin is how much headroom is added to the node's gas estimate, in
// percent.
//
// An estimate is made against the CURRENT state, and the transaction executes
// against a later one. For a plain transfer the gap is usually nothing, but a
// token whose first transfer to a recipient writes a fresh storage slot, or one
// that updates a fee accumulator, can cost more by the time it lands. Too low
// does not fail cheaply: the transaction is mined, reverts, and the gas is
// spent regardless.
const gasMargin = 25

// tipCeiling and feeMultiplier shape the fee cap.
//
// maxFeePerGas is set to baseFee*feeMultiplier + tip so the transaction
// survives a few blocks of rising base fee — EIP-1559 lets it climb 12.5% per
// block, so a cap at exactly the current base fee is a transaction that stops
// being mineable almost immediately. The unspent difference is refunded, so the
// multiplier costs nothing when fees are calm.
const feeMultiplier = 2

// Chain sweeps one EVM network.
type Chain struct {
	network custody.Network
	rpc     *husdindex.Client
	chainID *big.Int
}

// New builds a sweeper for network at rpcURL.
//
// The chain id is READ from the node rather than configured, and that is a
// safety property rather than a convenience: the chain id is the only thing
// that stops a signature made for Base from also being valid on Ethereum, and a
// wrong constant would produce a transaction that is replayable wherever the
// same address holds funds. A node cannot be wrong about its own id.
func New(ctx context.Context, network custody.Network, rpcURL string) (*Chain, error) {
	if c, ok := network.Curve(); !ok || c != custody.Secp256k1 {
		return nil, fmt.Errorf("evm: %q is not an EVM network on this rail", network)
	}
	// tokenAddr is empty: this client is used for chain-level calls, and each
	// token's balance is read through a client bound to that token.
	rpc := husdindex.NewClient(rpcURL, "")
	id, err := rpc.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("evm: %s: reading chain id: %w", network, err)
	}
	if id.Sign() <= 0 {
		return nil, fmt.Errorf("evm: %s reported chain id %s", network, id)
	}
	return &Chain{network: network, rpc: rpc, chainID: id}, nil
}

// Network is the code the signer resolves a curve from.
func (c *Chain) Network() custody.Network { return c.network }

// ChainID is the id read from the node at construction.
func (c *Chain) ChainID() *big.Int { return new(big.Int).Set(c.chainID) }

// Draft reads the chain and returns the unsigned transfer with the one digest
// that must be signed.
func (c *Chain) Draft(ctx context.Context, t custody.Transfer) (*custody.Draft, error) {
	from, err := parseAddr(t.From)
	if err != nil {
		return nil, fmt.Errorf("evm: from: %w", err)
	}
	to, err := parseAddr(t.To)
	if err != nil {
		return nil, fmt.Errorf("evm: to: %w", err)
	}

	// What the transaction actually carries: for a token, a call to the token
	// contract with the recipient in the calldata; for the native coin, value
	// straight to the recipient.
	var (
		target common.Address
		value  *big.Int
		data   []byte
	)
	if t.Token != "" {
		token, err := parseAddr(t.Token)
		if err != nil {
			return nil, fmt.Errorf("evm: token: %w", err)
		}
		amount := t.Amount
		if amount == nil {
			// Sweep everything: whatever the contract says we hold.
			amount, err = c.rpc.BalanceOfToken(ctx, t.Token, t.From)
			if err != nil {
				return nil, fmt.Errorf("evm: reading %s balance of %s: %w", t.Token, t.From, err)
			}
		}
		if amount.Sign() <= 0 {
			return nil, custody.ErrEmpty
		}
		target, value, data = token, new(big.Int), transferCall(to, amount)
	} else {
		target, data = to, nil
		value = t.Amount // may be nil; settled below, once the fee is known
	}

	nonce, err := c.rpc.Nonce(ctx, t.From)
	if err != nil {
		return nil, fmt.Errorf("evm: reading nonce of %s: %w", t.From, err)
	}
	tip, err := c.rpc.Tip(ctx)
	if err != nil {
		return nil, fmt.Errorf("evm: reading priority fee: %w", err)
	}
	base, err := c.rpc.BaseFee(ctx)
	if err != nil {
		return nil, fmt.Errorf("evm: reading base fee: %w", err)
	}
	feeCap := new(big.Int).Add(new(big.Int).Mul(base, big.NewInt(feeMultiplier)), tip)

	gas, err := c.rpc.EstimateGas(ctx, t.From, target.Hex(), value, data)
	if err != nil {
		return nil, fmt.Errorf("evm: estimating gas: %w", err)
	}
	gas = gas * (100 + gasMargin) / 100

	// What the whole thing can cost at the cap. This is what must be on hand in
	// the native coin, and it is the number a native sweep has to leave behind.
	maxCost := new(big.Int).Mul(new(big.Int).SetUint64(gas), feeCap)

	held, err := c.rpc.NativeBalance(ctx, t.From)
	if err != nil {
		return nil, fmt.Errorf("evm: reading native balance of %s: %w", t.From, err)
	}

	if t.Token != "" {
		// A token transfer pays gas out of a coin it does not carry. This is the
		// ordinary state of a deposit address the first time it is swept — a
		// payer who sends USDC sends no ETH with it — so it gets a typed error
		// naming exactly what to send, rather than a revert.
		if held.Cmp(maxCost) < 0 {
			return nil, &custody.ErrNoFee{Network: c.network, Addr: t.From, Have: held, Need: maxCost}
		}
	} else {
		// A native sweep pays for itself, so "everything" means everything minus
		// the fee. Computing it against the CAP rather than the expected cost is
		// deliberate: the transaction must remain fundable at any base fee it
		// could legally meet, and the difference is refunded when it lands.
		if value == nil {
			value = new(big.Int).Sub(held, maxCost)
			if value.Sign() <= 0 {
				return nil, &custody.ErrNoFee{Network: c.network, Addr: t.From, Have: held, Need: maxCost}
			}
		} else if total := new(big.Int).Add(value, maxCost); held.Cmp(total) < 0 {
			return nil, &custody.ErrNoFee{Network: c.network, Addr: t.From, Have: held, Need: total}
		}
	}

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   c.chainID,
		Nonce:     nonce,
		GasTipCap: tip,
		GasFeeCap: feeCap,
		Gas:       gas,
		To:        &target,
		Value:     value,
		Data:      data,
	})
	return c.draft(tx, from)
}

// draft turns a built transaction into the Draft that Sweep drives, and it is
// separate from Draft so the tests can reach it with a transaction of their own
// and check the encoding against an outside implementation without a live node.
func (c *Chain) draft(tx *types.Transaction, from common.Address) (*custody.Draft, error) {
	signer := types.LatestSignerForChainID(c.chainID)
	digest := signer.Hash(tx)

	return &custody.Draft{
		Digests: [][]byte{digest.Bytes()},
		Seal: func(sigs [][]byte) ([]byte, error) {
			if len(sigs) != 1 {
				return nil, fmt.Errorf("evm: expected 1 signature, got %d", len(sigs))
			}
			sig, err := recoverable(digest.Bytes(), sigs[0], from)
			if err != nil {
				return nil, err
			}
			signed, err := tx.WithSignature(signer, sig)
			if err != nil {
				return nil, fmt.Errorf("evm: attaching signature: %w", err)
			}
			// Belt and braces, and cheap: ask the encoding itself who sent this,
			// through the same path a node will. The recovery above proves the
			// signature matches the key; this proves the assembled TRANSACTION
			// does, which is what the chain will check.
			sender, err := types.Sender(signer, signed)
			if err != nil {
				return nil, fmt.Errorf("evm: assembled transaction has no recoverable sender: %w", err)
			}
			if sender != from {
				return nil, fmt.Errorf("evm: assembled transaction is from %s, not the custody address %s", sender.Hex(), from.Hex())
			}
			return signed.MarshalBinary()
		},
	}, nil
}

// Broadcast submits the signed bytes.
func (c *Chain) Broadcast(ctx context.Context, raw []byte) (string, error) {
	return c.rpc.Send(ctx, raw)
}

// recoverable turns the signer's answer into the 65-byte r‖s‖v that geth wants,
// and REFUSES anything that does not belong to from.
//
// This is the last check before bytes become a transaction, and it is the one
// that catches the whole family of custody mistakes at once: a wrong wallet id,
// a wallet whose key was reshared, a signer that answered on the wrong curve, a
// digest that drifted between drafting and signing. All of them produce a
// signature that recovers to some other address, and none of them is
// distinguishable from success by looking at the signature alone.
//
// mpcd already returns v, and its own recovery self-check refuses to emit a
// signature whose v does not match the wallet's public key. That byte is
// nonetheless RE-DERIVED here rather than trusted, because trusting it would
// make this check circular: it would confirm the signer agrees with itself,
// when the question is whether the signer agrees with the address commerce
// recorded and told a customer to pay into.
func recoverable(digest, sig []byte, from common.Address) ([]byte, error) {
	if len(sig) != 64 && len(sig) != 65 {
		return nil, fmt.Errorf("evm: signature should be 64 or 65 bytes, got %d", len(sig))
	}
	out := make([]byte, 65)
	copy(out, sig[:64])
	for _, v := range []byte{0, 1} {
		out[64] = v
		// Ecrecover returns the uncompressed public key with its 0x04 prefix;
		// the address is the low 20 bytes of keccak256 over the 64 bytes after
		// it. This is byte for byte what luxfi/geth's own recoverPlain does, so
		// this check and the node's check cannot disagree about what "from"
		// means.
		pub, err := crypto.Ecrecover(digest, out)
		if err != nil || len(pub) != 65 {
			continue
		}
		var addr common.Address
		copy(addr[:], crypto.Keccak256(pub[1:])[12:])
		if addr == from {
			return out, nil
		}
	}
	return nil, fmt.Errorf("evm: signature over %s does not belong to custody address %s — "+
		"neither recovery id yields it, so this signature is for some other key and must not be broadcast",
		"0x"+hex.EncodeToString(digest), from.Hex())
}

// transferCall is the calldata for ERC-20 transfer(to, amount): the selector
// followed by two 32-byte words.
func transferCall(to common.Address, amount *big.Int) []byte {
	sel, _ := hex.DecodeString(transferSelector)
	out := make([]byte, 0, 4+64)
	out = append(out, sel...)
	out = append(out, common.LeftPadBytes(to.Bytes(), 32)...)
	out = append(out, common.LeftPadBytes(amount.Bytes(), 32)...)
	return out
}

// parseAddr accepts a 0x address and refuses everything else.
//
// geth's common.HexToAddress does NOT refuse: it silently truncates or
// zero-pads, so a typo one character short becomes a different, valid-looking
// address, and an empty string becomes the zero address — which on an EVM chain
// is a burn. A destination is where money goes; it is parsed strictly or not at
// all.
func parseAddr(s string) (common.Address, error) {
	h := strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if len(h) != 40 {
		return common.Address{}, fmt.Errorf("%q is not a 20-byte address", s)
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return common.Address{}, fmt.Errorf("%q is not hex: %w", s, err)
	}
	var a common.Address
	copy(a[:], b)
	if a == (common.Address{}) {
		return common.Address{}, fmt.Errorf("refusing the zero address")
	}
	return a, nil
}

var _ custody.Chain = (*Chain)(nil)
