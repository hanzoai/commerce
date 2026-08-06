package depositwatch

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/hanzoai/commerce/billing/husdindex"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
)

// Reader reads ONE asset's chain state. *husdindex.Client satisfies it; the
// tests inject a fake so every crediting decision below is proven without a
// chain.
type Reader interface {
	BlockNumber(ctx context.Context) (uint64, error)
	TransfersTo(ctx context.Context, addrs []string, fromBlock, toBlock uint64) ([]husdindex.Transfer, error)
	Decimals(ctx context.Context) (int, error)
	Symbol(ctx context.Context) (string, error)
}

// Watched is one minted deposit address the watcher must observe, together with
// everything needed to credit it: WHERE the ledger row goes (Org, Test) and WHO
// it credits (Subject — the intent's CustomerRef, which is the same key
// GET /v1/billing/me/balance and the gateway's prepaid gate read).
type Watched struct {
	Org      string
	Test     bool
	IntentID string
	Subject  string
	Address  string // lowercased 0x — case is never trusted, see indexByAddress
	Status   cryptopaymentintent.Status
	TxHash   string // the sighting currently recorded on the intent
	Block    uint64
}

// Sighting is a deposit seen on chain but not yet deep enough to credit. It
// drives the intent's display state only (pending → confirming) and moves no
// money, which is why it may be recorded from a shallow block.
type Sighting struct {
	Org           string
	IntentID      string
	TxHash        string
	Block         uint64
	Confirmations int
}

// Credit is one fully-confirmed deposit to be turned into spendable balance.
//
// DedupKey is the whole exactly-once story: it names the on-chain event, not
// this run, so the ledger row it produces is the same row no matter how many
// times it is computed — by a re-scan, by a second replica, by a restart
// mid-pass. See depositledger.creditKey.
type Credit struct {
	Org           string
	Subject       string
	IntentID      string
	Chain         string
	Token         string
	AmountCents   int64
	Units         string // raw token base units, decimal (audit trail)
	PegRate       string // USD per whole token at the peg used ("1.00")
	TxHash        string
	LogIndex      uint64
	Block         uint64
	Confirmations int
	Test          bool
	DedupKey      string // chain:txHash:logIndex
}

// Store is the intent + ledger side of the world.
//
// Credit MUST be idempotent on Credit.DedupKey, and MUST write the ledger row
// BEFORE advancing the intent. That order is not a preference: if the intent
// advanced first and the ledger write then failed, a re-run would find a
// succeeded intent and could decide the work was done — money received, never
// credited, which is the exact bug this package exists to fix. In the other
// order the worst case is a re-run that re-writes an identical row.
//
// WHICH FAILURES ARE FATAL IS THE STORE'S CALL, not this package's, and the
// watcher simply propagates what it is told. The store is the only layer that
// can tell a money write from a display write: an unreadable intent list hides
// addresses and so must stop the pass, while a stale intent status is a display
// defect that must NOT wedge every other customer's deposits behind it.
type Store interface {
	// Watched returns every minted deposit address for (chain, token), across
	// every org.
	Watched(ctx context.Context, chain, token string) ([]Watched, error)
	// Sight records a not-yet-final deposit. Idempotent; a no-op for an intent
	// that has already moved on.
	Sight(ctx context.Context, s Sighting) error
	// Unsight clears a sighting whose transaction left the canonical chain,
	// returning the intent to pending.
	Unsight(ctx context.Context, s Sighting) error
	// Credit writes the idempotent ledger credit, then advances the intent. It
	// reports whether it WROTE a new credit, so that re-crediting an already
	// credited transfer — which happens on every pass for as long as it stays in
	// the re-scan window — is counted and logged as the no-op it is. A counter
	// that ticked up each time would read exactly like a double-credit.
	Credit(ctx context.Context, c Credit) (written bool, err error)
}

// Cursor persists the last block fully committed per asset, so a restart
// resumes instead of rescanning (or, worse, skipping).
type Cursor interface {
	Last(ctx context.Context, assetKey string) (uint64, error)
	Save(ctx context.Context, assetKey string, block uint64) error
}

// Bound pairs a configured asset with the reader that can see it.
type Bound struct {
	Asset  Asset
	Reader Reader
}

