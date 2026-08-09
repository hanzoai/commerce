package depositwatch

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"sync"

	"github.com/hanzoai/commerce/models/cryptopaymentintent"
)

// Reader reads ONE asset's chain state. billing/solanarpc.Client satisfies it
// directly and *husdindex.Client through a two-line adapter (the EVM client
// predates this interface and serves the HUSD indexer too); the tests inject a
// fake so every crediting decision below is proven without a chain.
//
// Nothing in it is EVM-shaped. "Block" is whatever position that chain counts
// monotonically — a block on the EVM, a slot on Solana, a masterchain seqno on
// TON, a ledger index on XRPL — and the only property relied on is that it
// increases and that depth beneath the head means age.
type Reader interface {
	BlockNumber(ctx context.Context) (uint64, error)
	TransfersTo(ctx context.Context, addrs []string, fromBlock, toBlock uint64) ([]Transfer, error)
	Decimals(ctx context.Context) (int, error)
	Symbol(ctx context.Context) (string, error)
}

// Transfer is one value-moving event that landed in a watched address.
//
// It is deliberately NOT husdindex.Transfer. That type is an ERC-20 Transfer
// LOG — a thing with topics and a block-scoped log index — and describing a
// Solana balance change with it would mean writing an instruction position into
// a field called LogIndex and hoping every later reader understood. The two
// concepts are kept apart so the per-chain reading of each field can be stated
// once, here, and never guessed at:
//
//	field       EVM                    Solana                 TON                     XRPL
//	─────────── ────────────────────── ────────────────────── ─────────────────────── ──────────────────────
//	To          recipient address      the OWNER address we   the OWNER address we    the POOLED custody
//	                                   minted, not the token  minted, not the jetton  account — shared by
//	                                   account it landed in   wallet it landed in     every payer
//	Tag         (none)                 (none)                 (none)                  destination tag
//	Units       raw ERC-20 amount      post − pre balance     transferred amount,     delivered_amount,
//	                                                          jetton base units       rendered at xrplrpc.Scale
//	TxHash      transaction hash       transaction signature  the RECEIVING wallet's  transaction hash
//	                                                          transaction hash
//	EventIndex  log index              index of the token-    0 — a jetton wallet     0 — a Payment delivers
//	                                   balance record         transaction receives    to exactly one tagged
//	                                                          exactly one transfer    destination
//	Block       block number           slot                   masterchain seqno       ledger index
type Transfer struct {
	To    string
	Units *big.Int

	// Tag is the chain-native discriminator that, WITH To, says which intent
	// this transfer belongs to. It is empty on every chain that mints one
	// address per payer, and it is the destination tag on XRPL, where every
	// payer shares one address and the tag is the only thing that says whose
	// money arrived. See Asset.Identity and Asset.Pooled.
	//
	// An empty Tag on a pooled chain is NOT "tag zero" — it means the payment
	// carried no tag at all, which names nobody. See Unattributed.
	Tag string

	TxHash string

	// EventIndex is the position of this event WITHIN its transaction, under
	// that chain's own definition of position.
	//
	// It exists because exactly-once is a property of the dedup key, and one
	// transaction can credit the same address twice — an EVM transaction can
	// emit two Transfer logs to one address, a Solana transaction can move
	// tokens into two watched accounts. The pair (TxHash, EventIndex) must
	// therefore name the EVENT, and it must be a function of the chain's own
	// record of it: a re-scan, a crash retry and a second replica all recompute
	// it from the same bytes and land on the same ledger row.
	//
	// What it is NOT: a counter over the results of this scan, or a position in
	// a slice. Either would renumber when the window moved, and renumbering a
	// dedup key is a double credit.
	EventIndex uint64

	Block uint64
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
	Address  string // as minted; normalised for comparison by Asset.Fold
	// Tag is the routing tag this intent was issued, on a chain where the
	// address is shared. Empty everywhere else. Address and Tag are never
	// compared separately — Asset.Identity combines them, once, for both the
	// minted side and the observed side.
	Tag    string
	Status cryptopaymentintent.Status
	TxHash string // the sighting currently recorded on the intent
	Block  uint64
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
// mid-pass. See dedupKey below and depositledger.creditKey.
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
	EventIndex    uint64 // see Transfer.EventIndex
	Block         uint64
	Confirmations int
	Test          bool
	DedupKey      string // chain:txHash:eventIndex
}

