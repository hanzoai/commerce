// Copyright © 2026 Hanzo AI. MIT License.

package db_test

import (
	"context"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/util/test/postgres"
)

// transaction_pg_test.go pins the ONE property the money plane buys from this
// package: a transaction that says [db.IsolationSerializable] does not lose an
// update.
//
// It is DETERMINISTIC. A contention test that fires N goroutines and hopes for
// the bad interleaving is a coin toss dressed as a gate — it passes on a broken
// store most of the time, which is worse than no test. This one drives the two
// transactions through the exact interleaving that loses an update and asserts
// the store refuses it.

type counter struct {
	N int64 `json:"n"`
}

// TestRunInTransaction_SerializableWillNotLoseAnUpdate is the interleaving that
// breaches a ceiling:
//
//	A: read n=0 ............................. write n=1, commit
//	B: ........ read n=0, write n=1, commit
//
// Under READ COMMITTED both commit and the row says 1: B's increment is GONE,
// with no error told to anybody. That is exactly how a reserve's running total
// falls behind what it has actually withheld, and the merchant money the total
// no longer knows about is money a lift will never return.
//
// Under SERIALIZABLE the store must either refuse one of them — which the
// caller's retry budget then re-runs against the committed value — or land both.
// It may never silently land one.
func TestRunInTransaction_SerializableWillNotLoseAnUpdate(t *testing.T) {
	store := postgres.New(t)
	ctx := context.Background()
	key := store.NewKey("counter", "c1", 0, nil)

	if _, err := store.Put(ctx, key, &counter{N: 0}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// once bounds each transaction to a single attempt, so what this measures is
	// the STORE's isolation and not the caller's retry loop.
	once := &db.TransactionOptions{Isolation: db.IsolationSerializable, MaxAttempts: 1}

	readA := make(chan struct{})
	doneB := make(chan struct{})
	errs := make(chan error, 2)

	go func() {
		errs <- store.RunInTransaction(ctx, func(tx db.Transaction) error {
			var c counter
			if err := tx.Get(key, &c); err != nil {
				return err
			}
			close(readA) // A has read; let B run to completion
			<-doneB      // A writes only after B has committed
			c.N++
			_, err := tx.Put(key, &c)
			return err
		}, once)
	}()

	<-readA
	errs <- store.RunInTransaction(ctx, func(tx db.Transaction) error {
		var c counter
		if err := tx.Get(key, &c); err != nil {
			return err
		}
		c.N++
		_, err := tx.Put(key, &c)
		return err
	}, once)
	close(doneB)

	refused := 0
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			if !isSerializationFailure(err) {
				t.Fatalf("transaction failed for another reason: %v", err)
			}
			refused++
		}
	}

	var final counter
	if err := store.Get(ctx, key, &final); err != nil {
		t.Fatalf("read back: %v", err)
	}
	landed := int(final.N)
	if landed+refused != 2 {
		t.Fatalf("LOST UPDATE: %d increment(s) landed and %d transaction(s) were refused — "+
			"one increment vanished with nobody told", landed, refused)
	}
}

// TestRunInTransaction_ARefusedTransactionIsRetried — the other half of the
// contract. The store refuses a transaction it cannot serialize, and the caller
// states a budget for re-running it; without the retry every contended
// disbursement would surface as a 503 rather than as a slightly slower payout.
func TestRunInTransaction_ARefusedTransactionIsRetried(t *testing.T) {
	store := postgres.New(t)
	ctx := context.Background()
	key := store.NewKey("counter", "retry", 0, nil)
	if _, err := store.Put(ctx, key, &counter{N: 0}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	attempts := 0
	err := store.RunInTransaction(ctx, func(tx db.Transaction) error {
		attempts++
		var c counter
		if err := tx.Get(key, &c); err != nil {
			return err
		}
		if attempts < 3 {
			// The shape the store returns when two transactions cannot both have
			// happened. The retry predicate reads the SQLSTATE text because this
			// package speaks to more than one driver.
			return errSerialization
		}
		c.N++
		_, e := tx.Put(key, &c)
		return e
	}, &db.TransactionOptions{Isolation: db.IsolationSerializable, MaxAttempts: 5})
	if err != nil {
		t.Fatalf("the budgeted retry did not run: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("%d attempts, want 3 — the budget is not being spent", attempts)
	}

	var final counter
	if err := store.Get(ctx, key, &final); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if final.N != 1 {
		t.Fatalf("the successful attempt wrote %d times", final.N)
	}
}

// TestRunInTransaction_NoBudgetMeansOneAttempt — a caller that states no
// options gets exactly the single try it always got. A retry nobody asked for
// re-runs a side effect nobody expected to run twice.
func TestRunInTransaction_NoBudgetMeansOneAttempt(t *testing.T) {
	store := postgres.New(t)
	attempts := 0
	err := store.RunInTransaction(context.Background(), func(db.Transaction) error {
		attempts++
		return errSerialization
	}, nil)
	if err == nil {
		t.Fatal("a refused transaction reported success")
	}
	if attempts != 1 {
		t.Fatalf("%d attempts with no budget stated, want 1", attempts)
	}
}

// errSerialization is what a driver hands back when two transactions cannot
// both have happened; the text carries the SQLSTATE the predicate matches.
var errSerialization = &pgError{"pq: could not serialize access due to read/write dependencies among transactions (SQLSTATE 40001)"}

type pgError struct{ s string }

func (e *pgError) Error() string { return e.s }

// isSerializationFailure mirrors the predicate under test closely enough to
// tell "the store refused us" from "something else broke", without importing
// the unexported one.
func isSerializationFailure(err error) bool {
	s := err.Error()
	return strings.Contains(s, "40001") ||
		strings.Contains(s, "could not serialize") ||
		strings.Contains(s, "deadlock detected")
}
