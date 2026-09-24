package store

import "testing"

// The first listing upsert on a brand-new store 500'd with "assignment to entry in
// nil map": Listings is a map, `orm:"default:{}"` only applies on a DB round-trip, and
// a freshly constructed Store never had one. That made the ONE write that turns an
// empty storefront into a real one impossible.
func TestDefaultsMakesListingsWritable(t *testing.T) {
	var s Store
	s.Defaults()
	if s.Listings == nil {
		t.Fatal("Listings must be writable on a fresh store, not nil")
	}
	s.Listings["valentina"] = Listing{} // must not panic
	if len(s.Listings) != 1 {
		t.Fatalf("listing not stored: %d", len(s.Listings))
	}

	// Defaults must never discard listings a loaded store already has.
	existing := Store{Listings: Listings{"keep": Listing{}}}
	existing.Defaults()
	if _, ok := existing.Listings["keep"]; !ok {
		t.Fatal("Defaults() must never discard existing listings")
	}
}