// Unattributed is money that arrived at a custody address we own and named
// NOBODY.
//
// It exists only on a pooled chain (Asset.Pooled), because only there can a
// payment reach a destination we control without saying whose it is: an XRPL
// payment to the shared account with no destination tag, or with a tag matching
// no intent we ever issued. On every other chain the address IS the answer, so
// this is unreachable by construction.
//
// The two obvious things to do with it are both wrong:
//
//	credit it   to whom? Guessing is spending one customer's deposit on another
//	            customer's balance.
//	drop it     it is somebody's real money. Silently discarding it is the exact
//	            class of failure this whole package exists to end, and it would
//	            be INVISIBLE — the scan window moves on and the payment is never
//	            looked at again.
//
// So it is neither. It is recorded, durably and idempotently on the same
// DedupKey a credit would have used, and it credits nothing. That leaves a
// permanent, reconcilable record an operator can refund from or attribute by
// hand, and it deliberately does NOT stop the pass: a stranger can send one drop
// to a published custody address, and a rail that wedged on that would be a
// denial of service anyone could trigger against every other customer's
// deposit.
type Unattributed struct {
	Chain   string
	Token   string
	Address string // the pooled custody address it landed in
	Tag     string // the tag it carried; EMPTY means it carried none at all
	Units   string // raw base units, decimal (audit trail)
	TxHash  string
	// EventIndex and Block are the same chain facts a Credit carries; see
	// Transfer.
	EventIndex uint64
	Block      uint64
	DedupKey   string // chain:txHash:eventIndex
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
	// RecordUnattributed durably records money that arrived and named nobody.
	// Idempotent on Unattributed.DedupKey.
	//
	// Unlike Sight, a failure here IS fatal to the pass, and the asymmetry is
	// the point: a failed sighting loses a display update that the next pass
	// redoes, while a failed record here would let the cursor advance past the
	// only evidence that a customer's money exists. Refusing parks the cursor
	// and retries — the block is still inside the re-scan window.
	RecordUnattributed(ctx context.Context, u Unattributed) error
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
	mu     sync.Mutex // one pass at a time within a process
	assets []*asset
	store  Store
	cursor Cursor
	// terms resolves an ORG's deductions, overriding the asset's platform
	// default. Nil means every org is on the default — which is the behaviour
	// this rail had before terms existed, and the right one for a deployment
	// that charges everybody the same.
	terms    TermsResolver
	maxRange uint64
	maxAddrs int
}

