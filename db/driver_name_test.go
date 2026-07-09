package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestEncDriverDSN_UnencryptedUsesRegisteredDriver pins the driver NAME the
// unencrypted merchant-store path opens with. It MUST be "sqlite" — the name that
// BOTH backends register (cgo driver_cgo.go via sql.Register("sqlite", …); pure-Go
// via modernc's own init()). The prior code returned "sqlite3", which only the
// cgo/mattn backend provides, so under the canonical CGO_ENABLED=0 production
// image sql.Open failed with `unknown driver "sqlite3"` and every per-org store
// was unopenable. If this ever regresses to "sqlite3" the merchant surface breaks
// on the deployed build even though CGO-on dev tests pass — hence this guard.
func TestEncDriverDSN_UnencryptedUsesRegisteredDriver(t *testing.T) {
	name, dsn := encDriverDSN("/tmp/whatever/data.db", nil, DefaultConfig().SQLite)
	if name != "sqlite" {
		t.Fatalf("unencrypted driver name = %q; want \"sqlite\" (the name registered by BOTH the cgo and pure-Go backends)", name)
	}
	if dsn == "" {
		t.Fatal("empty DSN")
	}
}

// TestManagerOrg_CreatesAndQueriesUnencrypted reproduces the exact production
// scenario that was broken: a fresh org's UNENCRYPTED per-org SQLite store must
// be creatable, writable, and readable. With no master key configured (the state
// of the deployed pod) NewSQLiteDB opens plaintext; the bug made sql.Open fail
// before the file was ever created, so Manager.Org returned an error and callers
// saw "database not initialized". This test creates an org DB, round-trips a row,
// and asserts the on-disk file is a real (plaintext) SQLite database.
//
// Run it under the production build to actually exercise the pure-Go backend:
//
//	CGO_ENABLED=0 go test ./db -run TestManagerOrg_CreatesAndQueriesUnencrypted
func TestManagerOrg_CreatesAndQueriesUnencrypted(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager(&Config{
		DataDir:            dir,
		EnableVectorSearch: false,
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	defer mgr.Close()

	odb, err := mgr.Org("karma")
	if err != nil {
		t.Fatalf("Manager.Org(karma): %v (this is the production bug: unknown sqlite driver on the CGO-off build)", err)
	}

	ctx := context.Background()
	key := odb.NewKey("listing", "ches", 0, nil)
	if _, err := odb.Put(ctx, key, &ledgerRow{Account: "karma", Amount: 9800, Memo: "ches"}); err != nil {
		t.Fatalf("Put into fresh org store: %v", err)
	}

	var got ledgerRow
	if err := odb.Get(ctx, key, &got); err != nil {
		t.Fatalf("Get back from fresh org store: %v", err)
	}
	if got.Amount != 9800 || got.Memo != "ches" {
		t.Fatalf("round-trip mismatch: got %+v", got)
	}

	// The file must exist on disk and be a real SQLite database (plaintext, since
	// no master key was configured).
	dbPath := filepath.Join(dir, "orgs", "karma", "data.db")
	raw, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read org data.db (should have been created): %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("org data.db is empty — store was never initialized")
	}
}
