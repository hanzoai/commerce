package db

import (
	"context"
	"testing"
)

// A row read by query answers the key it was written under, on Postgres as on
// SQLite: an integer key reads back as that integer key — through GetAll, First,
// Keys and Run alike — and a string key as that string. Read as a string key,
// a row written by an integer key reported its decimal id where a row got by
// key reported its hashid, and a GetById of the decimal id missed the row.
//
// Env-gated as TestPostgresNextSequence is (postgresSeqDB).
func TestPostgresQueryRowsKeepTheirKey(t *testing.T) {
	pdb := postgresSeqDB(t)
	ctx := context.Background()
	if _, err := pdb.db.Exec(`TRUNCATE _entities`); err != nil {
		t.Fatalf("clearing the throwaway table: %v", err)
	}
	type row struct {
		Name string `json:"name"`
	}
	if _, err := pdb.Put(ctx, pdb.NewKey("thing", "", 1793644859, nil), &row{Name: "int"}); err != nil {
		t.Fatal(err)
	}
	if _, err := pdb.Put(ctx, pdb.NewKey("thing", "acme-1", 0, nil), &row{Name: "string"}); err != nil {
		t.Fatal(err)
	}
	want := func(how string, k Key) {
		t.Helper()
		switch {
		case k == nil:
			t.Fatalf("%s: no key", how)
		case k.StringID() == "" && k.IntID() == 1793644859:
		case k.IntID() == 0 && k.StringID() == "acme-1":
		default:
			t.Fatalf("%s: key int %d string %q, want the key the row was written under", how, k.IntID(), k.StringID())
		}
	}

	rows := make([]row, 0)
	keys, err := pdb.Query("thing").GetAll(ctx, &rows)
	if err != nil || len(keys) != 2 {
		t.Fatalf("GetAll: %d keys, %v", len(keys), err)
	}
	for _, k := range keys {
		want("GetAll", k)
	}
	var one row
	k, err := pdb.Query("thing").Filter("Name=", "int").First(ctx, &one)
	if err != nil || k.IntID() != 1793644859 {
		t.Fatalf("First: %v %v", k, err)
	}
	ks, err := pdb.Query("thing").Keys(ctx)
	if err != nil || len(ks) != 2 {
		t.Fatalf("Keys: %d, %v", len(ks), err)
	}
	for _, k := range ks {
		want("Keys", k)
	}
	it := pdb.Query("thing").Run(ctx)
	for range 2 {
		var r row
		k, err := it.Next(&r)
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		want("Run", k)
	}
	if _, err := it.Next(&row{}); err != ErrDone {
		t.Fatalf("past the last row: %v, want ErrDone", err)
	}
}
