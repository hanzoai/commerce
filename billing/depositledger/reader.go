package depositledger

import (
	"context"
	"fmt"

	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/billing/husdindex"
	"github.com/hanzoai/commerce/billing/solanarpc"
)

// newReader builds the chain read client for one asset.
//
// This is the ONE place a chain family turns into a concrete client, and it is
// exhaustive rather than defaulted: depositwatch.AssetsFromEnv has already
// refused any chain that is not in its family table, so an asset arriving here
// is one of these two by construction. Adding a chain means adding a Reader and
// a family entry — and nothing in the policy half moves.
func newReader(a depositwatch.Asset) (depositwatch.Reader, error) {
	if a.IsSolana() {
		mint, err := solanarpc.ParsePublicKey(a.Contract)
		if err != nil {
			return nil, fmt.Errorf("depositledger: %s: %w", a.Key(), err)
		}
		return solanarpc.NewClient(a.RPCURL, mint), nil
	}
	// husdindex.Client is this repo's one ERC-20 JSON-RPC read client; it is
	// parameterized by (rpcURL, tokenAddr) and is named for its first caller,
	// not for a restriction.
	return evmReader{husdindex.NewClient(a.RPCURL, a.Contract)}, nil
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
