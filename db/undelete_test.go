package db

import (
	"context"
	"testing"
)

// A Put on the key of a deleted entity stores it again, through every write
// path: a deterministic id (a lease, an idempotency record) is written, deleted
// and written again, and the last write is what a read finds.
func TestPutAfterDeleteStoresTheEntityAgain(t *testing.T) {
	d, err := testManager(t, 4).Org("undelete")
	if err != nil {
		t.Fatalf("open org: %v", err)
	}
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		put  func(Key, *tenantThing) error
	}{
		{"put", func(k Key, v *tenantThing) error { _, err := d.Put(ctx, k, v); return err }},
		{"put multi", func(k Key, v *tenantThing) error {
			_, err := d.PutMulti(ctx, []Key{k}, []*tenantThing{v})
			return err
		}},
		{"transaction", func(k Key, v *tenantThing) error {
			return d.RunInTransaction(ctx, func(tx Transaction) error { _, err := tx.Put(k, v); return err }, nil)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := d.NewKey("lease", "undelete-"+tc.name, 0, nil)
			if err := tc.put(key, &tenantThing{Name: "first"}); err != nil {
				t.Fatalf("first put: %v", err)
			}
			if err := d.Delete(ctx, key); err != nil {
				t.Fatalf("delete: %v", err)
			}
			if err := tc.put(key, &tenantThing{Name: "second"}); err != nil {
				t.Fatalf("second put: %v", err)
			}
			if got := get(t, d, "lease", "undelete-"+tc.name); got != "second" {
				t.Fatalf("read %q, want the second write", got)
			}
		})
	}
}
