package proxy

import "testing"

func TestParsePriceTable_VectorSpec(t *testing.T) {
	// Vector: writes (POST under /collections) cost 2c, other /collections
	// calls (reads/search) 1c, everything else free.
	tbl := ParsePriceTable("POST|/collections|2 ; *|/collections|1 ; default:0")

	cases := []struct {
		method, path string
		status       int
		want         int64
	}{
		{"POST", "/collections/docs/points", 200, 2},        // upsert
		{"POST", "/collections/docs/points/search", 200, 2}, // POST under /collections -> first rule
		{"GET", "/collections/docs", 200, 1},                // read
		{"PUT", "/collections/docs", 200, 1},                // create collection -> 2nd rule (not POST)
		{"GET", "/healthz", 200, 0},                         // unmatched -> default 0
		{"POST", "/collections/docs/points", 500, 0},        // error -> never charged
		{"POST", "/collections/docs/points", 402, 0},        // 4xx -> never charged
	}
	for _, c := range cases {
		if got := tbl.Price(c.method, c.path, c.status); got != c.want {
			t.Errorf("Price(%s %s %d) = %d, want %d", c.method, c.path, c.status, got, c.want)
		}
	}
}

func TestParsePriceTable_SearchSpec(t *testing.T) {
	// Search: a search query costs 1c, indexing documents costs 3c.
	tbl := ParsePriceTable("POST|/indexes/|3 ; POST|/multi-search|1 ; default:1")
	if got := tbl.Price("POST", "/indexes/movies/documents", 200); got != 3 {
		t.Errorf("index doc want 3, got %d", got)
	}
	if got := tbl.Price("POST", "/multi-search", 200); got != 1 {
		t.Errorf("multi-search want 1, got %d", got)
	}
	if got := tbl.Price("POST", "/indexes/movies/search", 200); got != 3 {
		t.Errorf("search under /indexes/ matches first rule (3), got %d", got)
	}
	if got := tbl.Price("GET", "/version", 200); got != 1 {
		t.Errorf("default want 1, got %d", got)
	}
}

func TestParsePriceTable_Malformed_IsSafe(t *testing.T) {
	// Garbage in -> empty table (Default 0), never panics, never over-bills.
	for _, spec := range []string{"", "   ", "nonsense", "POST|/x", "POST|/x|notanumber", "POST|/x|-5"} {
		tbl := ParsePriceTable(spec)
		if got := tbl.Price("POST", "/x", 200); got != 0 {
			t.Errorf("malformed %q must price 0, got %d", spec, got)
		}
	}
}

func TestRule_Matches_AnyMethodAnyPath(t *testing.T) {
	r := Rule{Cents: 5} // no methods, no prefix => matches everything
	if !r.matches("DELETE", "/anything") {
		t.Error("empty rule must match any method/path")
	}
}