// asset is a Bound plus the decimals VERIFIED against the contract itself.
type asset struct {
	Asset
	reader   Reader
	decimals int // 0 until verified; never used unverified
}

// Watcher scans configured assets and credits confirmed deposits exactly once.
type Watcher struct {
	mu       sync.Mutex // one pass at a time within a process
	assets   []*asset
	store    Store
	cursor   Cursor
	maxRange uint64
	maxAddrs int
}

const (
	// defaultMaxRange caps blocks per eth_getLogs call (public RPCs reject wide
	// windows). Same value husdindex settled on.
	defaultMaxRange = 2000
	// defaultMaxAddrs caps addresses per eth_getLogs topic filter. The watched
	// set grows with every deposit ever taken, and an unbounded topic array is
	// how a scan silently starts erroring at some customer count; chunking makes
	// the cost linear and the behaviour identical at 10 addresses and 10,000.
	defaultMaxAddrs = 100
)

// New builds a Watcher over the given bound assets.
func New(bound []Bound, store Store, cursor Cursor) *Watcher {
	w := &Watcher{
		store:    store,
		cursor:   cursor,
		maxRange: defaultMaxRange,
		maxAddrs: defaultMaxAddrs,
	}
	for _, b := range bound {
		w.assets = append(w.assets, &asset{Asset: b.Asset, reader: b.Reader})
	}
	return w
}

// Assets reports the configured assets (observability; no chain access).
func (w *Watcher) Assets() []Asset {
	out := make([]Asset, 0, len(w.assets))
	for _, a := range w.assets {
		out = append(out, a.Asset)
	}
	return out
}

// Sync runs one pass over every configured asset and returns the number of
// ledger credits written. One broken chain must not blind the others, so every
// asset is attempted and the failures are joined — a wedged RPC on Polygon
// cannot stop Base deposits from being credited.
func (w *Watcher) Sync(ctx context.Context) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	total := 0
	var errs []error
	for _, a := range w.assets {
		n, err := w.syncAsset(ctx, a)
		total += n
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", a.Key(), err))
		}
	}
	return total, errors.Join(errs...)
}

