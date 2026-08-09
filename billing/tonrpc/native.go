package tonrpc

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/hanzoai/commerce/billing/depositwatch"
)

// Native reads NATIVE TON — the coin itself, not a jetton.
//
// It is a separate type from Client rather than a mode on it, because almost
// nothing is shared once you look: a jetton deposit lands in a JETTON WALLET
// derived from the owner and the master, and is credited by a decoded
// internal_transfer body; a native deposit lands in the OWNER ACCOUNT and is the
// message's own attached value. Only the HTTP transport and the paging cursor
// are common, and those are shared directly (walk).
//
// WHY THIS IS THE EASIEST NATIVE COIN ON THE RAIL: it needs no token identity at
// all. The jetton path has to prove a master contract IS the USDT everyone means,
// which TON's USDT makes impossible on chain — it publishes decimals and a URI
// and no symbol — and that is what keeps jetton USDT unarmed. A native coin is
// the chain's own unit; there is nothing to identify, only something to price.
type Native struct {
	c *Client
}

// NewNative builds a reader for native TON against a TON Index endpoint.
func NewNative(baseURL string) *Native {
	// The master is the zero Address and is never used: no jetton wallet is
	// derived, no master content is read. Passing one would be inviting a later
	// reader to think it mattered.
	return &Native{c: NewClient(baseURL, Address{})}
}

// nativeDecimals is TON's, by protocol: 1 TON is 10^9 nanotons.
//
// Everywhere else on this rail decimals come from the token contract, "never
// from config", because a decimals constant is a number no code can check and a
// wrong one credits 10^12 times too much. A native coin has NO contract to ask —
// the unit is the chain's own, fixed by consensus and not by a deployment — so a
// constant is the only truthful answer here rather than a shortcut.
const nativeDecimals = 9

// BlockNumber is the masterchain head, the same position the jetton reader uses.
func (n *Native) BlockNumber(ctx context.Context) (uint64, error) { return n.c.BlockNumber(ctx) }

// Decimals is 9, by protocol. See nativeDecimals.
func (n *Native) Decimals(context.Context) (int, error) { return nativeDecimals, nil }

// Symbol is TON. There is no contract to ask and nothing that could disagree.
func (n *Native) Symbol(context.Context) (string, error) { return "TON", nil }

