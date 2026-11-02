package db

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

// On Postgres, both ways round: a row stored the way Postgres stored rows before
// this codec (encoding/json, by json tags) reads through the store now, by key
// and by query; a row the store writes now reads the way an image from before
// read it (encoding/json); and a filter on a field whose tag is not its name
// (Code_, tagged "code") finds the row written now, as it does on SQLite.
//
// Env-gated as TestPostgresNextSequence is (postgresSeqDB).
func TestPostgresRowsReadBothWays(t *testing.T) {
	pdb := postgresSeqDB(t)
	ctx := context.Background()
	if _, err := pdb.db.Exec(`TRUNCATE _entities`); err != nil {
		t.Fatalf("clearing the throwaway table: %v", err)
	}
	in := sample()

	// Before: stored by encoding/json, read now.
	old, _ := json.Marshal(&in)
	if _, err := pdb.db.Exec(`INSERT INTO _entities (id, kind, tenant_id, data) VALUES ('7', 'row', $1, $2::jsonb)`,
		pdb.tenantFor(ctx), string(old)); err != nil {
		t.Fatal(err)
	}
	var got row
	if err := pdb.Get(ctx, pdb.NewKey("row", "", 7, nil), &got); err != nil {
		t.Fatal(err)
	}
	if got.Code_ != "50OFF" || got.IdemKey != "k-1" || got.Big != in.Big || len(got.Lines) != 1 || got.Limits == nil || got.Limits.MaxSeats_ != 2000 {
		t.Fatalf("an old row read by key: %+v", got)
	}
	var listed []row
	if _, err := pdb.Query("row").GetAll(ctx, &listed); err != nil || len(listed) != 1 || listed[0].Code_ != "50OFF" {
		t.Fatalf("an old row read by query: %+v %v", listed, err)
	}

	// Now: written by the store, read the way an image from before read it.
	if _, err := pdb.Put(ctx, pdb.NewKey("row", "", 8, nil), &in); err != nil {
		t.Fatal(err)
	}
	var data []byte
	if err := pdb.db.QueryRow(`SELECT data FROM _entities WHERE id = '8' AND kind = 'row'`).Scan(&data); err != nil {
		t.Fatal(err)
	}
	var before row
	if err := json.Unmarshal(data, &before); err != nil {
		t.Fatal(err)
	}
	want := in
	want.Raw_ = ""
	if !reflect.DeepEqual(before, want) {
		t.Fatalf("an image from before reads the new row as %+v\nwant %+v", before, want)
	}

	// And filtered by the Go name, as SQLite is: the row written now is found; a
	// false finds a row that leaves the field out.
	var found row
	k, err := pdb.Query("row").Filter("Code_=", "50OFF").Filter("IdemKey=", "k-1").Filter("Credited=", false).First(ctx, &found)
	if err != nil || k == nil || k.IntID() != 8 {
		t.Fatalf("a filter by Go name: %v %v", k, err)
	}
}
