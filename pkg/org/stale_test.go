package org

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/db"
)

// closedStore is a default datastore every query against fails: a store that
// cannot answer, as a reader queued behind a flooded writer cannot.
func closedStore(t *testing.T) db.DB {
	t.Helper()
	mgr, err := db.NewManager(&db.Config{DataDir: t.TempDir(), IsDev: true})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	shared, err := mgr.Org("system")
	if err != nil {
		t.Fatalf("mgr.Org(system): %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return shared
}

// An org that resolved is still served when the store fails a refresh, for
// maxStale after its last good resolve, and the failed store is asked again
// only every staleRetry. Past maxStale the failure is the caller's.
func TestResolve_ServesTheLastResolvedOrgWhileTheStoreFails(t *testing.T) {
	setup(t)
	clock := time.Now()
	now = func() time.Time { return clock }

	first, err := Resolve(context.Background(), "hanzo")
	if err != nil {
		t.Fatalf("first Resolve: %v", err)
	}

	datastore.SetDefaultDB(closedStore(t))
	clock = clock.Add(ttl + time.Second)

	got, err := Resolve(context.Background(), "hanzo")
	if err != nil {
		t.Fatalf("Resolve with a failing store inside maxStale: %v", err)
	}
	if got.Id() != first.Id() || got.Name != "hanzo" {
		t.Fatalf("served %q (%s), want the org resolved before (%s)", got.Name, got.Id(), first.Id())
	}

	// Inside staleRetry the cache answers: the failing store is not asked again.
	clock = clock.Add(staleRetry / 2)
	if _, err := Resolve(context.Background(), "hanzo"); err != nil {
		t.Fatalf("Resolve inside staleRetry: %v", err)
	}

	clock = clock.Add(maxStale)
	if _, err := Resolve(context.Background(), "hanzo"); !errors.Is(err, ErrResolveFailed) {
		t.Fatalf("Resolve past maxStale with a failing store = %v, want ErrResolveFailed", err)
	}
}

// An org never resolved has nothing to serve: a failing store is a failure.
func TestResolve_AFailingStoreWithNoResolvedOrgFails(t *testing.T) {
	setup(t)
	datastore.SetDefaultDB(closedStore(t))
	if _, err := Resolve(context.Background(), "neverseen"); !errors.Is(err, ErrResolveFailed) {
		t.Fatalf("Resolve of an unseen org on a failing store = %v, want ErrResolveFailed", err)
	}
}
