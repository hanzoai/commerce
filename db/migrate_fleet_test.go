package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sqlitedrv "github.com/hanzoai/sqlite"
)

// TestEncryptDataDirContinuesPastAnUnconvertibleTenant pins the behaviour that
// makes a fleet backfill usable.
//
// EncryptDataDir used to `return` on the first tenant that failed. On the live
// estate — 67 stores, 66 of them plaintext — that meant one bad file left every
// tenant AFTER it in the directory walk untouched, and WHICH ones survived
// depended on os.ReadDir ordering. The operator got "an unknown prefix converted,
// unknown suffix not", which is the one outcome you cannot act on.
//
// Continuing is safe because encryptTenantFile is per-file and atomic-by-cutover:
// it renames only after per-table content parity passes, so a failed tenant keeps
// its plaintext source exactly as it was. A failure costs a retry, never data.
//
// The unconvertible tenant here is a data.db that is not a database at all. That
// is a real shape (a truncated or half-written file), and it fails inside the
// migration rather than at the directory walk, which is the path that matters.
func TestEncryptDataDirContinuesPastAnUnconvertibleTenant(t *testing.T) {
	if !sqlitedrv.EncryptionAvailable() || !sqlitedrv.CodecLinked() {
		t.Skip("codec not linked; migration proof requires SQLCipher")
	}
	key := testMasterKey()
	ctx := context.Background()
	dataDir := t.TempDir()

	// Three healthy orgs, named so the broken one sorts BETWEEN them: with the old
	// return-on-first-error the walk would convert "aaa" and never reach "zzz".
	healthy := []string{"aaa-org", "zzz-org"}
	for _, id := range healthy {
		p := filepath.Join(dataDir, "orgs", id, "data.db")
		s, err := NewSQLiteDB(&SQLiteDBConfig{
			Path: p, Config: DefaultConfig().SQLite, TenantID: id, TenantType: "org",
		})
		if err != nil {
			t.Fatalf("open plaintext %s: %v", id, err)
		}
		k := s.NewKey("ledger", "row-1", 0, nil)
		if _, err := s.Put(ctx, k, &ledgerRow{Account: id, Amount: 42, Memo: "keep-me"}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
		s.Close()
	}

	// The broken tenant: a data.db that is not a SQLite file.
	brokenID := "mmm-broken"
	brokenPath := filepath.Join(dataDir, "orgs", brokenID, "data.db")
	if err := os.MkdirAll(filepath.Dir(brokenPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brokenPath, []byte("this is not a database"), 0o600); err != nil {
		t.Fatal(err)
	}

	rep, err := EncryptDataDir(dataDir, key, false)
	if err != nil {
		t.Fatalf("EncryptDataDir returned a RUN-level error for a per-TENANT failure: %v", err)
	}
	if rep == nil {
		t.Fatal("nil report")
	}

	// Every healthy tenant converted, regardless of where the broken one sorted.
	converted := map[string]bool{}
	for _, p := range rep.Encrypted {
		converted[filepath.Base(filepath.Dir(p))] = true
	}
	for _, id := range healthy {
		if !converted[id] {
			t.Errorf("healthy tenant %q was NOT converted (encrypted=%v) — a single bad "+
				"tenant stranded the fleet again", id, rep.Encrypted)
		}
	}

	// The broken one is reported, by name, with its cause.
	if len(rep.Failed) != 1 {
		t.Fatalf("want exactly 1 failure, got %d: %+v", len(rep.Failed), rep.Failed)
	}
	if !strings.Contains(rep.Failed[0].Path, brokenID) {
		t.Errorf("failure names %q, want the broken tenant %q", rep.Failed[0].Path, brokenID)
	}
	if rep.Failed[0].Err == nil {
		t.Error("failure carries no cause — the operator cannot act on it")
	}

	// The verdict is non-nil and names the tenant, so a caller can exit non-zero
	// without losing the successes.
	verdict := rep.Err()
	if verdict == nil {
		t.Fatal("Err() is nil despite a recorded failure")
	}
	if !strings.Contains(verdict.Error(), brokenID) {
		t.Errorf("verdict %q does not name the failed tenant", verdict)
	}

	// The failed tenant's source is untouched — a retry is still possible.
	b, err := os.ReadFile(brokenPath)
	if err != nil {
		t.Fatalf("broken source disappeared: %v", err)
	}
	if string(b) != "this is not a database" {
		t.Errorf("broken source was modified by a failed migration: %q", b)
	}
}

// TestEncryptDataDirDryRunWritesNothing pins the flag an operator reaches for
// first on live money data.
func TestEncryptDataDirDryRunWritesNothing(t *testing.T) {
	if !sqlitedrv.EncryptionAvailable() || !sqlitedrv.CodecLinked() {
		t.Skip("codec not linked; migration proof requires SQLCipher")
	}
	key := testMasterKey()
	ctx := context.Background()
	dataDir := t.TempDir()

	id := "dry-org"
	p := filepath.Join(dataDir, "orgs", id, "data.db")
	s, err := NewSQLiteDB(&SQLiteDBConfig{
		Path: p, Config: DefaultConfig().SQLite, TenantID: id, TenantType: "org",
	})
	if err != nil {
		t.Fatalf("open plaintext: %v", err)
	}
	k := s.NewKey("ledger", "row-1", 0, nil)
	if _, err := s.Put(ctx, k, &ledgerRow{Account: id, Amount: 7, Memo: "dry"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	s.Close()

	before, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}

	rep, err := EncryptDataDir(dataDir, key, true)
	if err != nil {
		t.Fatalf("dry run errored: %v", err)
	}
	if len(rep.Encrypted) != 1 {
		t.Errorf("dry run should REPORT 1 conversion, got %d", len(rep.Encrypted))
	}
	if len(rep.Failed) != 0 {
		t.Errorf("dry run reported failures: %+v", rep.Failed)
	}

	after, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("dry run MODIFIED the source file")
	}
	if _, err := os.Stat(p + dekSuffix); err == nil {
		t.Error("dry run wrote a DEK sidecar")
	}
	if _, err := os.Stat(p + ".plaintext.bak"); err == nil {
		t.Error("dry run wrote a plaintext backup")
	}
	fmt.Fprintln(os.Stderr, "dry run clean")
}
