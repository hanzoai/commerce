package db

import (
	"context"
	"os"
	"sync"
	"testing"
)

// The named counter on the dialect PRODUCTION ACTUALLY RUNS.
//
// The SQLite tests prove the property on the dev/local store; this proves it
// where it matters, and the difference is not cosmetic. Postgres runs this pool
// at READ COMMITTED — the isolation DB.RunInTransaction opens and the one
// nobody in this codebase sets — where a read-modify-write counter is NOT safe:
// two transactions both read N and both commit N+1, with no conflict raised.
// The whole argument for the upsert is that it never reads a value into the
// client at all, and an argument about isolation levels deserves to be measured
// rather than asserted.
//
// Env-gated exactly like TestPostgresGuard, so CI never depends on a database.
// Point it at a THROWAWAY database; it creates and writes tables:
//
//	docker run -d --rm --name commerce-seq-pg -e POSTGRES_PASSWORD=x -p 15433:5432 postgres:16-alpine
//	COMMERCE_TEST_POSTGRES_DSN='postgres://postgres:x@127.0.0.1:15433/postgres?sslmode=disable' \
//	  go test ./db/ -run TestPostgresNextSequence -v -count=1
//
// ⚠ -count=1 matters: the sequence is durable, so a cached PASS would report a
// previous run's evidence for a table that now starts at a different value.
func postgresSeqDB(t *testing.T) *PostgresDB {
	t.Helper()
	dsn := os.Getenv("COMMERCE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("COMMERCE_TEST_POSTGRES_DSN not set")
	}
	pdb, err := NewPostgresDB(&PostgresDBConfig{
		DSN:      dsn,
		TenantID: "acme", TenantType: "org",
		// Real concurrency needs real connections. One connection would let the
		// pool serialise the very contention this is measuring.
		MaxOpenConns: 16, MaxIdleConns: 16,
	})
	if err != nil {
		t.Fatalf("NewPostgresDB: %v", err)
	}
	t.Cleanup(func() { pdb.Close() })
	// Re-runnable: the counter is durable, so a second run would otherwise
	// start where the first stopped and the "starts at 0" assertion would fail
	// for a reason that is not a defect.
	if _, err := pdb.db.Exec(`TRUNCATE _sequences`); err != nil {
		t.Fatalf("clearing the throwaway sequence table: %v", err)
	}
	return pdb
}

func TestPostgresNextSequence_StartsAtZeroAndCounts(t *testing.T) {
	pdb := postgresSeqDB(t)
	ctx := context.Background()

	for want := uint64(0); want < 5; want++ {
		got, err := pdb.NextSequence(ctx, "tags")
		if err != nil {
			t.Fatalf("NextSequence: %v", err)
		}
		if got != want {
			t.Fatalf("allocation %d returned %d, want %d", want, got, want)
		}
	}
}

// THE production-dialect test. Every goroutine here is on its own pooled
// connection, which is precisely what two pods look like to Postgres: it cannot
// tell, and does not care, whether two connections come from one process or
// several. So this measures the multi-replica claim directly.
func TestPostgresNextSequence_ConcurrentAllocationsAreAllDistinct(t *testing.T) {
	pdb := postgresSeqDB(t)
	ctx := context.Background()

	const goroutines, each = 32, 40
	got := make([][]uint64, goroutines)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			<-start
			for i := 0; i < each; i++ {
				v, err := pdb.NextSequence(ctx, "tags")
				if err != nil {
					t.Errorf("goroutine %d: NextSequence: %v", g, err)
					return
				}
				got[g] = append(got[g], v)
			}
		}(g)
	}
	close(start)
	wg.Wait()
	if t.Failed() {
		return
	}

	seen := make(map[uint64]int, goroutines*each)
	for g, vals := range got {
		for _, v := range vals {
			if prev, dup := seen[v]; dup {
				t.Fatalf("value %d was handed to goroutine %d AND goroutine %d — at READ COMMITTED two replicas would issue one routing tag twice", v, prev, g)
			}
			seen[v] = g
		}
	}
	for want := uint64(0); want < uint64(goroutines*each); want++ {
		if _, ok := seen[want]; !ok {
			t.Fatalf("value %d was never handed out — the sequence skipped under contention", want)
		}
	}
}
