package db

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sqlitedrv "github.com/hanzoai/sqlite"
)

// ledgerRow is a minimal money-shaped entity used by the proof tests. Its
// exported fields are serialized (camelCase) into the _entities.data JSON, so the
// Memo marker appears verbatim in a PLAINTEXT file — and must NOT appear in an
// encrypted one.
type ledgerRow struct {
	Account string
	Amount  int64
	Memo    string
}

func (ledgerRow) Kind() string { return "ledger" }

func testMasterKey() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i*7 + 1)
	}
	return k
}

// newEncryptedTenant opens an org tenant store at dir/data.db encrypted under key.
func newEncryptedTenant(t *testing.T, path string, key []byte) *SQLiteDB {
	t.Helper()
	db, err := NewSQLiteDB(&SQLiteDBConfig{
		Path:               path,
		Config:             DefaultConfig().SQLite,
		EnableVectorSearch: false,
		TenantID:           "acme",
		TenantType:         "org",
		MasterKey:          key,
	})
	if err != nil {
		t.Fatalf("open encrypted tenant: %v", err)
	}
	return db
}

// TestTenantEncryptionProof is commerce's anti-silent-plaintext gate for the
// per-tenant money stores. Under a cgo+libsqlcipher build it writes a marker to a
// tenant DB opened with a master key and asserts:
//
//   - the on-disk file is real ciphertext (no "SQLite format 3" header, no
//     plaintext marker),
//   - reopening with the SAME master key reads the row back — the key is DERIVED,
//     so nothing had to be persisted beside the file for that to work,
//   - reopening with a WRONG master key fails closed.
//
// A cgo build WITHOUT libsqlcipher linked would silently write plaintext; this
// SKIPs there (the Docker build sets SQLITE_REQUIRE_CODEC=1 to turn the skip into
// a hard failure, so a mis-linked image can never ship).
func TestTenantEncryptionProof(t *testing.T) {
	const marker = "anti-plaintext-canary-commerce-7f3a"
	key := testMasterKey()

	if !sqlitedrv.EncryptionAvailable() {
		// Pure-Go backend cannot encrypt: opening a tenant with a master key MUST
		// fail rather than silently persist plaintext money data.
		dir := t.TempDir()
		if _, err := NewSQLiteDB(&SQLiteDBConfig{
			Path:       filepath.Join(dir, "data.db"),
			Config:     DefaultConfig().SQLite,
			TenantID:   "acme",
			TenantType: "org",
			MasterKey:  key,
		}); err == nil {
			t.Fatal("pure-Go backend accepted a master key — would ship plaintext")
		}
		return
	}

	if !sqlitedrv.CodecLinked() {
		if os.Getenv("SQLITE_REQUIRE_CODEC") == "1" {
			t.Fatal("SQLITE_REQUIRE_CODEC=1 but CodecLinked()=false: this build advertises encryption (cgo) yet did NOT link libsqlcipher — it would ship PLAINTEXT money data. Build with -tags \"libsqlite3 sqlite_fts5\" + CGO_CFLAGS=-DSQLITE_HAS_CODEC -DSQLITE_USE_URI=1 + CGO_LDFLAGS=-lsqlcipher")
		}
		t.Skip("cgo build without libsqlcipher linked; encryption assertions skipped (Docker build is the hard gate)")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")
	ctx := context.Background()

	// Write a marker row through the encrypted store, then close.
	db := newEncryptedTenant(t, dbPath, key)
	if !db.Encrypted() {
		t.Fatal("tenant reports Encrypted()=false with a master key set")
	}
	k := db.NewKey("ledger", "row-1", 0, nil)
	if _, err := db.Put(ctx, k, &ledgerRow{Account: "acme", Amount: 4242, Memo: marker}); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// The raw db file must be ciphertext.
	raw, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read raw db: %v", err)
	}
	if bytes.HasPrefix(raw, []byte("SQLite format 3\x00")) {
		t.Fatal("ENCRYPTION FAILURE: data.db has a plaintext SQLite header — libsqlcipher not linked")
	}
	if bytes.Contains(raw, []byte(marker)) {
		t.Fatal("ENCRYPTION FAILURE: plaintext marker found in data.db — money data is NOT encrypted at rest")
	}

	// Reopen with the correct master key: the row must read back.
	db2 := newEncryptedTenant(t, dbPath, key)
	var got ledgerRow
	if err := db2.Get(ctx, db2.NewKey("ledger", "row-1", 0, nil), &got); err != nil {
		t.Fatalf("reopen read: %v", err)
	}
	if got.Memo != marker || got.Amount != 4242 {
		t.Fatalf("reopen read = %+v, want Memo=%q Amount=4242", got, marker)
	}
	db2.Close()

	// Reopen with a WRONG master key: it derives a DIFFERENT file key, so SQLCipher
	// must reject it rather than open the file.
	wrong := testMasterKey()
	wrong[0] ^= 0xFF
	if _, err := NewSQLiteDB(&SQLiteDBConfig{
		Path:       dbPath,
		Config:     DefaultConfig().SQLite,
		TenantID:   "acme",
		TenantType: "org",
		MasterKey:  wrong,
	}); err == nil {
		t.Fatal("ENCRYPTION FAILURE: wrong master key opened the tenant — key not enforced")
	}
}

