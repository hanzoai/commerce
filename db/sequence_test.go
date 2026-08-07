package db

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
)

// The named counter, which exists for exactly one reason: to hand two
// concurrent callers two DIFFERENT numbers.
//
// Nothing else in this store can. Put is a blind upsert, so a second writer
// overwrites instead of losing; datastore.RunInTransaction opens no transaction
// at all; and DB.RunInTransaction runs at the Postgres default isolation, where
// two transactions can both read N and both commit N+1. Every "unique" number
// above the storage layer is therefore a deterministic hash (which COLLAPSES
// duplicates rather than allocating) or a check-then-write with a TOCTOU
// window. This is the primitive that closes it, so these tests are about
// concurrency and not about arithmetic.

func seqDB(t *testing.T) *SQLiteDB {
	t.Helper()
	sdb, err := NewSQLiteDB(&SQLiteDBConfig{
		Path:       filepath.Join(t.TempDir(), "seq.db"),
		Config:     DefaultConfig().SQLite,
		TenantID:   "acme",
		TenantType: "org",
	})
	if err != nil {
		t.Fatalf("NewSQLiteDB: %v", err)
	}
	t.Cleanup(func() { sdb.Close() })
	return sdb
}

// The first value is 0, and it is a REAL value.
//
// This is pinned because the caller this was built for issues XRPL destination
// tags, where 0 is a legal tag somebody will hold. A sequence that started at 1
// "to keep 0 free" would be quietly encoding a meaning 0 does not have, and the
// first customer would be the one to find out.
func TestNextSequence_StartsAtZeroAndCounts(t *testing.T) {
	sdb := seqDB(t)
	ctx := context.Background()

	for want := uint64(0); want < 5; want++ {
		got, err := sdb.NextSequence(ctx, "tags")
		if err != nil {
			t.Fatalf("NextSequence: %v", err)
		}
		if got != want {
			t.Fatalf("allocation %d returned %d, want %d", want, got, want)
		}
	}
}

// Two names are two counters. A shared counter would be correct-but-wasteful
// here; the reason to pin it is the opposite failure, a name being ignored so
// that every caller in the process draws from one sequence.
func TestNextSequence_NamesAreIndependent(t *testing.T) {
	sdb := seqDB(t)
	ctx := context.Background()

	if _, err := sdb.NextSequence(ctx, "xrpl"); err != nil {
		t.Fatalf("NextSequence: %v", err)
	}
	if _, err := sdb.NextSequence(ctx, "xrpl"); err != nil {
		t.Fatalf("NextSequence: %v", err)
	}
	got, err := sdb.NextSequence(ctx, "other")
	if err != nil {
		t.Fatalf("NextSequence: %v", err)
	}
	if got != 0 {
		t.Fatalf("a second name started at %d, want 0 — the name is being ignored", got)
	}
}

// THE test. 64 goroutines allocating 50 apiece must produce 3200 DISTINCT
// values and nothing else.
//
// A check-then-write allocator passes a sequential test and fails this one,
// which is the entire point of writing it this way. The assertion is on the
// SET: every value distinct, and the set exactly [0, N) — so an implementation
// cannot pass by handing out unique-but-arbitrary numbers that skip or repeat
// under load.
func TestNextSequence_ConcurrentAllocationsAreAllDistinct(t *testing.T) {
	sdb := seqDB(t)
	ctx := context.Background()

	const goroutines, each = 64, 50
	got := make([][]uint64, goroutines)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			<-start // release them together, so they actually contend
			for i := 0; i < each; i++ {
				v, err := sdb.NextSequence(ctx, "tags")
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
				t.Fatalf("value %d was handed to goroutine %d AND goroutine %d — the allocator is not atomic", v, prev, g)
			}
			seen[v] = g
		}
	}
	if len(seen) != goroutines*each {
		t.Fatalf("got %d distinct values from %d allocations", len(seen), goroutines*each)
	}
	// Exactly [0, N): no gaps, no repeats, nothing invented.
	for want := uint64(0); want < uint64(goroutines*each); want++ {
		if _, ok := seen[want]; !ok {
			t.Fatalf("value %d was never handed out — the sequence skipped", want)
		}
	}
}

// THE REPLICA test: two independent stores on ONE file must not hand out the
// same number.
//
// The test above proves atomicity within a process, and on SQLite that is
// over-determined — writeMu alone would serialise it, so a read-modify-write
// implementation could pass. Two stores are two writeMus, which is exactly the
// shape of two replicas: nothing in Go is shared, and the ONLY thing standing
// between them is that the allocation is a single statement against one row.
//
// If this passes and the previous one passes, the guarantee is the statement's
// and not the mutex's — which is what the production claim rests on, because
// production is Postgres behind several pods and there is no mutex at all.
func TestNextSequence_TwoStoresOnOneFileNeverCollide(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.db")
	open := func() *SQLiteDB {
		sdb, err := NewSQLiteDB(&SQLiteDBConfig{
			Path:       path,
			Config:     DefaultConfig().SQLite,
			TenantID:   "acme",
			TenantType: "org",
		})
		if err != nil {
			t.Fatalf("NewSQLiteDB: %v", err)
		}
		t.Cleanup(func() { sdb.Close() })
		return sdb
	}
	replicas := []*SQLiteDB{open(), open()}
	ctx := context.Background()

	const each = 60
	got := make([][]uint64, len(replicas))
	var wg sync.WaitGroup
	start := make(chan struct{})
	for r, sdb := range replicas {
		wg.Add(1)
		go func(r int, sdb *SQLiteDB) {
			defer wg.Done()
			<-start
			for i := 0; i < each; i++ {
				v, err := sdb.NextSequence(ctx, "tags")
				if err != nil {
					t.Errorf("replica %d: NextSequence: %v", r, err)
					return
				}
				got[r] = append(got[r], v)
			}
		}(r, sdb)
	}
	close(start)
	wg.Wait()
	if t.Failed() {
		return
	}

	seen := make(map[uint64]int, len(replicas)*each)
	for r, vals := range got {
		for _, v := range vals {
			if prev, dup := seen[v]; dup {
				t.Fatalf("value %d was handed to replica %d AND replica %d — two replicas would issue one customer's routing tag to two customers", v, prev, r)
			}
			seen[v] = r
		}
	}
	if len(seen) != len(replicas)*each {
		t.Fatalf("got %d distinct values from %d allocations across %d replicas", len(seen), len(replicas)*each, len(replicas))
	}
}

// An unnamed counter is a caller bug, not a shared default. Answering one
// anyway would silently pool unrelated callers into one sequence.
func TestNextSequence_RefusesAnEmptyName(t *testing.T) {
	if _, err := seqDB(t).NextSequence(context.Background(), ""); err == nil {
		t.Fatal("an empty sequence name was accepted")
	}
}

// The store must satisfy the capability interface, since every caller reaches
// it by type assertion and a rename would otherwise turn into a silent refusal
// at runtime rather than a build failure.
var _ Sequencer = (*SQLiteDB)(nil)
var _ Sequencer = (*PostgresDB)(nil)
var _ Sequencer = tenantDB{}