// TransfersTo returns the native TON that landed in each watched account.
//
// The accounts are the deposit addresses themselves — no jetton wallet is
// derived — which is why this can watch an address the moment it is minted,
// before anything has ever been sent to it.
func (n *Native) TransfersTo(ctx context.Context, owners []string, fromSeqno, toSeqno uint64) ([]depositwatch.Transfer, error) {
	if len(owners) == 0 || toSeqno < fromSeqno {
		return nil, nil
	}
	// A TON account has several correct spellings, so two watched addresses can
	// be the SAME account written differently. Refused rather than resolved:
	// "which of these two intents owns this account?" has no safe answer.
	byAccount := make(map[Address]string, len(owners))
	for _, o := range owners {
		addr, err := ParseAddress(o)
		if err != nil {
			return nil, fmt.Errorf("tonrpc: watched deposit address %q is not a TON address: %w", o, err)
		}
		if prev, dup := byAccount[addr]; dup {
			return nil, fmt.Errorf("tonrpc: watched addresses %q and %q are the same account (%s) written two ways", prev, o, addr)
		}
		byAccount[addr] = o
	}

	var out []depositwatch.Transfer
	for account, owner := range byAccount {
		got, err := n.c.walk(ctx, account, fromSeqno, toSeqno, func(tx *transaction) (depositwatch.Transfer, bool, error) {
			return nativeReceipt(tx, account, owner)
		})
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return out, nil
}

// nativeReceipt turns ONE transaction on a watched account into the native TON
// credit it produced, or reports that it produced none.
//
// THE DISCRIMINATOR IS THE OPCODE, and getting it wrong credits gas as a deposit.
// Every inbound message carries a `value`, including the gas attached to a
// contract call: a jetton transfer notification lands on this same account with
// a small amount of TON attached to pay for it. Crediting that would turn every
// jetton transfer into a small phantom TON deposit. So a native deposit is a
// message with NO operation — an empty opcode, or the explicit zero opcode a
// plain transfer with a text comment carries — and anything else is a contract
// operation whose value is fuel rather than payment.
//
// The success checks mirror the jetton path for the same reason they exist
// there: a transaction that aborted, whose compute phase failed or was skipped,
// or whose action phase failed, moved nothing. On the native path the value is
// on the message rather than in a validated body, so these checks are what
// carry the chain's own verdict into this process.
func nativeReceipt(tx *transaction, account Address, owner string) (depositwatch.Transfer, bool, error) {
	var zero depositwatch.Transfer
	if tx.InMsg == nil {
		return zero, false, nil // an outbound-only transaction credits nothing
	}
	if tx.InMsg.Bounced {
		return zero, false, nil // a bounce of something we sent, not a deposit
	}
	if op := strings.TrimSpace(tx.InMsg.Opcode); op != "" && !isZeroOpcode(op) {
		return zero, false, nil // a contract operation; its value is gas
	}
	// An inbound message with no source is an external message — a wallet being
	// driven by its owner's key, not value arriving from another account.
	if strings.TrimSpace(tx.InMsg.Source) == "" {
		return zero, false, nil
	}
	if got, err := ParseAddress(tx.InMsg.Destination); err != nil || got != account {
		return zero, false, fmt.Errorf("tonrpc: transaction on %s carries an inbound message addressed to %q", account, tx.InMsg.Destination)
	}
	d := tx.Description
	if d.Aborted ||
		d.ComputePh == nil || d.ComputePh.Skipped || !d.ComputePh.Success || d.ComputePh.ExitCode != 0 ||
		(d.Action != nil && (!d.Action.Success || d.Action.ResultCode != 0)) {
		return zero, false, nil // failed on chain; nothing moved
	}
	raw := strings.TrimSpace(tx.InMsg.Value)
	if raw == "" {
		// A plain transfer always carries a value. Its absence means the index
		// answered a shape this code does not understand, and guessing zero
		// would silently drop a real deposit.
		return zero, false, fmt.Errorf("tonrpc: transaction on %s carries a plain inbound message with no value", account)
	}
	units, ok := new(big.Int).SetString(raw, 10)
	if !ok || units.Sign() < 0 {
		return zero, false, fmt.Errorf("tonrpc: transaction on %s carries value %q, which is not an amount in nanotons", account, raw)
	}
	if units.Sign() == 0 {
		return zero, false, nil
	}
	hash, err := decodeHash(tx.Hash)
	if err != nil {
		return zero, false, fmt.Errorf("tonrpc: transaction on %s: %w", account, err)
	}
	return depositwatch.Transfer{
		To:     owner,
		Units:  units,
		TxHash: hash,
		// A TON transaction has AT MOST ONE inbound message — `in_msg` is a
		// single field, not a list — so one transaction on an account is exactly
		// one credit to it, and the hash alone names this event. 0 is the honest
		// "there is only one" rather than a placeholder.
		EventIndex: 0,
		Block:      tx.McBlockSeqno,
	}, true, nil
}

// isZeroOpcode reports whether an opcode string means "no operation".
//
// The index writes it as hex, and a plain transfer carrying a text comment uses
// the explicit zero opcode 0x00000000 — which is a DEPOSIT, not a contract call,
// and is exactly how most people send TON with a memo. Comparing the string to
// "0x00000000" alone would miss "0x0" and "0", so the digits are what is read.
func isZeroOpcode(op string) bool {
	s := strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(op), "0x"), "0X")
	if s == "" {
		return true
	}
	return strings.Trim(s, "0") == ""
}
