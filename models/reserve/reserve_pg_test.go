// Copyright © 2026 Hanzo AI. MIT License.

package reserve_test

import (
	"context"
	"sync"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/reserve"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/postgres"
)

// reserve_pg_test.go runs the ATOMICITY claims of this package against the
// store that has to provide them.
//
// SQLite cannot fail these tests. It holds one process-wide write mutex, so a
// running total updated by read-modify-write looks perfectly safe there and the
// lost update — the whole of SB-3 — is invisible. Production is Postgres.

// declare places one reserve on a fresh Postgres store and returns both.
func declare(t *testing.T, ns string, cap0 int64) (*datastore.Datastore, *control.Control) {
	t.Helper()
	pg := postgres.New(t)
	ds := datastore.NewWithDB(nscontext.WithNamespace(context.Background(), ns), pg)

	c := control.New(ds)
	c.Effect = control.Reserve
	c.SubjectKind = "merchant"
	c.Subject = "m1"
	c.Rate = control.FullRate
	c.Cap = cap0
	c.Currency = currency.USD
	if err := c.Create(); err != nil {
		t.Fatalf("declare: %v", err)
	}
	return ds, c
}

// cause is one judgement's movement.
func cause(screen string) reserve.Cause {
	return reserve.Cause{
		SubjectKind: "merchant", Subject: "m1",
		Currency: currency.USD, Screen: screen, Reference: "po_" + screen,
	}
}

// ledger is what the entries say is held net of what they returned. It must
// equal the account, because they are written in one act.
func ledger(ds *datastore.Datastore) (held, released int64) {
	for _, e := range reserve.For(ds, "merchant", "m1", 0) {
		held += e.Held
		released += e.Released
	}
	return held, released
}

// TestTake_ConcurrentDisbursementsNeverBreachTheCeiling_Postgres is SB-3 on the
// backend it was found on: N disbursements at once against a ceiling of C
// withhold C in total, not N×C, and the ledger explains every cent of it.
//
// A read-modify-write on the total loses increments here — the losers overwrite
// each other's writes, so the store believes the reserve is emptier than it is
// and keeps granting past the ceiling. Money is then withheld from merchants
// that the total does not know about and a lift will never return.
func TestTake_ConcurrentDisbursementsNeverBreachTheCeiling_Postgres(t *testing.T) {
	const (
		n    = 8
		cap0 = 1000
		want = 1000 / n * n // each move asks for its whole share
	)
	ds, c := declare(t, "reserve-race-org", cap0)

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		total int64
		fails []error
	)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			got, err := reserve.Take(ds, c, 1000, cause(string(rune('a'+i))))
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fails = append(fails, err)
				return
			}
			total += got
		}(i)
	}
	close(start)
	wg.Wait()

	for _, e := range fails {
		t.Errorf("take: %v", e)
	}
	if total > cap0 {
		t.Fatalf("BREACH: %d concurrent disbursements withheld %d against a declared ceiling of %d",
			n, total, cap0)
	}
	if total != cap0 {
		t.Fatalf("withheld %d of a %d ceiling — the reserve stopped short of its own declaration",
			total, cap0)
	}
	if got := reserve.Held(ds, c); got != total {
		t.Fatalf("the account says %d and the disbursements took %d", got, total)
	}
	if h, r := ledger(ds); h-r != total {
		t.Fatalf("the ledger nets %d and the account says %d — a lift returns one of them",
			h-r, total)
	}
	if got := reserve.Headroom(ds, c); got != 0 {
		t.Fatalf("headroom=%d at a full ceiling", got)
	}
}

// TestTake_IsIdempotentOnTheJudgement_Postgres — a retried disbursement under
// one judgement takes its share once, concurrently as well as sequentially.
// Retries under one key are the normal case on a money route, and two of them
// racing must not double the hold.
func TestTake_IsIdempotentOnTheJudgement_Postgres(t *testing.T) {
	ds, c := declare(t, "reserve-idem-org", 10000)

	const n = 8
	var wg sync.WaitGroup
	got := make([]int64, n)
	errs := make([]error, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			got[i], errs[i] = reserve.Take(ds, c, 400, cause("one-judgement"))
		}(i)
	}
	close(start)
	wg.Wait()

	for i, e := range errs {
		if e != nil {
			t.Fatalf("take %d: %v", i, e)
		}
		if got[i] != 400 {
			t.Fatalf("attempt %d took %d, want the one share of 400", i, got[i])
		}
	}
	if held := reserve.Held(ds, c); held != 400 {
		t.Fatalf("%d retries of one disbursement withheld %d", n, held)
	}
	if rows := reserve.For(ds, "merchant", "m1", 0); len(rows) != 1 {
		t.Fatalf("%d ledger entries for one move", len(rows))
	}
}

// TestClose_ReturnsExactlyWhatTheLedgerSays_Postgres — the invariant a
// merchant's money rests on. A lift returns the account, so the account and the
// ledger must be the same number; they are written in one transaction, and this
// asserts it after real contention rather than after a tidy sequence.
func TestClose_ReturnsExactlyWhatTheLedgerSays_Postgres(t *testing.T) {
	const cap0 = 900
	ds, c := declare(t, "reserve-close-org", cap0)

	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := reserve.Take(ds, c, 300, cause(string(rune('a'+i)))); err != nil {
				t.Errorf("take %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	held := reserve.Held(ds, c)
	returned, err := reserve.Close(ds, c)
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if returned != held || returned != cap0 {
		t.Fatalf("the lift returned %d of the %d the account held (ceiling %d)", returned, held, cap0)
	}
	h, r := ledger(ds)
	if h-r != 0 || reserve.Held(ds, c) != 0 {
		t.Fatalf("after the lift the account holds %d and the ledger nets %d",
			reserve.Held(ds, c), h-r)
	}
	// Twice returns it once.
	if again, err := reserve.Close(ds, c); err != nil || again != 0 {
		t.Fatalf("a retried lift returned %d again (err=%v)", again, err)
	}
}