// WithTerms makes the watcher multi-tenant about what it deducts.
//
// It is a separate setter rather than a constructor argument because charging
// every org the same is a legitimate configuration, and a required argument
// would make "nobody has negotiated anything" look like a missing dependency.
func (w *Watcher) WithTerms(r TermsResolver) *Watcher {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.terms = r
	return w
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
	byID, addrs, ambiguous := indexWatched(watched, a)
	if len(ambiguous) > 0 {
		// Two intents claiming one identity means we cannot say WHO a deposit
		// belongs to. Crediting either is a guess with someone's money, so the
		// asset stops here — no scan, no cursor advance — and resumes the moment
		// the collision is resolved. MPC mints a fresh wallet per keygen and a
		// fresh tag per pooled intent, so this is a fail-closed assertion, not an
		// expected state.
		return 0, fmt.Errorf("deposit identit(ies) claimed by more than one intent: %s", strings.Join(ambiguous, ", "))
	}
	// The set of addresses we OWN, which is not the set of identities: on a
	// pooled chain thousands of identities share one address. Money landing at
	// an owned address that names no identity is unattributed (below); money
	// landing anywhere else is a reader that answered outside its filter.
	owned := make(map[string]bool, len(addrs))
	for _, addr := range addrs {
		owned[addr] = true
	}

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
		if t.Block > head {
			continue // a reader that answered outside the window it was asked for
		}
		depth := int(head - t.Block + 1)
		wt, ok := byID[a.Identity(t.To, t.Tag)]
		if !ok {
			if !owned[a.Fold(t.To)] {
				continue // not one of ours (the reader's filter should already prevent this)
			}
			// Money at an address we own that names NO intent: on a pooled chain,
			// a payment with no routing tag or a tag we never issued. It cannot be
			// credited — to whom? — and it must not vanish. Recorded, not credited,
			// and not fatal. See Unattributed.
			//
			// Recorded only once it is as deep as a CREDIT would need to be, so
			// the record describes the canonical chain and not a transaction that
			// may still be reorganised out from under it.
			if uint64(depth) < required {
				continue
			}
			if err := w.store.RecordUnattributed(ctx, Unattributed{
				Chain: a.Chain, Token: a.Token,
				Address: a.Fold(t.To), Tag: strings.TrimSpace(t.Tag),
				Units: t.Units.String(), TxHash: a.Fold(t.TxHash),
				EventIndex: t.EventIndex, Block: t.Block,
				DedupKey: a.dedupKey(t),
			}); err != nil {
				return credited, err
			}
			continue
		}
		txHash := a.Fold(t.TxHash)
		observed[seenKey(wt.IntentID, a.Fold, txHash)] = true

		if uint64(depth) < required {
			if err := w.store.Sight(ctx, Sighting{
				Org: wt.Org, IntentID: wt.IntentID,
				TxHash: txHash, Block: t.Block, Confirmations: depth,
			}); err != nil {
				return credited, err
			}
			continue
		}

		// Terms are resolved PER ORG, per credit: the asset carries the platform
		// default and this org may be on something else. Resolved here rather
		// than once per pass because one pass credits many orgs.
		terms, err := resolveTerms(ctx, w.terms, wt.Org, a.Chain, a.Terms)
		if err != nil {
			return credited, err
		}
		cents, err := AmountCents(t.Units, a.decimals, a.PegCents(), terms)
		if errors.Is(err, ErrDust) {
			continue // a real transfer worth less than a cent: nothing to credit
		}
		if errors.Is(err, ErrUnderFee) {
			// Real money arrived and does not cover the cost of moving it, so
			// there is nothing to credit — the same outcome as dust, reached for
			// a different reason.
			//
			// It is NOT announced from here. This package writes no logs by
			// design, and a log would be the wrong remedy anyway: a customer
			// learning after the fact that their deposit bought nothing is a
			// failure of DISCLOSURE, and disclosure belongs at the quote, before
			// they send. The minimum is a number the picker should state.
			continue
		}
		if err != nil {
			return credited, fmt.Errorf("%s: %w", a.dedupKey(t), err)
		}
		written, err := w.store.Credit(ctx, Credit{
			Org: wt.Org, Subject: wt.Subject, IntentID: wt.IntentID,
			Chain: a.Chain, Token: a.Token,
			AmountCents: cents, Units: t.Units.String(), PegRate: a.PegRate(),
			TxHash: txHash, EventIndex: t.EventIndex, Block: t.Block,
			Confirmations: depth, Test: wt.Test, DedupKey: a.dedupKey(t),
		})
		if err != nil {
			return credited, err
		}
		if written {
			credited++
		}
	}

	if err := w.correctReorgs(ctx, a, watched, observed, from, head); err != nil {
		return credited, err
	}
	return credited, w.commit(ctx, a.Key(), head, required, last)
}