// TestTenantIsolation proves two tenants get independent keys: a file created for
// "acme" cannot be opened as "globex" even under the same master key, because the
// namespace is bound into the derivation. Runs only with the codec linked.
func TestTenantIsolation(t *testing.T) {
	if !sqlitedrv.EncryptionAvailable() || !sqlitedrv.CodecLinked() {
		t.Skip("codec not linked; isolation proof requires SQLCipher")
	}
	key := testMasterKey()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")

	// Create the file wrapped for org "acme".
	db := newEncryptedTenant(t, dbPath, key)
	db.Close()

	// Opening the SAME file+master but as tenant "globex" must fail: "globex"
	// derives a different file key, so SQLCipher rejects it.
	if _, err := NewSQLiteDB(&SQLiteDBConfig{
		Path:       dbPath,
		Config:     DefaultConfig().SQLite,
		TenantID:   "globex",
		TenantType: "org",
		MasterKey:  key,
	}); err == nil {
		t.Fatal("cross-tenant open succeeded — per-tenant key isolation broken")
	}
}

// TestResolveMasterKey covers the env parsing (pure Go; runs under any build).
func TestResolveMasterKey(t *testing.T) {
	t.Setenv(masterKeyEnv, "")
	k, err := ResolveMasterKey()
	if sqlitedrv.CodecLinked() {
		// Production (codec-linked): an unset key is fatal — never open a money store
		// plaintext. (Mirrors cek's fail-closed posture.)
		if err == nil {
			t.Fatalf("unset on a codec-linked build: got (%v,nil), want a fail-closed error", k)
		}
	} else {
		// pure-Go / codec-unlinked dev/CI: unset → unencrypted dev mode.
		if err != nil || k != nil {
			t.Fatalf("unset on a non-codec build: got (%v,%v), want (nil,nil)", k, err)
		}
	}

	// A valid key is accepted ONLY on a build that actually encrypts (cgo AND
	// libsqlcipher linked). On a pure-Go OR a codec-unlinked cgo build, ANY set key
	// must be refused — never silently write plaintext. Malformed hex only reaches
	// its specific error where encryption is real (the not-available / no-codec
	// error fires first otherwise).
	canEncrypt := sqlitedrv.EncryptionAvailable() && sqlitedrv.CodecLinked()
	if canEncrypt {
		t.Setenv(masterKeyEnv, "not-hex!!")
		if _, err := ResolveMasterKey(); err == nil {
			t.Fatal("malformed hex accepted")
		}
		t.Setenv(masterKeyEnv, "abcd")
		if _, err := ResolveMasterKey(); err == nil {
			t.Fatal("wrong-length key accepted")
		}
		t.Setenv(masterKeyEnv, "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
		k, err := ResolveMasterKey()
		if err != nil || len(k) != 32 {
			t.Fatalf("valid key: got (len=%d,%v), want (32,nil)", len(k), err)
		}
	} else {
		// No real encryption available: any set key must be refused.
		t.Setenv(masterKeyEnv, "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
		if _, err := ResolveMasterKey(); err == nil {
			t.Fatal("build without a linked codec accepted a master key")
		}
	}
}

// TestEncryptedUserTenantIsRefused pins the ONE thing this migration could not
// carry over: hanzoai/namespace has no database layout for a user (namespace.Key
// refuses KindUser), so a per-user store cannot be named — and therefore cannot
// be keyed — in the platform's shared at-rest scheme.
//
// The refusal is explicit and build-independent, which is the point: the
// alternative was inventing a user layout here, and a second convention for
// where a database lives is how two services end up disagreeing about which file
// an entity's data is in. Per-user stores remain supported UNENCRYPTED.
func TestEncryptedUserTenantIsRefused(t *testing.T) {
	dir := t.TempDir()
	_, err := NewSQLiteDB(&SQLiteDBConfig{
		Path:       filepath.Join(dir, "data.db"),
		Config:     DefaultConfig().SQLite,
		TenantID:   "bob",
		TenantType: "user",
		MasterKey:  testMasterKey(),
	})
	if err == nil {
		t.Fatal("an encrypted per-user store was opened; namespace cannot name one, so this must fail closed")
	}
	if !strings.Contains(err.Error(), "no database layout for a user") {
		t.Fatalf("refusal does not name the limitation: %v", err)
	}
}

// TestUnencryptedUserTenantStillWorks is the other half: the user tree is only
// unencryptABLE, not unusable. The dev/CI posture (no master key) never reaches
// the derivation, so both tenant types open exactly as before.
func TestUnencryptedUserTenantStillWorks(t *testing.T) {
	dir := t.TempDir()
	db, err := NewSQLiteDB(&SQLiteDBConfig{
		Path:       filepath.Join(dir, "data.db"),
		Config:     DefaultConfig().SQLite,
		TenantID:   "bob",
		TenantType: "user",
	})
	if err != nil {
		t.Fatalf("unencrypted per-user store must still open: %v", err)
	}
	defer db.Close()
	if db.Encrypted() {
		t.Fatal("a store opened with no master key reports Encrypted()=true")
	}
}
