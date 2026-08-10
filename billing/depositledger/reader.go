package depositledger

import (
	"context"
	"fmt"

	"github.com/hanzoai/commerce/billing/bitcoinrpc"
	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/billing/husdindex"
	"github.com/hanzoai/commerce/billing/solanarpc"
	"github.com/hanzoai/commerce/billing/tonrpc"
	"github.com/hanzoai/commerce/billing/xrplrpc"
)

// newReader builds the chain read client for one asset.
//
// This is the ONE place a chain family turns into a concrete client, and it
// switches on the family exhaustively rather than defaulting: an unhandled
// family must be a compile-time-visible gap here, not an asset quietly handed
// an EVM reader because everything looks like an EVM. That assumption is how a
// Solana endpoint gets asked for eth_getLogs and watches nothing while looking
// configured.
//
// Adding a chain means adding a Reader and a family entry — and nothing in the
// policy half moves.
func newReader(a depositwatch.Asset) (depositwatch.Reader, error) {
	// A coin is read by a different client from the tokens on its chain. Asked
	// BEFORE the family switch because the family cannot answer it: "ton" names
	// both the coin and the chain carrying jetton USDT.
	if a.MarketPriced() {
		switch a.Family() {
		case depositwatch.FamilyBitcoin:
			return bitcoinrpc.NewClient(a.RPCURL), nil
		case depositwatch.FamilyTON:
			return tonrpc.NewNative(a.RPCURL), nil
		case depositwatch.FamilyXRPL:
			return xrplrpc.NewNative(a.RPCURL), nil
		default:
			return nil, fmt.Errorf("depositledger: %s: %q is market-priced but its chain has no native reader", a.Key(), a.Token)
		}
	}

	switch a.Family() {
	case depositwatch.FamilySolana:
		mint, err := solanarpc.ParsePublicKey(a.Contract)
		if err != nil {
			return nil, fmt.Errorf("depositledger: %s: %w", a.Key(), err)
		}
		return solanarpc.NewClient(a.RPCURL, mint), nil

	case depositwatch.FamilyTON:
		master, err := tonrpc.ParseAddress(a.Contract)
		if err != nil {
			return nil, fmt.Errorf("depositledger: %s: %w", a.Key(), err)
		}
		return tonrpc.NewClient(a.RPCURL, master), nil

	case depositwatch.FamilyXRPL:
		// The whole token is (currency, issuer): neither half names a token by
		// itself, so both are parsed here and the issuer is checksummed.
		token, err := xrplrpc.ParseIssued(a.Contract)
		if err != nil {
			return nil, fmt.Errorf("depositledger: %s: %w", a.Key(), err)
		}
		return xrplrpc.NewClient(a.RPCURL, token), nil

	case depositwatch.FamilyEVM:
		// husdindex.Client is this repo's one ERC-20 JSON-RPC read client; it is
		// parameterized by (rpcURL, tokenAddr) and is named for its first caller,
		// not for a restriction.
		return evmReader{husdindex.NewClient(a.RPCURL, a.Contract)}, nil

	default:
		return nil, fmt.Errorf("depositledger: %s: chain %q has no reader", a.Key(), a.Chain)
	}
}

// evmReader adapts the ERC-20 read client to the watcher's chain-agnostic event.
//
// The adapter exists in one direction only, and that asymmetry is the point:
// husdindex.Transfer is an ERC-20 LOG — it predates this rail and still serves
// the HUSD indexer — while depositwatch.Transfer is a value movement on any
// chain. Translating at this seam is what lets the EVM keep calling its event
// index a log index while the ledger keys on something a Solana signature can
// also answer.
type evmReader struct{ *husdindex.Client }

var _ depositwatch.Reader = evmReader{}

func (r evmReader) TransfersTo(ctx context.Context, addrs []string, fromBlock, toBlock uint64) ([]depositwatch.Transfer, error) {
	logs, err := r.Client.TransfersTo(ctx, addrs, fromBlock, toBlock)
	if err != nil {
		return nil, err
	}
	out := make([]depositwatch.Transfer, 0, len(logs))
	for _, l := range logs {
		out = append(out, depositwatch.Transfer{
			To:     l.To,
			Units:  l.ValueWei,
			TxHash: l.TxHash,
			// An ERC-20 log index is scoped to the BLOCK, not to the
			// transaction, which is a strictly stronger identifier than the
			// dedup key needs: a log belongs to exactly one transaction, so
			// (txHash, logIndex) names the event either way.
			EventIndex: l.LogIndex,
			Block:      l.Block,
		})
	}
	return out, nil
}
