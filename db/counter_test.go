package db

import (
	"context"
	"path/filepath"
	"sort"
	"sync"
	"testing"
)

// The named total behind exact sub-cent billing. Each Add must own a disjoint
// span of the running sum, or two usage rows would both debit the cent one of
// them completed.

func TestCounter_AddsAndReturnsTheNewTotal(t *testing.T) {
	sdb := seqDB(t)
	ctx := context.Background()
	want := int64(0)
	for _, d := range []int64{2500, 2500, 7000, -4000, 0} {
		want += d
		got, err := sdb.Add(ctx, "usage", d)
		if err != nil {
			t.Fatalf("Add(%d): %v", d, err)
		}
		if got != want {
			t.Fatalf("Add(%d) = %d, want %d", d, got, want)
		}
	}
	if got, _ := sdb.Add(ctx, "other", 1); got != 1 {
		t.Fatalf("a second name started from %d, want 0", got-1)
	}
}

// spansDisjoint checks that the totals handed back for equal-sized additions
// tile [0, n*delta] with no repeats: every caller got its own span.
func spansDisjoint(t *testing.T, totals []int64, delta int64) {
	t.Helper()
	sort.Slice(totals, func(i, j int) bool { return totals[i] < totals[j] })
	for i, v := range totals {
		if want := int64(i+1) * delta; v != want {
			t.Fatalf("total #%d = %d, want %d — two additions started from the same total", i, v, want)
		}
	}
}

func TestCounter_ConcurrentAddsOwnDisjointSpans(t *testing.T) {
	sdb := seqDB(t)
	ctx := context.Background()
	const goroutines, each, delta = 64, 40, int64(3)

	var mu sync.Mutex
	var totals []int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < each; i++ {
				v, err := sdb.Add(ctx, "usage", delta)
				if err != nil {
					t.Errorf("Add: %v", err)
					return
				}
				mu.Lock()
				totals = append(totals, v)
				mu.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()
	if !t.Failed() {
		spansDisjoint(t, totals, delta)
	}
}

// Two stores on one file are two replicas: no Go mutex is shared, so only the
// single statement keeps their spans apart.
func TestCounter_TwoStoresOnOneFileOwnDisjointSpans(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.db")
	open := func() *SQLiteDB {
		sdb, err := NewSQLiteDB(&SQLiteDBConfig{Path: path, Config: DefaultConfig().SQLite, TenantID: "acme", TenantType: "org"})
		if err != nil {
			t.Fatalf("NewSQLiteDB: %v", err)
		}
		t.Cleanup(func() { sdb.Close() })
		return sdb
	}
	replicas := []*SQLiteDB{open(), open()}
	ctx := context.Background()
	const each, delta = 60, int64(7)

	var mu sync.Mutex
	var totals []int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, sdb := range replicas {
		wg.Add(1)
		go func(sdb *SQLiteDB) {
			defer wg.Done()
			<-start
			for i := 0; i < each; i++ {
				v, err := sdb.Add(ctx, "usage", delta)
				if err != nil {
					t.Errorf("Add: %v", err)
					return
				}
				mu.Lock()
				totals = append(totals, v)
				mu.Unlock()
			}
		}(sdb)
	}
	close(start)
	wg.Wait()
	if !t.Failed() {
		spansDisjoint(t, totals, delta)
	}
}

func TestCounter_RefusesAnEmptyName(t *testing.T) {
	if _, err := seqDB(t).Add(context.Background(), "", 1); err == nil {
		t.Fatal("an empty counter name was accepted")
	}
}

// The production dialect, env-gated like the sequence tests.
func TestPostgresCounter_ConcurrentAddsOwnDisjointSpans(t *testing.T) {
	pdb := postgresSeqDB(t)
	ctx := context.Background()
	const goroutines, each, delta = 32, 40, int64(5)

	var mu sync.Mutex
	var totals []int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < each; i++ {
				v, err := pdb.Add(ctx, "usage", delta)
				if err != nil {
					t.Errorf("Add: %v", err)
					return
				}
				mu.Lock()
				totals = append(totals, v)
				mu.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()
	if !t.Failed() {
		spansDisjoint(t, totals, delta)
	}
}

var _ Counter = (*SQLiteDB)(nil)
var _ Counter = (*PostgresDB)(nil)
var _ Counter = tenantDB{}
