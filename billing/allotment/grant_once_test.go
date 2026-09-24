package allotment

import (
	"sync"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A month's included credit must be granted ONCE, however many schedulers run.
//
// This package's doc claimed idempotence per (user, period), and Grant's call
// site claimed "the check-and-create runs inside a datastore transaction so
// concurrent schedulers cannot double-grant". Neither was true:
// datastore.RunInTransaction is a no-op stub — its body is literally "For now,
// just run the function directly" — and the write used a generated id, so two
// runs both read "not granted" and both created a row. The user got the monthly
// credit twice, with mintauth.WithAuthorized, i.e. real spendable balance.
//
// The fix is the same one that makes depositledger.creditKey safe: derive the
// storage id from (user, period, mode) so concurrent writers land on ONE row.
// These tests assert the MONEY — GrantedCents — rather than the number of calls
// that reported success, because two callers both believing they granted is
// fine as long as the balance moved once.
func TestGrantIsExactlyOnceUnderConcurrency(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	db := datastore.New(c)
	const user, plan, cents = "acme/alice", "pro", 2000
	at := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

	const racers = 24
	var wg sync.WaitGroup
	errs := make(chan error, racers)
	start := make(chan struct{})

	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // release them together, so the reads genuinely interleave
			if _, err := Grant(db, user, plan, cents, at, false); err != nil {
				errs <- err
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("Grant under concurrency: %v", err)
	}

	// The balance is the assertion. Before the deterministic key this returned a
	// multiple of `cents` — one row per racer that lost the read/write race.
	if got := GrantedCents(db, user, at, false); got != cents {
		t.Fatalf("granted %d cents after %d concurrent grants, want exactly %d — the month's credit was issued %.1f times",
			got, racers, cents, float64(got)/float64(cents))
	}
}

// Sequential re-runs are the scheduler's ordinary case (a retry, a manual
// re-trigger) and must also move the balance once.
func TestGrantRepeatedSequentiallyStillGrantsOnce(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	db := datastore.New(c)
	const user, plan, cents = "acme/bob", "pro", 1500
	at := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)

	first, err := Grant(db, user, plan, cents, at, false)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Granted {
		t.Fatalf("first grant did not report Granted: %+v", first)
	}
	for i := 0; i < 4; i++ {
		again, err := Grant(db, user, plan, cents, at, false)
		if err != nil {
			t.Fatal(err)
		}
		if again.Granted {
			t.Fatalf("re-run %d reported a fresh grant; want already_granted", i)
		}
	}
	if got := GrantedCents(db, user, at, false); got != cents {
		t.Fatalf("granted %d cents after 5 runs, want %d", got, cents)
	}
}

// Test-mode and live money are different money. They share a user and a month,
// so if `mode` were left out of the key they would collide on one row and one
// would silently overwrite the other.
func TestGrantKeepsTestAndLiveApart(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	db := datastore.New(c)
	const user, plan, cents = "acme/carol", "pro", 900
	at := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)

	if _, err := Grant(db, user, plan, cents, at, false); err != nil {
		t.Fatal(err)
	}
	if _, err := Grant(db, user, plan, cents, at, true); err != nil {
		t.Fatal(err)
	}
	if got := GrantedCents(db, user, at, false); got != cents {
		t.Errorf("live granted %d, want %d", got, cents)
	}
	if got := GrantedCents(db, user, at, true); got != cents {
		t.Errorf("test granted %d, want %d", got, cents)
	}
}

// Different users and different months are different grants — the key must
// separate them, or one tenant's credit would land on another's row.
func TestGrantKeySeparatesUsersAndPeriods(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	db := datastore.New(c)
	june := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	july := time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC)

	seen := map[string]bool{}
	for _, tc := range []struct {
		user string
		at   time.Time
		test bool
	}{
		{"acme/dave", june, false},
		{"acme/erin", june, false},
		{"acme/dave", july, false},
		{"acme/dave", june, true},
	} {
		k := grantKey(db, tc.user, Tag(tc.at), tc.test).Encode()
		if seen[k] {
			t.Fatalf("key collision for %s %s test=%v", tc.user, Tag(tc.at), tc.test)
		}
		seen[k] = true
	}
	if len(seen) != 4 {
		t.Fatalf("got %d distinct keys, want 4", len(seen))
	}
}
