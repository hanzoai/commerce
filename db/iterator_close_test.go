package db

import (
	"path/filepath"
	"testing"
)

// TestIteratorCloseReleasesPooledConn is the regression guard for the connection
// leak that starved the co-resident commerce Postgres pool in production.
//
// A single-row read (datastore/query.Query.First → Get/ById/GetById →
// org.Resolve on every balance + per-tier gate read) runs a LIMIT 1 query and
// calls Next() exactly once. Because the iterator is never exhausted,
// database/sql never auto-closes the *sql.Rows, so the pooled connection stays
// checked out until ConnMaxLifetime. Under load this pins all MaxOpenConns
// connections and every subsequent query blocks in (*DB).conn until its context
// deadline — org.Resolve then times out and the per-tier gate fails open.
//
// The iterators now expose Close(); First() defers it. This test asserts Close()
// actually returns the connection to the pool (InUse drops back to 0).
func TestIteratorCloseReleasesPooledConn(t *testing.T) {
	sdb, err := NewSQLiteDB(&SQLiteDBConfig{
		Path:       filepath.Join(t.TempDir(), "data.db"),
		Config:     DefaultConfig().SQLite,
		TenantID:   "system",
		TenantType: "org",
		MasterKey:  nil, // plaintext open (PragmaDSN branch)
	})
	if err != nil {
		t.Fatalf("NewSQLiteDB: %v", err)
	}
	defer sdb.Close()

	if _, err := sdb.writeDB.Exec(`CREATE TABLE tt(id TEXT PRIMARY KEY, data TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := sdb.writeDB.Exec(`INSERT INTO tt(id, data) VALUES('1', '{}')`); err != nil {
		t.Fatalf("seed insert: %v", err)
	}

	rows, err := sdb.readDB.Query(`SELECT id, data FROM tt LIMIT 1`)
	if err != nil {
		t.Fatalf("read query: %v", err)
	}
	it := &sqliteIterator{rows: rows, kind: "tt"}

	// The First() pattern: read exactly one row, then stop (do NOT drain).
	var dst map[string]any
	if _, err := it.Next(&dst); err != nil {
		t.Fatalf("Next (single row): %v", err)
	}
	if inUse := sdb.readDB.Stats().InUse; inUse == 0 {
		t.Fatalf("precondition: expected the connection checked out while the cursor is open, got InUse=0")
	}

	// The fix: Close() must return the pooled connection.
	if err := it.Close(); err != nil {
		t.Fatalf("iterator Close: %v", err)
	}
	if inUse := sdb.readDB.Stats().InUse; inUse != 0 {
		t.Fatalf("connection leaked: after Close InUse=%d, want 0", inUse)
	}
}