// syncAsset scans and credits ONE asset.
//
// The window is asymmetric on purpose — SCAN AHEAD, COMMIT BEHIND:
//
//	scan   [cursor+1 … head]          every pass, including the unconfirmed tip
//	commit head − requiredConfirmations
//
// Scanning to the tip is what lets a customer watch their deposit go
// "confirming" within a block. Committing behind is what makes a reorg
// harmless: the last `required` blocks are re-scanned on every pass, so a
// transaction that appears, disappears, or moves inside the reorg window is
// re-read from the canonical chain each time rather than trusted from the one
// moment we happened to look.
func (w *Watcher) syncAsset(ctx context.Context, a *asset) (int, error) {
	if err := a.verify(ctx); err != nil {
		return 0, err
	}

	head, err := a.reader.BlockNumber(ctx)
	if err != nil {
		return 0, err
	}
	required := uint64(cryptopaymentintent.RequiredConfirmationsForChain(cryptopaymentintent.Chain(a.Chain)))

	last, err := w.cursor.Last(ctx, a.Key())
	if err != nil {
		return 0, err
	}
	from := last + 1
	if last == 0 {
		// Fresh cursor (a first deploy, never a restart — the cursor is
		// persisted): start at the bottom of the confirmation window, so the very
		// first pass scans exactly the range it is about to commit and no block is
		// ever committed unscanned. There is no history behind that to backfill —
		// a deposit address only receives money after a running deploy mints it —
		// and cold-scanning a public chain from genesis would burn an RPC budget
		// to find nothing.
		from = 0
		if head > required {
			from = head - required
		}
	}

	watched, err := w.store.Watched(ctx, a.Chain, a.Token)
	if err != nil {
		return 0, err
	}
	byAddr, ambiguous := indexByAddress(watched)
	if len(ambiguous) > 0 {
		// Two intents claiming one address means we cannot say WHO a deposit
		// belongs to. Crediting either is a guess with someone's money, so the
		// asset stops here — no scan, no cursor advance — and resumes the moment
		// the collision is resolved. MPC mints a fresh wallet per keygen, so this
		// is a fail-closed assertion, not an expected state.
		return 0, fmt.Errorf("deposit address(es) claimed by more than one intent: %s", strings.Join(ambiguous, ", "))
	}
	addrs := make([]string, 0, len(byAddr))
	for addr := range byAddr {
		addrs = append(addrs, addr)
	}
	sort.Strings(addrs) // deterministic chunking

	if len(addrs) == 0 || from > head {
		return 0, w.commit(ctx, a.Key(), head, required, last)
	}

	transfers, err := w.scan(ctx, a, addrs, from, head)
	if err != nil {
		return 0, err
	}

	credited := 0
	observed := make(map[string]bool, len(transfers))
	for _, t := range transfers {
		to := strings.ToLower(t.To)
		wt, ok := byAddr[to]
		if !ok {
			continue // not one of ours (the topic filter should already prevent this)
		}
		if t.Block > head {
			continue // a reader that answered outside the window it was asked for
		}
		observed[seenKey(wt.IntentID, t.TxHash)] = true

		depth := int(head - t.Block + 1)
		if uint64(depth) < required {
			if err := w.store.Sight(ctx, Sighting{
				Org: wt.Org, IntentID: wt.IntentID,
				TxHash: strings.ToLower(t.TxHash), Block: t.Block, Confirmations: depth,
			}); err != nil {
				return credited, err
			}
			continue
		}

		cents, err := AmountCents(t.ValueWei, a.decimals, a.PegCents())
		if errors.Is(err, ErrDust) {
			continue // a real transfer worth less than a cent: nothing to credit
		}
		if err != nil {
			return credited, fmt.Errorf("%s: %w", dedupKey(a.Chain, t), err)
		}
		written, err := w.store.Credit(ctx, Credit{
			Org: wt.Org, Subject: wt.Subject, IntentID: wt.IntentID,
			Chain: a.Chain, Token: a.Token,
			AmountCents: cents, Units: t.ValueWei.String(), PegRate: a.PegRate(),
			TxHash: strings.ToLower(t.TxHash), LogIndex: t.LogIndex, Block: t.Block,
			Confirmations: depth, Test: wt.Test, DedupKey: dedupKey(a.Chain, t),
		})
		if err != nil {
			return credited, err
		}
		if written {
			credited++
		}
	}

	if err := w.correctReorgs(ctx, watched, observed, from, head); err != nil {
		return credited, err
	}
	return credited, w.commit(ctx, a.Key(), head, required, last)
}

// scan reads the transfers into the watched addresses over [from, head],
// chunked by block range AND by address, and returns them in chain order.
func (w *Watcher) scan(ctx context.Context, a *asset, addrs []string, from, head uint64) ([]husdindex.Transfer, error) {
	var out []husdindex.Transfer
	for start := from; start <= head; start += w.maxRange {
		end := start + w.maxRange - 1
		if end > head || end < start {
			end = head
		}
		for _, chunk := range chunkStrings(addrs, w.maxAddrs) {
			got, err := a.reader.TransfersTo(ctx, chunk, start, end)
			if err != nil {
				return nil, err
			}
			out = append(out, got...)
		}
	}
	// Deterministic order: block, then log index. Two replicas processing the
	// same window therefore do the same work in the same order.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Block != out[j].Block {
			return out[i].Block < out[j].Block
		}
		return out[i].LogIndex < out[j].LogIndex
	})
	return out, nil
}

// correctReorgs returns an intent to pending when the deposit it is confirming
// is no longer on the canonical chain.
//
// It is safe precisely because of the commit-behind window: an intent stays
// `confirming` only while its transaction is shallower than the confirmation
// depth, and every such block is inside [from, head] on every pass. So "we
// scanned the range that contains its block and eth_getLogs did not return it"
// means the transaction is gone, not that we failed to look. Blocks outside the
// scanned range are left alone — we did not look there and must not conclude
// anything from silence.
func (w *Watcher) correctReorgs(ctx context.Context, watched []Watched, observed map[string]bool, from, head uint64) error {
	for _, wt := range watched {
		if wt.Status != cryptopaymentintent.Confirming || wt.TxHash == "" {
			continue
		}
		if wt.Block < from || wt.Block > head {
			continue
		}
		if observed[seenKey(wt.IntentID, wt.TxHash)] {
			continue
		}
		if err := w.store.Unsight(ctx, Sighting{
			Org: wt.Org, IntentID: wt.IntentID,
			TxHash: strings.ToLower(wt.TxHash), Block: wt.Block,
		}); err != nil {
			return err
		}
	}
	return nil
}

