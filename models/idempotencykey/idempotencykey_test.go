package idempotencykey

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

func nsDB(parent context.Context, ns string) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(parent, ns))
}

func TestGuard_FirstThenReplay(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := nsDB(c, "acme")

	g := Guard{Scope: "refund:ord_1", Key: "key_abc"}
	rec, replay, err := Begin(db, g)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if replay {
		t.Fatal("first Begin reported replay=true; want false")
	}
	if rec.Status != StatusStarted {
		t.Fatalf("status = %q; want started", rec.Status)
	}

	// Complete records the response.
	if err := Complete(rec, `{"ok":true,"refunded":1500}`); err != nil {
		t.Fatalf("complete: %v", err)
	}

	// A replay returns the stored completed record + response.
	rec2, replay2, err := Begin(db, g)
	if err != nil {
		t.Fatalf("begin replay: %v", err)
	}
	if !replay2 {
		t.Fatal("second Begin reported replay=false; want true")
	}
	if rec2.Status != StatusCompleted {
		t.Fatalf("replay status = %q; want completed", rec2.Status)
	}
	if rec2.Response != `{"ok":true,"refunded":1500}` {
		t.Fatalf("replay response = %q; not persisted", rec2.Response)
	}
}

func TestGuard_DifferentScopeSameKeyNoCollision(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := nsDB(c, "acme")

	_, replayA, _ := Begin(db, Guard{Scope: "refund:ord_1", Key: "same_key"})
	_, replayB, _ := Begin(db, Guard{Scope: "refund:ord_2", Key: "same_key"})
	if replayA || replayB {
		t.Fatalf("distinct scopes collided: A=%v B=%v", replayA, replayB)
	}
}

// TestGuard_ConcurrentSameKeyHasExactlyOneWinner is the property the whole
// package exists for, and the one a read-then-write cannot provide.
//
// N callers race on a first-ever key. The store's own claim decides, so EXACTLY
// ONE is told to perform the side effect; every other is told the key is taken.
// Before the claim, all N could observe "not started" and all N could move
// money — one record in the store, N disbursements in the world.
func TestGuard_ConcurrentSameKeyHasExactlyOneWinner(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	const n = 20
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		winners int
		fails   []error
	)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			db := nsDB(c, "acme")
			_, replay, err := Begin(db, Guard{Scope: "refund:ord_race", Key: "race_key"})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err != nil:
				fails = append(fails, err)
			case !replay:
				winners++
			}
		}()
	}
	wg.Wait()

	if len(fails) > 0 {
		t.Fatalf("%d of %d claims errored: %v", len(fails), n, fails[0])
	}
	if winners != 1 {
		t.Fatalf("%d of %d concurrent callers were told to move the money; want exactly 1", winners, n)
	}

	db := nsDB(c, "acme")
	recs := make([]*IdempotencyKey, 0, n)
	if _, err := Query(db).Filter("IdemKey=", "race_key").GetAll(&recs); err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("concurrent Begin created %d records; want 1", len(recs))
	}
}

func TestGuard_TenantIsolation(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	g := Guard{Scope: "refund:ord_1", Key: "shared"}
	if _, _, err := Begin(nsDB(c, "acme"), g); err != nil {
		t.Fatalf("acme begin: %v", err)
	}
	// Same scope+key in beta is a DIFFERENT record (namespace-scoped id column).
	_, replay, err := Begin(nsDB(c, "beta"), g)
	if err != nil {
		t.Fatalf("beta begin: %v", err)
	}
	if replay {
		t.Fatal("beta saw acme's idempotency record — tenant isolation broken")
	}
}

// TestGuard_AStartedGuardIsNeverReclaimed — the money rule. A guard whose
// operation started and never completed does NOT know whether the money moved,
// so the retry is refused rather than run: a retry costs a retry, and running
// costs a duplicate disbursement. There is no clock that makes that safe.
func TestGuard_AStartedGuardIsNeverReclaimed(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := nsDB(c, "acme")

	g := Guard{Scope: "refund:ord_x", Key: "k"}
	if _, replay, err := Begin(db, g); err != nil || replay {
		t.Fatalf("first begin: err=%v replay=%v", err, replay)
	}
	for i := 0; i < 3; i++ {
		if _, replay, err := Begin(db, g); err != nil || !replay {
			t.Fatalf("attempt %d: a started guard let a second caller through (replay=%v err=%v)", i, replay, err)
		}
	}
}

// TestGuard_AnAbandonedGuardIsFree — the ONE way a key becomes available again:
// the operation itself establishes that nothing happened and releases it. That
// is what keeps a refusal from wedging a merchant, without a timer that would
// let an unknown outcome be retried.
func TestGuard_AnAbandonedGuardIsFree(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := nsDB(c, "acme")

	g := Guard{Scope: "payout:ba_1", Key: "k"}
	rec, replay, err := Begin(db, g)
	if err != nil || replay {
		t.Fatalf("first begin: err=%v replay=%v", err, replay)
	}
	if err := rec.Delete(); err != nil {
		t.Fatalf("abandon: %v", err)
	}

	again, replay, err := Begin(db, g)
	if err != nil {
		t.Fatalf("begin after abandon: %v", err)
	}
	if replay {
		t.Fatal("an abandoned guard stayed taken — the key is wedged for good")
	}
	if err := Complete(again, `{"ok":true}`); err != nil {
		t.Fatalf("complete after re-claim: %v", err)
	}
	got, replay, _ := Begin(db, g)
	if !replay || got.Response != `{"ok":true}` {
		t.Fatalf("post-reclaim replay: replay=%v resp=%q", replay, got.Response)
	}
}

// TestGuard_ADifferentRequestUnderOneKeyIsRefused — a key names ONE question.
// Answering a second question with the first one's answer is how a caller asks
// for a $10,000 payout and is told "created" with a $1 receipt.
func TestGuard_ADifferentRequestUnderOneKeyIsRefused(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	db := nsDB(c, "acme")

	small := Guard{Scope: "payout:ba_1", Key: "same", Digest: "amount=100"}
	large := Guard{Scope: "payout:ba_1", Key: "same", Digest: "amount=1000000"}

	first, replay, err := Begin(db, small)
	if err != nil || replay {
		t.Fatalf("first: err=%v replay=%v", err, replay)
	}
	if err := Complete(first, `{"amount":100}`); err != nil {
		t.Fatalf("complete: %v", err)
	}

	if _, _, err := Begin(db, large); !errors.Is(err, ErrConflict) {
		t.Fatalf("one key for a different request returned %v; want ErrConflict", err)
	}
	// The SAME request still replays.
	if _, replay, err := Begin(db, small); err != nil || !replay {
		t.Fatalf("the same request stopped replaying: err=%v replay=%v", err, replay)
	}
}

// TestGuard_ADigestIsNotPartOfTheKey — if it were, a retry of the same request
// would mint a NEW guard and the money would move twice, which is the exact
// opposite of what the digest is for.
func TestGuard_ADigestIsNotPartOfTheKey(t *testing.T) {
	plain := Guard{Scope: "s", Key: "k"}
	digested := Guard{Scope: "s", Key: "k", Digest: "d"}
	if plain.ID() != digested.ID() {
		t.Fatalf("the digest changed the guard's identity: %s != %s", plain.ID(), digested.ID())
	}
	if (Guard{Scope: "s", Key: "other"}).ID() == plain.ID() {
		t.Fatal("two different keys share one guard row")
	}
}
