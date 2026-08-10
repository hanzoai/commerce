package tonrpc

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/hanzoai/commerce/billing/depositwatch"
)

// Native reads native TON — the coin, not a jetton.
//
// Separate from Client because little is shared: a jetton lands in a wallet
// derived from owner+master and is credited from a decoded body, a coin lands in
// the owner account and is the message's own value. Transport and paging are
// shared via walk.
//
// It needs no token identity, which is why it works where jetton USDT does not:
// that master publishes decimals and a URI but no symbol.
type Native struct {
	c *Client
}

// NewNative builds a reader for native TON. The master is unused — no jetton
// wallet is derived and no master content is read.
func NewNative(baseURL string) *Native {
	return &Native{c: NewClient(baseURL, Address{})}
}

// nativeDecimals is 9 by protocol. A coin has no contract to read it from.
const nativeDecimals = 9

// BlockNumber is the masterchain head.
func (n *Native) BlockNumber(ctx context.Context) (uint64, error) { return n.c.BlockNumber(ctx) }

// Decimals is 9. See nativeDecimals.
func (n *Native) Decimals(context.Context) (int, error) { return nativeDecimals, nil }

// Symbol is TON. Nothing on chain could disagree.
func (n *Native) Symbol(context.Context) (string, error) { return "TON", nil }

// TransfersTo returns the native TON that landed in each watched account. The
// accounts ARE the deposit addresses, so an address is watchable the moment it
// is minted.
func (n *Native) TransfersTo(ctx context.Context, owners []string, fromSeqno, toSeqno uint64) ([]depositwatch.Transfer, error) {
	if len(owners) == 0 || toSeqno < fromSeqno {
		return nil, nil
	}
	// A TON account has several spellings, so two watched addresses can be one
	// account. Refused: "which intent owns this?" has no safe answer.
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

// nativeReceipt turns one transaction into the credit it produced, or none.
//
// THE DISCRIMINATOR IS THE OPCODE. Every inbound message carries a value,
// including the gas on a contract call — a jetton notification lands on this
// same account with TON attached to pay for it, and crediting that would make
// every jetton transfer a phantom deposit. A deposit has NO operation: an empty
// opcode, or the zero opcode a plain transfer with a comment carries.
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
		// A shape this code does not understand. Guessing zero would drop a real
		// deposit.
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
		// in_msg is a single field, so one transaction is exactly one credit and
		// the hash alone names it. 0 means "there is only one".
		EventIndex: 0,
		Block:      tx.McBlockSeqno,
	}, true, nil
}

// isZeroOpcode reports "no operation". A plain transfer with a text comment uses
// 0x00000000 and IS a deposit — so the digits are read, not the string, since
// "0x0" and "0" mean the same thing.
func isZeroOpcode(op string) bool {
	s := strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(op), "0x"), "0X")
	if s == "" {
		return true
	}
	return strings.Trim(s, "0") == ""
}
