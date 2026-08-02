package idempotencykey_test

import (
	"context"
	"sync"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/postgres"
)

// These run against REAL Postgres, because both properties they assert are
// properties of the STORE and SQLite provides them for free — see the package
// doc on util/test/postgres.

// TestBegin_Idempotent_NoDoubleDebit_Postgres reproduces the LIVE double-charge
// on the production (Postgres) backend. It mirrors the money path of
// billing.RecordUsage — Begin the guard, and only when it is NOT a replay
// perform a withdraw + Complete — and asserts that two submits of the SAME
// (scope, key) perform AT MOST ONE withdraw.
//
// Each debit builds its OWN datastore (datastore.NewWithDB) exactly as each HTTP
// request does in production (datastore.New(org.Namespaced(c.Context()))), both sharing the
// one Postgres store. On the unfixed code the second Begin cannot find the guard
// its own first write created (the read decodes the deterministic id into a
// KIND-LESS key, which Postgres's kind-qualified Get never matches), so it
// returns replay=false and a SECOND withdraw is issued: two rows → DOUBLE CHARGE.
// After the fix the second Begin replays and no second withdraw is written.
//
// The sibling TestRecordUsage_Idempotent_NoDoubleDebit (SQLite, via ae) PASSES on
// the unfixed code — SQLite's kind-less Get fallback hides the bug — which is
// exactly why this Postgres-backed reproduction is required.
func TestBegin_Idempotent_NoDoubleDebit_Postgres(t *testing.T) {
	pg := postgres.New(t)

	const ns = "usage-idem-pg-org"
	ctx := nscontext.WithNamespace(context.Background(), ns)

	const subject = ns + "/alice@example.com"
	const scope = "billing-usage"
	const key = "req-pg-abc-123" // the requestId chat sends per spend

	moves := 0
	debitOnce := func() {
		d := datastore.NewWithDB(ctx, pg)
		rec, replay, err := idempotencykey.Begin(d, idempotencykey.Guard{Scope: scope, Key: key})
		if err != nil {
			t.Fatalf("Begin: %v", err)
		}
		if replay {
			if rec.Status != idempotencykey.StatusCompleted {
				t.Fatalf("replay but status=%q, want completed", rec.Status)
			}
			return
		}
		moves++
		trans := transaction.New(d)
		trans.Type = transaction.Withdraw
		trans.SourceId = subject
		trans.SourceKind = "iam-user"
		trans.Currency = currency.USD
		trans.Amount = currency.Cents(100) // $1.00 debit
		trans.Tags = "api-usage"
		if err := trans.Create(); err != nil {
			t.Fatalf("withdraw Create: %v", err)
		}
		if err := idempotencykey.Complete(rec, `{"type":"withdraw","amount":100}`); err != nil {
			t.Fatalf("Complete: %v", err)
		}
	}

	debitOnce() // first spend
	debitOnce() // retry with the SAME key — MUST replay, never re-debit

	if moves != 1 {
		t.Fatalf("DOUBLE-CHARGE: performed %d debits for one (scope,key); want 1 — "+
			"Begin failed to find the prior guard on the Postgres backend.", moves)
	}

	// Independent evidence straight from Postgres: exactly ONE withdraw row.
	verify := datastore.NewWithDB(ctx, pg)
	var withdraws []*transaction.Transaction
	if _, err := transaction.Query(verify).
		Filter("SourceId=", subject).
		Filter("Type=", string(transaction.Withdraw)).
		GetAll(&withdraws); err != nil {
		t.Fatalf("count withdraws: %v", err)
	}
	if len(withdraws) != 1 {
		t.Fatalf("Postgres holds %d withdraw rows for %s; want 1 (double charge persisted)",
			len(withdraws), subject)
	}
}

// TestBegin_ConcurrentClaimsHaveExactlyOneWinner_Postgres is the guard's whole
// reason to exist, asserted where it has to hold.
//
// N requests arrive at once under ONE key. A guard that READS and then WRITES
// lets every one of them observe "not started" and every one of them move
// money — the duplicate disbursement, wearing a lock. A CLAIM is one store
// statement, so the row's own lock picks a winner and the losers are told.
//
// SQLite cannot fail this test: it serialises every write behind one process
// mutex, so a read-then-write looks atomic there and the defect is invisible.
// Production is Postgres, and Postgres is where the claim has to be a claim.
func TestBegin_ConcurrentClaimsHaveExactlyOneWinner_Postgres(t *testing.T) {
	pg := postgres.New(t)
	ctx := nscontext.WithNamespace(context.Background(), "claim-race-org")

	const n = 12
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		winners int
		replays int
		fails   []error
	)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			// Its own datastore, exactly as its own request would have.
			_, replay, err := idempotencykey.Begin(
				datastore.NewWithDB(ctx, pg),
				idempotencykey.Guard{Scope: "billing-payout:acct_1", Key: "one-key"},
			)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err != nil:
				fails = append(fails, err)
			case replay:
				replays++
			default:
				winners++
			}
		}()
	}
	close(start)
	wg.Wait()

	for _, e := range fails {
		t.Errorf("Begin: %v", e)
	}
	if winners != 1 {
		t.Fatalf("DUPLICATE DISBURSEMENT: %d of %d concurrent callers were told they own "+
			"the key and would each have moved money; want exactly 1", winners, n)
	}
	if winners+replays != n {
		t.Fatalf("%d callers of %d got an answer", winners+replays, n)
	}
}

// TestBegin_AReleasedGuardCanBeClaimedAgain_Postgres pins the other half of the
// claim: an operation that establishes NOTHING happened releases its guard by
// deleting it, and the key must then be free. A claim that could not revive a
// released row would wedge a merchant's key forever on the first refusal.
func TestBegin_AReleasedGuardCanBeClaimedAgain_Postgres(t *testing.T) {
	pg := postgres.New(t)
	ctx := nscontext.WithNamespace(context.Background(), "claim-release-org")
	g := idempotencykey.Guard{Scope: "billing-payout:acct_1", Key: "k"}

	first, replay, err := idempotencykey.Begin(datastore.NewWithDB(ctx, pg), g)
	if err != nil || replay {
		t.Fatalf("first: replay=%v err=%v", replay, err)
	}
	if err := first.Delete(); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, replay, err = idempotencykey.Begin(datastore.NewWithDB(ctx, pg), g); err != nil || replay {
		t.Fatalf("a released key was not free again: replay=%v err=%v", replay, err)
	}
	// And it is a guard again: the next caller is refused.
	if _, replay, err = idempotencykey.Begin(datastore.NewWithDB(ctx, pg), g); err != nil || !replay {
		t.Fatalf("the re-claimed key does not guard: replay=%v err=%v", replay, err)
	}
}