// scan reads the transfers into the watched addresses over [from, head],
// chunked by block range AND by address, and returns them in chain order.
func (w *Watcher) scan(ctx context.Context, a *asset, addrs []string, from, head uint64) ([]Transfer, error) {
	var out []Transfer
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
	// Deterministic order: block, then event index. Two replicas processing the
	// same window therefore do the same work in the same order.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Block != out[j].Block {
			return out[i].Block < out[j].Block
		}
		return out[i].EventIndex < out[j].EventIndex
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
func (w *Watcher) correctReorgs(ctx context.Context, a *asset, watched []Watched, observed map[string]bool, from, head uint64) error {
	for _, wt := range watched {
		if wt.Status != cryptopaymentintent.Confirming || wt.TxHash == "" {
			continue
		}
		if wt.Block < from || wt.Block > head {
			continue
		}
		if observed[seenKey(wt.IntentID, a.Fold, wt.TxHash)] {
			continue
		}
		if err := w.store.Unsight(ctx, Sighting{
			Org: wt.Org, IntentID: wt.IntentID,
			TxHash: a.Fold(wt.TxHash), Block: wt.Block,
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

// verify asserts, once, that the account the asset is configured with IS the
// token it claims to be, and takes its decimals from the chain rather than from
// config.
//
// This is the boundary check that makes the amount arithmetic trustworthy. A
// mistyped token address is otherwise undetectable from inside this process and
// would price some unrelated token at a dollar; a wrong decimals constant is a
// 10^12 error in the credit. Both become "this asset is refused" instead.
// Failure is not cached, so a transient RPC error retries on the next pass.
//
// Both reads are the CHAIN's answer, not ours, in whatever form that chain
// offers it:
//
//	EVM     the ERC-20 answers symbol() and decimals() itself.
//	Solana  the SPL mint carries its decimals; the name lives in a separate
//	        Metaplex account.
//	TON     the jetton master's ON-CHAIN TEP-64 content dictionary carries the
//	        decimals — and, for some jettons, nothing else. One that publishes no
//	        on-chain symbol is refused here rather than identified from the
//	        off-chain document its content points at.
//	XRPL    an issued currency has no decimals to read AT ALL, because it has no
//	        base unit: the ledger states a decimal number. The reader renders it
//	        at a scale it reports itself, so the parse and the scale cannot
//	        disagree; identity comes from asking the issuer what it issues.
//
// What each answer is worth is documented at the reader; what this function
// requires is the same in every case: identify the token, or credit nothing.
func (a *asset) verify(ctx context.Context) error {
	if a.decimals != 0 {
		return nil
	}
	sym, err := a.reader.Symbol(ctx)
	if err != nil {
		return fmt.Errorf("cannot read the symbol of %s: %w", a.Contract, err)
	}
	if !strings.EqualFold(strings.TrimSpace(sym), a.Token) {
		return fmt.Errorf("token account %s reports symbol %q, but it is configured as %q — refusing to credit deposits of a token we cannot identify",
			a.Contract, strings.TrimSpace(sym), a.Token)
	}
	d, err := a.reader.Decimals(ctx)
	if err != nil {
		return fmt.Errorf("cannot read the decimals of %s: %w", a.Contract, err)
	}
	if d < 2 || d > 36 {
		return fmt.Errorf("token account %s reports %d decimals, which cannot express a cent", a.Contract, d)
	}
	a.decimals = d
	return nil
}

// indexWatched maps deposit IDENTITY → the intent that owns it, lists the
// distinct addresses to ask the chain about, and reports any identity claimed by
// more than one intent.
//
// The identity and the address are separated on purpose, because on a pooled
// chain they are not the same thing and conflating them is a mis-credit:
//
//	EVM / Solana / TON  one address per payer, so identity == address and the
//	                    two outputs are the same set.
//	XRPL                ten thousand identities over ONE address. The chain is
//	                    asked about one address; the tag decides whose money it
//	                    was.
//
// Both sides of every comparison go through the SAME per-chain functions and
// are never compared raw. On the EVM that means a case fold, because the custody
// service returns EIP-55 checksummed addresses while chain logs are lowercase
// hex and comparing them literally would miss every deposit; on the non-EVM
// chains it means leaving case alone, because their encodings are
// case-significant. See Asset.Fold and Asset.Identity.
// AMBIGUITY IS ABOUT WHOSE MONEY IT IS, NOT ABOUT HOW MANY INTENTS THERE ARE.
//
// Counting intents per identity looked like the same question and is not, and
// the difference stopped every EVM asset dead. Two decisions upstream are each
// right on their own:
//
//   - Watched() enumerates intents REGARDLESS of status, deliberately: money
//     sent to an expired or already-settled intent is still the customer's.
//   - CreateCryptoDeposit reuses ONE address per (payer, chain, token) — on a
//     per-payer chain the address is derived from that payer's key, so it is the
//     same address every time by construction, not by accident.
//
// Together they mean the SECOND deposit a customer ever makes leaves two intents
// on one address, and a count-based test reads that as a collision and halts the
// whole asset — permanently, since the old intent never goes away. Observed in
// production as `ethereum:usdc … claimed by more than one intent`, once per pass,
// with the cursor frozen behind it.
//
// But both intents name the SAME payer, so nothing is unclear: whichever one the
// sighting is recorded against, the credit goes to the same person. A genuine
// collision is two DIFFERENT payers on one identity — that is the case where
// crediting either is a guess with someone's money, and it still fails closed.
//
// Among a payer's own intents, an open one wins so the sighting lands on the
// intent the customer is currently looking at; ties break on IntentID so a pass
// is reproducible.
func indexWatched(watched []Watched, a *asset) (byID map[string]Watched, addrs []string, ambiguous []string) {
	byID = make(map[string]Watched, len(watched))
	subjects := make(map[string]map[string]bool, len(watched))
	addrSet := make(map[string]bool, len(watched))
	for _, wt := range watched {
		addr := a.Fold(wt.Address)
		if addr == "" {
			continue
		}
		// The address is watched even if its identity turns out to be ambiguous:
		// dropping it would stop the chain being asked about an address we own,
		// which is how money arrives unseen. An ambiguous identity fails the pass
		// below anyway.
		addrSet[addr] = true
		id := a.Identity(wt.Address, wt.Tag)
		if subjects[id] == nil {
			subjects[id] = make(map[string]bool, 1)
		}
		subjects[id][wt.Subject] = true
		if cur, seen := byID[id]; !seen || preferIntent(wt, cur) {
			byID[id] = wt
		}
	}
	for id, subs := range subjects {
		if len(subs) > 1 {
			ambiguous = append(ambiguous, id)
			delete(byID, id)
		}
	}
	for addr := range addrSet {
		addrs = append(addrs, addr)
	}
	sort.Strings(addrs) // deterministic chunking
	sort.Strings(ambiguous)
	return byID, addrs, ambiguous
}

// preferIntent picks between two intents of the SAME payer on one identity.
// Open beats terminal — that is the one the customer has on screen — and
// otherwise the higher IntentID wins so the choice does not depend on the order
// orgs happened to enumerate in.
func preferIntent(candidate, current Watched) bool {
	if co, cu := intentOpen(candidate), intentOpen(current); co != cu {
		return co
	}
	return candidate.IntentID > current.IntentID
}

// intentOpen reports whether an intent is still waiting on money. Written as an
// allow-list of the two live states rather than a deny-list of the terminal
// ones, so a status added later is treated as terminal — which only costs a
// preference between one payer's own intents, where a deny-list would silently
// promote an unknown state to "the customer is looking at this".
func intentOpen(w Watched) bool {
	return w.Status == cryptopaymentintent.Pending || w.Status == cryptopaymentintent.Confirming
}

// dedupKey names the on-chain event a credit came from: chain, transaction,
// position within that transaction.
//
// All three parts are load-bearing.
//
//	chain       EVM chains share an address space, and a pre-EIP-155
//	            transaction can be replayed onto another chain with an
//	            identical hash. Without it, two genuine deposits on two chains
//	            collapse into one credit — the customer pays twice and is
//	            credited once.
//	transaction the chain's own identifier for the transaction (a hash on the
//	            EVM, a signature on Solana, a hash rendered canonically as
//	            lowercase hex on TON and XRPL), folded the way that chain's
//	            identifiers compare.
//	eventIndex  which value movement WITHIN that transaction, since one
//	            transaction can credit the same address more than once and can
//	            credit two different addresses. Without it, the second one is
//	            silently swallowed as a duplicate of the first. It is 0 on TON
//	            and XRPL, where it is not a placeholder but a fact about the
//	            chain: a TON transaction has at most one inbound message and an
//	            XRPL Payment has exactly one destination, so the transaction
//	            names the event by itself.
//
// The routing TAG is deliberately absent. The key names the on-chain event, and
// which intent that event was routed to is a different question, answered by
// Asset.Identity. Putting the tag in here would make one payment produce two
// ledger rows if it were ever re-read against a corrected tag.
//
// It is a function of the EVENT and of nothing else — not of this pass, not of
// the scan window, not of the reader — which is exactly why a re-scan, a crash
// retry and N replicas all produce the same ledger row.
func (a *asset) dedupKey(t Transfer) string {
	return fmt.Sprintf("%s:%s:%d", a.Chain, a.Fold(t.TxHash), t.EventIndex)
}

func seenKey(intentID string, fold func(string) string, txHash string) string {
	return intentID + "|" + fold(txHash)
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
