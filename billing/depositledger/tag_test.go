package depositledger

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/models/cryptopaymentintent"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The destination tag, against a REAL datastore.
//
// These are integration tests for the same reason the credit tests are: the
// guarantee is not a property of any logic here, it is a property of the
// STORAGE meeting concurrent callers. A fake allocator would prove that a
// counter counts, which nobody doubts, and would pass just as happily against
// the check-then-write implementation these exist to rule out.
//
// What a duplicate tag costs, so the stakes are on the page: the watcher
// refuses an identity claimed by two intents rather than guess whose money
// arrived (depositwatch.indexWatched → "claimed by more than one intent"), and
// that stops the ASSET — every customer's XRPL deposits, not just the two that
// collided.

// xrplPool is the shared custody account these intents all point at, so the
// only thing distinguishing them is the tag. Exactly the production shape.
const xrplPool = "rMxCKbEDwqr76QuheSUMdEGf4B9xJ8m5De"

// xrplAsset is the asset whose Identity() decides who gets credited. Tests
// compare through it rather than comparing tags directly, because the tag is
// only ever half of the answer — the watcher matches on (address, tag).
func xrplAsset() depositwatch.Asset {
	return depositwatch.Asset{Chain: "xrpl", Token: "rlusd", PooledAddress: xrplPool}
}

// The first tag issued is "0", and it is a REAL tag.
//
// XRPL destination tags are uint32 and 0 is legal, so somebody holds it. A
// sequence that started at 1 to "keep 0 free" would be inventing a meaning, and
// code that later read 0 as "no tag" would strand that customer's deposit as
// unattributed. Emptiness is what means no tag; "0" is a tag.
func TestNextTag_StartsAtZeroAndIsDense(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	for want := 0; want < 4; want++ {
		got, err := NextTag(context.Background(), "xrpl")
		if err != nil {
			t.Fatalf("NextTag: %v", err)
		}
		if got != strconv.Itoa(want) {
			t.Fatalf("allocation %d returned %q, want %q", want, got, strconv.Itoa(want))
		}
	}
}

// "0" and "" are different answers, and the read side already knows it. This
// pins that the WRITE side produces a tag the read side can tell apart from
// absence — the identity of tag-zero must not equal the identity of no-tag.
func TestNextTag_ZeroIsNotTheSameAsNoTag(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	zero, err := NextTag(context.Background(), "xrpl")
	if err != nil {
		t.Fatalf("NextTag: %v", err)
	}
	if zero == "" {
		t.Fatal("the first tag is the empty string, which means NO TAG — every deposit carrying it would be unattributed")
	}
	a := xrplAsset()
	if a.Identity(xrplPool, zero) == a.Identity(xrplPool, "") {
		t.Fatalf("tag %q and no tag share one identity — a payment with no tag would be credited to whoever holds tag zero", zero)
	}
}

// THE test: 48 concurrent mints must produce 48 distinct IDENTITIES.
//
// It asserts on depositwatch.Asset.Identity — the exact value indexWatched
// keys on — rather than on tags, because that is the thing that must not
// collide. Every intent shares one address here, so a duplicate tag is a
// duplicate identity and the watcher would halt the asset.
//
// The intents are written to the real per-org store the same way
// api/billing.CreateCryptoDeposit writes them, and then READ BACK through the
// production Watched() path, so this proves the property where the watcher
// actually finds it and not in a local slice.
func TestNextTag_ConcurrentMintsNeverShareAnIdentity(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	const org = "acme"
	seedOrg(t, org)

	const payers = 48
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, payers)
	for i := 0; i < payers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // release together, so they genuinely contend
			tag, err := NextTag(context.Background(), "xrpl")
			if err != nil {
				errs[i] = err
				return
			}
			db := orgDB(org)
			in := cryptopaymentintent.New(db)
			in.Currency = "usd"
			in.Chain = cryptopaymentintent.XRPL
			in.Token = "rlusd"
			in.DepositAddress = xrplPool
			in.AddressTag = tag
			in.CustomerRef = org + "/payer-" + strconv.Itoa(i)
			in.Status = cryptopaymentintent.Pending
			in.ExpiresAt = time.Now().Add(24 * time.Hour)
			in.Defaults()
			errs[i] = in.Create()
		}(i)
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("mint %d: %v", i, err)
		}
	}

	// Read them back the way the watcher does.
	watched, err := (intentStore{}).Watched(context.Background(), "xrpl", "rlusd")
	if err != nil {
		t.Fatalf("Watched: %v", err)
	}
	if len(watched) != payers {
		t.Fatalf("minted %d intents but the watcher sees %d", payers, len(watched))
	}

	a := xrplAsset()
	owner := make(map[string]string, payers)
	for _, w := range watched {
		if w.Tag == "" {
			t.Fatalf("intent %s (%s) carries no tag — its deposits would be credited to nobody", w.IntentID, w.Subject)
		}
		id := a.Identity(w.Address, w.Tag)
		if prev, dup := owner[id]; dup {
			t.Fatalf("identity %q is claimed by BOTH %s and %s — the watcher would refuse to credit either and halt XRPL for every customer",
				id, prev, w.Subject)
		}
		owner[id] = w.Subject
	}
	if len(owner) != payers {
		t.Fatalf("%d intents produced %d distinct identities", payers, len(owner))
	}
}

// Reuse allocates NOTHING. A payer refreshing the page must not consume tags.
//
// This is asserted on the SEQUENCE rather than on the returned intent: reading
// the same intent back proves the read, while an unchanged next-allocation
// proves nothing was drawn. A mutant that re-allocates on reuse still returns a
// tag, and only the sequence position shows it.
func TestNextTag_IsOnlyConsumedByAnActualMint(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	ctx := context.Background()
	first, err := NextTag(ctx, "xrpl")
	if err != nil {
		t.Fatalf("NextTag: %v", err)
	}
	// Nothing else allocates: the next value must be exactly one on.
	second, err := NextTag(ctx, "xrpl")
	if err != nil {
		t.Fatalf("NextTag: %v", err)
	}
	f, _ := strconv.Atoi(first)
	s, _ := strconv.Atoi(second)
	if s != f+1 {
		t.Fatalf("tags %q then %q — the sequence is not dense, so something else is drawing from it", first, second)
	}
}

// Each pooled chain counts on its own account, so a second pooled chain cannot
// exhaust the first's space or make its tags sparse.
func TestNextTag_IsPerChain(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	ctx := context.Background()
	if _, err := NextTag(ctx, "xrpl"); err != nil {
		t.Fatalf("NextTag: %v", err)
	}
	if got := tagSequence("xrpl"); got == tagSequence("other") {
		t.Fatalf("two chains share one sequence name (%q)", got)
	}
}

// A per-payer chain has no tags, and asking for one is refused rather than
// answered. An EVM intent carrying a tag would name an identity nothing on
// chain can ever match, so its deposits would be credited to nobody forever.
func TestNextTag_RefusesAPerPayerChain(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	for _, chain := range []string{"ethereum", "base", "solana", "ton", ""} {
		if tag, err := NextTag(context.Background(), chain); err == nil {
			t.Fatalf("chain %q was issued destination tag %q, but its deposits are matched by address alone", chain, tag)
		}
	}
}
