package depositledger

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/db"
)

// The routing tag that says WHOSE a pooled deposit is.
//
// On every other chain in this rail the deposit address answers that: one payer,
// one address, and the address is minted per deposit. XRPL charges a
// non-refundable base reserve for every funded account, so the model there is
// ONE custody account shared by everybody plus a per-deposit DESTINATION TAG —
// which makes the tag, and not the address, the thing that must be unique.
//
// THE TAG BELONGS HERE AND NOT TO CUSTODY. It is a routing fact — which intent
// does this payment belong to — not a key fact. Custody derives and controls
// the account; commerce decides who gets which tag, because commerce is what
// knows there is an intent at all. The code agrees: the signer's GenerateAddress
// mints a FRESH WALLET per call by design, which on XRPL would strand a reserve
// on every single deposit, and it derives no XRPL key anyway (its chain switch
// falls through to the EVM address). Asking custody for a tag would be asking
// the wrong service a question it has no way to answer without being told about
// intents.
//
// WHAT MAKES A TAG UNIQUE — the whole job, because a duplicate does not
// mis-credit, it HALTS: the watcher refuses an identity claimed by two intents
// rather than guess whose money arrived, and that stops the asset for every
// customer, not just the two.
//
// The tag is ALLOCATED BY THE DATABASE, not chosen and then checked. NextTag
// makes exactly one call to db.Sequencer.NextSequence, which is a single
// INSERT … ON CONFLICT DO UPDATE … RETURNING statement against one row. No
// value is ever read into Go and written back, so there is no window in which
// two callers can both see N: concurrent statements queue on that row and each
// returns a distinct number. That holds across goroutines, across requests, and
// across replicas — replicas share the row — and it needs no lock, no lease and
// no leader election, exactly like the deterministic key that makes the ledger
// credit idempotent.
//
// What was deliberately NOT built, and why:
//
//	a random 32-bit tag   32 bits is 77,000 tags to a coin-flip collision, and
//	                      the collision halts the rail. "Unlikely" is not a
//	                      property to hand a custody account.
//	random + a check      a query cannot see a row a concurrent request has not
//	                      written yet. It converts a certainty into a race and
//	                      reads like a guarantee, which is worse than neither.
//	derived from the      no function from an unbounded payer key into 2^32 is
//	  payer or intent id  injective, so this is the random case wearing a hat.

// tagSequence names the counter a pooled chain's tags come from.
//
// It is keyed by CHAIN and deliberately not by (chain, token), because the
// thing being kept unique is a position on an ACCOUNT and one account holds
// every currency sent to it. Two tokens on one pooled chain therefore draw from
// one sequence, and no tag is ever issued twice against that account even
// though the watcher scans each token separately.
func tagSequence(chain string) string { return "crypto-deposit-tag:" + chain }

// NextTag allocates this intent's destination tag on a pooled chain.
//
// It returns the tag rendered the way the intent stores it and the way the
// reader compares it: decimal, with NO padding and no prefix. "0" is a real tag
// and the FIRST one issued — XRPL destination tags are uint32 and 0 is legal,
// so somebody will hold it. Emptiness is what means "no tag", and the two are
// distinguishable everywhere it matters (depositwatch.Asset.Identity renders
// them as `addr#` and `addr#0`), which is exactly why 0 must not be skipped or
// treated as absent.
func NextTag(ctx context.Context, chain string) (string, error) {
	if !depositwatch.Pooled(chain) {
		// Asking for a tag on a per-payer chain is a caller bug, and answering
		// would be worse than refusing: a tag on a chain whose reader never
		// reads one produces an intent whose identity nothing on chain can ever
		// match, so its deposits would be credited to nobody forever.
		return "", fmt.Errorf("depositledger: chain %q mints one address per payer and has no destination tags", chain)
	}

	seq, ok := systemDB().DB().(db.Sequencer)
	if !ok {
		// Fail closed. There is no safe way to pick a tag without an atomic
		// allocator — see the comment above — so a store that cannot allocate
		// means no XRPL deposits, not deposits with guessed tags.
		return "", fmt.Errorf("depositledger: this store cannot allocate destination tags atomically, so none will be issued")
	}

	n, err := seq.NextSequence(ctx, tagSequence(chain))
	if err != nil {
		return "", fmt.Errorf("depositledger: allocate %s destination tag: %w", chain, err)
	}
	if n > math.MaxUint32 {
		// 4.29 billion deposits on one account. The next step is a SECOND
		// custody account, not a wrapped tag: wrapping would re-issue a tag an
		// old intent still holds, and old intents are watched forever (see
		// Watched, which filters on the asset and never on status).
		return "", fmt.Errorf("depositledger: %s destination tags are exhausted at %d — the pooled account needs a successor", chain, n)
	}
	return strconv.FormatUint(n, 10), nil
}
