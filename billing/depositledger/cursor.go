// Package depositledger wires the crypto deposit watcher (billing/depositwatch)
// to commerce's production stores and runs it on a schedule.
//
// Decomplected from depositwatch on purpose, exactly as husdledger is from
// husdindex: every money DECISION (how deep is deep enough, what an amount is
// worth, which address belongs to whom) lives in the pure package and is proven
// there against fakes; this package only does I/O — read the intents, write the
// ledger row, persist the cursor, tick the clock.
package depositledger

import (
	"context"
	"errors"

	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/depositcursor"
	"github.com/hanzoai/commerce/util/nscontext"
)

// systemNamespace holds cross-org scanner bookkeeping — one place, not per
// tenant. Same namespace the HUSD indexer's cursor lives in.
const systemNamespace = "system"

// cursorStore is the production depositwatch.Cursor: the last committed block
// per asset, persisted as a singleton record in the system namespace.
type cursorStore struct{}

var _ depositwatch.Cursor = cursorStore{}

func systemDB() *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(context.Background(), systemNamespace))
}

// cursorID is deterministic per asset so the record is a singleton and two
// replicas write the same row rather than racing two of them.
func cursorID(assetKey string) string { return "deposit-cursor:" + assetKey }

func (cursorStore) Last(_ context.Context, assetKey string) (uint64, error) {
	db := systemDB()
	c := depositcursor.New(db)
	if err := c.Get(db.NewKey(c.Kind(), cursorID(assetKey), 0, nil)); err != nil {
		if errors.Is(err, datastore.ErrNoSuchEntity) {
			return 0, nil // never scanned: the watcher cold-starts
		}
		// Any OTHER error is unreadable, not empty. Reporting 0 here would tell
		// the watcher to cold-start and silently skip every block since the last
		// real cursor — so it propagates and the pass refuses instead.
		return 0, err
	}
	return c.LastBlock, nil
}

func (cursorStore) Save(_ context.Context, assetKey string, block uint64) error {
	chain, token := splitAssetKey(assetKey)
	db := systemDB()
	c := depositcursor.New(db)
	c.SetId(cursorID(assetKey))
	c.Chain = chain
	c.Token = token
	c.LastBlock = block
	return c.Put()
}

// splitAssetKey splits "chain:token" back into its parts (for the record's own
// readable fields; the id is the authority).
func splitAssetKey(k string) (chain, token string) {
	for i := 0; i < len(k); i++ {
		if k[i] == ':' {
			return k[:i], k[i+1:]
		}
	}
	return k, ""
}
