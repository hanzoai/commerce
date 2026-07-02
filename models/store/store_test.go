package store

import "testing"

// A store that has never held a listing deserializes Listings to nil. The
// listing admin API (updateListing) writes with a direct map assignment, and
// AddListing mutates the map too — both panic on a nil map. Load must
// materialize an empty map so the FIRST listing added to any fresh store (e.g.
// seeding a per-org catalog) succeeds instead of 500ing.
func TestLoadMaterializesNilListings(t *testing.T) {
	s := &Store{} // no Listings_, Listings is nil
	if err := s.Load(nil); err != nil {
		t.Fatalf("Load(nil): %v", err)
	}
	if s.Listings == nil {
		t.Fatal("Load left Listings nil — direct map assignment would panic")
	}
	// The exact operation the updateListing handler performs.
	s.Listings["agency"] = Listing{Slug: "agency"}
	if len(s.Listings) != 1 {
		t.Fatalf("listing not stored: %+v", s.Listings)
	}
}

// AddListing must never panic on a zero-value store.
func TestAddListingInitializesMap(t *testing.T) {
	s := &Store{}
	s.AddListing("instant-site", Listing{Slug: "instant-site"})
	if got, ok := s.Listings["instant-site"]; !ok || got.Slug != "instant-site" {
		t.Fatalf("AddListing failed on nil map: %+v", s.Listings)
	}
}
