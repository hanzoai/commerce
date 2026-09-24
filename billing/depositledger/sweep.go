package depositledger

import (
	"context"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"

	"github.com/hanzoai/commerce/billing/custody"
	"github.com/hanzoai/commerce/billing/custody/bitcoin"
	"github.com/hanzoai/commerce/billing/custody/evm"
	"github.com/hanzoai/commerce/billing/custody/solana"
	"github.com/hanzoai/commerce/billing/depositwatch"
)

// The other direction. This file is to money leaving what the rest of the
// package is to money arriving, and it reuses every piece: the same configured
// assets, the same enumeration of minted addresses, the same chain→family table.
// A second copy of any of those would be a second answer to "where is the money"
// that could disagree with the first.
//
// It is called from ONE place — `commerce sweep` — and deliberately from no
// schedule. Every EVM token sweep begins by needing gas at an address that holds
// none (custody.ErrNoFee is the ordinary first state, not an exotic one), so a
// timer would spend its life failing; and the destination is a treasury address,
// which is not a thing to infer from configuration when a person can state it.

// Chain builds the WRITE client for one asset — the mirror of newReader, and
// exhaustive for the same reason: a chain that reached a default here would be
// handed another chain's transaction encoder, which is how money goes somewhere
// unrecoverable rather than merely failing.
//
// Bitcoin is built on mainnet parameters. That is a real assumption and it is
// safe rather than silent: the chain validates every address it is given against
// those parameters, so a testnet address refuses instead of being spent from.
func Chain(ctx context.Context, a depositwatch.Asset) (custody.Chain, error) {
	switch a.Family() {
	case depositwatch.FamilyEVM:
		return evm.New(ctx, custody.Network(a.Chain), a.RPCURL)
	case depositwatch.FamilyBitcoin:
		return bitcoin.New(a.RPCURL, &chaincfg.MainNetParams), nil
	case depositwatch.FamilySolana:
		return solana.New(a.RPCURL), nil
	default:
		// TON and XRPL are readable and creditable; neither has an encoder in
		// billing/custody yet, so neither can be swept from here. Saying which
		// is missing beats a generic refusal, because the fix is one package.
		return nil, fmt.Errorf("depositledger: %s: no custody chain writes %s — deposits there can be credited but not yet swept", a.Key(), a.Chain)
	}
}

// Swept is what one custody address produced. Err is per-address on purpose: one
// payer's address that cannot pay its own gas must not stop the other five
// hundred from being collected.
type Swept struct {
	Org     string
	Address string
	TxID    string
	Err     error
}

// Sweep empties every custody address holding this asset into `to`.
//
// The amount is never named. Each chain computes "everything spendable" for
// itself — a token balance in full, a native balance less the fee it takes to
// move it — because that arithmetic is chain-specific and an operator's guess at
// it either leaves dust behind or builds a transaction that cannot pay for
// itself.
//
// An address with nothing in it is not a failure and produces no row: a sweep
// walks every address ever minted, and most of them are empty most of the time.
func Sweep(ctx context.Context, c custody.Chain, s custody.Signer, a depositwatch.Asset, to string) ([]Swept, error) {
	if to == "" {
		return nil, errors.New("depositledger: sweep needs a destination")
	}
	if a.PooledAddress != "" {
		// One account holds every payer's deposit, so there is nothing per-payer
		// to walk and the enumeration below would attempt the same address once
		// per intent — building N conflicting spends of one balance.
		return nil, fmt.Errorf("depositledger: %s is pooled into %s; sweep that account directly, not per payer", a.Key(), a.PooledAddress)
	}
	minted, err := intentStore{}.Watched(ctx, a.Chain, a.Token)
	if err != nil {
		return nil, err
	}

	var out []Swept
	seen := map[string]bool{}
	for _, w := range minted {
		// One address, one attempt. Two intents naming one address would
		// otherwise become two spends of the same coins, drafted against
		// identical chain state and therefore both valid to sign — the second
		// one only losing once the first is mined.
		if seen[w.Org+"\x00"+w.Address] {
			continue
		}
		seen[w.Org+"\x00"+w.Address] = true

		if w.Wallet == "" {
			// The keygen's wallet_id was dropped on the floor for a while, and
			// an address without one cannot be signed for by anything here. It
			// is reported rather than skipped: the money is real and recovering
			// it means reconciling against the signer's own records.
			out = append(out, Swept{Org: w.Org, Address: w.Address,
				Err: errors.New("no wallet id recorded at mint; the signer cannot be asked for this key")})
			continue
		}

		id, err := custody.Sweep(ctx, c, s, custody.Transfer{
			OrgID:    w.Org,
			WalletID: w.Wallet,
			From:     w.Address,
			To:       to,
			Token:    a.Contract, // empty for a native coin, which is what it means
		})
		if errors.Is(err, custody.ErrEmpty) {
			continue
		}
		out = append(out, Swept{Org: w.Org, Address: w.Address, TxID: id, Err: err})
	}
	return out, nil
}
