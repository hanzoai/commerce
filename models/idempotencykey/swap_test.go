package idempotencykey

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/db"
)

// Two processes over one store sight one key at once, many times each: exactly one
// Begin proceeds and every other replays, so the guarded money move runs once.
func TestAKeySightedAtOnceOnTwoReplicasRunsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guard.db")
	open := func() *datastore.Datastore {
		d, err := db.NewSQLiteDB(&db.SQLiteDBConfig{Path: path, Config: db.DefaultConfig().SQLite, TenantID: "acme", TenantType: "org"})
		if err != nil {
			t.Fatalf("open %s: %v", path, err)
		}
		t.Cleanup(func() { _ = d.Close() })
		return datastore.NewWithDB(context.Background(), d)
	}
	replicas := []*datastore.Datastore{open(), open()}

	var mu sync.Mutex
	proceeded := 0
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, replay, err := Begin(replicas[i%2], "billing-renew:sub_1", "period:1790000000")
			if err != nil {
				t.Errorf("begin: %v", err)
				return
			}
			if !replay {
				mu.Lock()
				proceeded++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if proceeded != 1 {
		t.Fatalf("%d first sightings proceeded, want exactly 1", proceeded)
	}
}