// commit advances the cursor to the deepest fully-confirmed block. It never
// moves backwards and never commits a block whose deposits were not creditable
// this pass.
func (w *Watcher) commit(ctx context.Context, key string, head, required, last uint64) error {
	if head < required {
		return nil // chain too young to have a confirmed block
	}
	to := head - required
	if to <= last {
		return nil
	}
	return w.cursor.Save(ctx, key, to)
}

// verify asserts, once, that the configured contract IS the token it claims to
// be, and takes its decimals from the chain rather than from config.
//
// This is the boundary check that makes the amount arithmetic trustworthy. A
// mistyped contract address is otherwise undetectable from inside this process
// and would price some unrelated token at a dollar; a wrong decimals constant is
// a 10^12 error in the credit. Both become "this asset is refused" instead.
// Failure is not cached, so a transient RPC error retries on the next pass.
func (a *asset) verify(ctx context.Context) error {
	if a.decimals != 0 {
		return nil
	}
	sym, err := a.reader.Symbol(ctx)
	if err != nil {
		return fmt.Errorf("cannot read symbol() of %s: %w", a.Contract, err)
	}
	if !strings.EqualFold(strings.TrimSpace(sym), a.Token) {
		return fmt.Errorf("contract %s reports symbol %q, but it is configured as %q — refusing to credit deposits of a token we cannot identify",
			a.Contract, strings.TrimSpace(sym), a.Token)
	}
	d, err := a.reader.Decimals(ctx)
	if err != nil {
		return fmt.Errorf("cannot read decimals() of %s: %w", a.Contract, err)
	}
	if d < 2 || d > 36 {
		return fmt.Errorf("contract %s reports %d decimals, which cannot express a cent", a.Contract, d)
	}
	a.decimals = d
	return nil
}

// indexByAddress maps lowercased address → the intent that owns it, and reports
// any address claimed by more than one intent.
//
// Case is normalised on BOTH sides and never compared raw: the custody service
// returns EIP-55 checksummed addresses while chain logs are lowercase hex, so
// comparing what we stored against what we read would miss every deposit.
func indexByAddress(watched []Watched) (map[string]Watched, []string) {
	byAddr := make(map[string]Watched, len(watched))
	claims := make(map[string]int, len(watched))
	for _, wt := range watched {
		addr := strings.ToLower(strings.TrimSpace(wt.Address))
		if addr == "" {
			continue
		}
		claims[addr]++
		byAddr[addr] = wt
	}
	var ambiguous []string
	for addr, n := range claims {
		if n > 1 {
			ambiguous = append(ambiguous, addr)
			delete(byAddr, addr)
		}
	}
	sort.Strings(ambiguous)
	return byAddr, ambiguous
}

// dedupKey names the on-chain event a credit came from. The CHAIN is part of it:
// EVM chains share an address space and a pre-EIP-155 transaction can be
// replayed onto another chain with an identical hash, so two genuine deposits
// could otherwise collapse into one credit.
func dedupKey(chain string, t husdindex.Transfer) string {
	return fmt.Sprintf("%s:%s:%d", chain, strings.ToLower(t.TxHash), t.LogIndex)
}

func seenKey(intentID, txHash string) string {
	return intentID + "|" + strings.ToLower(txHash)
}

// chunkStrings splits s into runs of at most n.
func chunkStrings(s []string, n int) [][]string {
	if n <= 0 || len(s) <= n {
		return [][]string{s}
	}
	out := make([][]string, 0, (len(s)+n-1)/n)
	for i := 0; i < len(s); i += n {
		end := i + n
		if end > len(s) {
			end = len(s)
		}
		out = append(out, s[i:end])
	}
	return out
}
