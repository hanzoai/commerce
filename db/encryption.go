// At-rest encryption for the per-tenant SQLite stores.
//
// Commerce keeps each tenant's data (balances, transactions, usage — MONEY) in
// its own SQLite file. This file wires those files to the Hanzo encrypted SQLite
// driver (github.com/hanzoai/sqlite): SQLCipher page-level AES-256 at rest,
// keyed per-tenant, with the master key sourced from KMS.
//
// DERIVED-KEY MODEL (github.com/hanzoai/cek, the ONE at-rest scheme across the
// platform — cloud and IAM open their stores the same way):
//
//  1. A master key (32 bytes) is supplied via COMMERCE_KMS_MASTER_KEY or handed
//     over by an embedding host — sourced from KMS, never hardcoded, never in git.
//  2. A tenant's file key is DERIVED: HKDF(master, namespace + subsystem), via
//     cek.DeriveKey. It is a pure function of the tenant's name, so the file
//     reopens after a restart with nothing persisted beside it.
//  3. There is no per-file key material. No DEK is generated, wrapped, stored or
//     rotated in place — so there is no sidecar to lose, no unwrap step, and no
//     migration path to maintain. Losing the master loses the data, which is the
//     property at-rest encryption is for and the reason the master lives in KMS.
//
// Commerce derives rather than calling cek.Open because it opens a DUAL pool — a
// concurrent read pool and a serialized single-connection writer — against ONE
// file under ONE key, and cek.Open hands back a single *sql.DB. cek.DeriveKey is
// exported for exactly this: the key is the shared part, the pool is ours.
//
// Posture is decided once, in ResolveMasterKey():
//   - unset                 → unencrypted per-tenant files (dev / CI).
//   - set + live codec       → per-tenant SQLCipher encryption (production).
//   - set + no live codec    → hard error. Commerce's dual concurrent-read/
//     serialized-write pool needs the LIVE libsqlcipher codec; it cannot use the
//     single-writer codec envelope that a non-libsqlcipher build falls back to. So
//     a build without libsqlcipher is refused. (Such a build would still ENCRYPT via
//     the envelope — it does not write plaintext — but it cannot serve commerce's
//     dual pool, so opening it here would be wrong.)
package db

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hanzoai/cek"
	"github.com/hanzoai/namespace"
	sqlitedrv "github.com/hanzoai/sqlite"
)

const (
	// masterKeyEnv is the SINGLE canonical environment variable that supplies the
	// per-tenant encryption master key (32 bytes, 64 hex chars). It is materialised
	// into the pod from KMS (orgs/hanzo/secrets/commerce/COMMERCE_KMS_MASTER_KEY)
	// by the kms-operator — see universe/infra/k8s/commerce/master-key-kmssecret.yaml.
	masterKeyEnv = "COMMERCE_KMS_MASTER_KEY"

	// tenantSubsystem names what commerce's per-tenant store HOLDS. It is the
	// second half of every derived key and the file's basename, so one org's
	// commerce store can never share a key or a path with another subsystem's
	// store for the same org — cloud's treasury.db and this commerce.db sit side
	// by side under one namespace and are independently keyed.
	tenantSubsystem = "commerce"
)

// ResolveMasterKey reads COMMERCE_KMS_MASTER_KEY and validates it. It returns:
//
//   - (nil, nil)   when unset on a pure-Go dev/CI build → unencrypted dev mode.
//   - (key, nil)   when set, 64 hex chars, AND this build ACTUALLY encrypts
//     (cgo + libsqlcipher linked and proven at runtime).
//   - (nil, error) when set but malformed; OR set on a build that cannot really
//     encrypt; OR UNSET on a production (codec-linked) build — we refuse to run
//     rather than silently write plaintext money data.
//
// The codec check is the crux, and it gates on CodecLinked, NOT EncryptionAvailable.
// EncryptionAvailable() is always true now (every build encrypts — via the live
// libsqlcipher codec OR the pure-Go codec envelope), so it no longer distinguishes
// a commerce-capable build. Commerce needs the LIVE codec specifically: its dual
// read/write pool (encDriverDSN opens a concurrent read pool + a serialized write
// pool on the same file) cannot use the single-writer envelope. CodecLinked() runs a
// one-time runtime probe (open a keyed temp db, assert real ciphertext), so a build
// without the live codec is refused here rather than mis-opened. A refused build is
// NOT one that would write plaintext — it would encrypt via the envelope — it simply
// cannot serve commerce's dual pool.
//
// This is the ONE place the master key is sourced, so every tenant store shares an
// identical posture decision.
func ResolveMasterKey() ([]byte, error) {
	mkHex := os.Getenv(masterKeyEnv)
	if mkHex == "" {
		// No key configured. On a production (codec-linked) build this is a misconfig —
		// fail closed rather than silently write tenant money data as plaintext at rest
		// (mirrors cek's fail-closed posture). On a pure-Go dev/CI build — which cannot
		// link the codec and whose dual pool cannot use the envelope — unencrypted
		// per-tenant files are the intended zero-config dev path.
		if sqlitedrv.CodecLinked() {
			return nil, fmt.Errorf("%s is required on a production (libsqlcipher-linked) build; refusing to open tenant money stores unencrypted — inject the KMS master key, or run a pure-Go dev build", masterKeyEnv)
		}
		return nil, nil
	}
	if !sqlitedrv.CodecLinked() {
		return nil, fmt.Errorf("%s is set but the live libsqlcipher codec is not linked; commerce's dual concurrent-read/serialized-write pool cannot use the codec envelope — build the image CGO_ENABLED=1 -tags \"libsqlite3 sqlite_fts5\" + CGO_CFLAGS=\"-DSQLITE_HAS_CODEC -DSQLITE_USE_URI=1\" + CGO_LDFLAGS=-lsqlcipher, or unset %s for an unencrypted dev build", masterKeyEnv, masterKeyEnv)
	}
	mk, err := hex.DecodeString(strings.TrimSpace(mkHex))
	if err != nil {
		return nil, fmt.Errorf("%s must be hex-encoded: %w", masterKeyEnv, err)
	}
	if len(mk) != 32 {
		return nil, fmt.Errorf("%s must decode to 32 bytes, got %d", masterKeyEnv, len(mk))
	}
	return mk, nil
}

// tenantNamespace names the database one tenant's records live in. It is the ONE
// place a commerce tenant becomes a namespace, so the file's key and the file's
// path are two renderings of one name and cannot drift apart.
//
// A USER tenant has no namespace. namespace.Key deliberately refuses KindUser —
// the org layout has no place for a user and inventing one silently is how a
// second convention starts — so commerce cannot name a per-user database in the
// shared scheme. Encrypted user stores are therefore refused outright (see
// tenantKey); the unencrypted dev path never reaches here and is unaffected.
func tenantNamespace(tenantType, tenantID string) (namespace.Namespace, error) {
	if tenantType == tenantUser {
		return namespace.Namespace{}, fmt.Errorf(
			"cannot encrypt the per-user store for %q: hanzoai/namespace has no database layout for a user "+
				"(namespace.Key refuses KindUser), so a user store cannot be named — and therefore cannot be "+
				"keyed — in the platform's one at-rest scheme. Per-user stores are supported UNENCRYPTED only; "+
				"give namespace a user layout before turning a master key on for them", tenantID)
	}
	return namespace.OrgProject(tenantID, "")
}

// tenantKey returns the at-rest encryption key for one tenant's store: the
// master, bound to the namespace that owns the file and the subsystem it holds.
//
// It is a pure function of its inputs — nothing is minted, written or read from
// disk — so the read pool and the write pool key the same file identically, and
// a restart reopens it with no state carried alongside.
func tenantKey(masterKey []byte, tenantType, tenantID string) ([]byte, error) {
	// The namespace refusal comes FIRST because it is unconditional: a user store
	// is unnameable on every build, so it must fail the same way on every build.
	// The codec gate below is a property of how this binary was linked.
	ns, err := tenantNamespace(tenantType, tenantID)
	if err != nil {
		return nil, err
	}
	// Gate on CodecLinked (the LIVE codec), not EncryptionAvailable (always true
	// now). ResolveMasterKey already refuses the env-sourced key without the live
	// codec, but NewSQLiteDB also accepts a MasterKey injected directly on its
	// config (an embedding host, the encryption-proof test). Without the live codec
	// those paths reach sqlitedrv.DSN(path, key), which PANICS on a build routed to
	// the pure-Go backend (the key never rides the pure-Go DSN — the envelope keys
	// the file out of band). Return the clear refusal instead of crashing. Commerce
	// cannot use the envelope for its dual pool, so this is a genuine refusal — not
	// a "would write plaintext" one.
	if !sqlitedrv.CodecLinked() {
		return nil, fmt.Errorf("commerce requires the live libsqlcipher codec (its dual read/write pool cannot use the codec envelope); build CGO_ENABLED=1 -tags \"libsqlite3 sqlite_fts5\" linked against libsqlcipher, or open without a master key for an unencrypted dev build")
	}
	return cek.DeriveKey(masterKey, ns, tenantSubsystem)
}

// encDriverDSN returns the (driverName, dsn) to open dbPath. Both branches use
// the SINGLE canonical "sqlite" driver registered by github.com/hanzoai/sqlite —
// mattn/SQLCipher under CGO, pure-Go modernc under CGO_ENABLED=0. Commerce imports
// no mattn/modernc driver directly; hanzoai/sqlite is the one sqlite backend.
//
//   - dek != nil → encrypted at rest (SQLCipher). The key rides the canonical
//     driver DSN as SQLCipher's URI `key` param (applied inside sqlite3_open_v2,
//     before any pragma battery, so create AND reopen succeed); commerce's extra
//     tuning pragmas are appended. The DSN CONTAINS the key — callers MUST NOT log
//     it. Reached only under CGO (ResolveMasterKey fails closed on the pure-Go
//     build), so sqlitedrv.DSN's key path never panics.
//   - dek == nil → plaintext (dev/CI + the CGO-off production boot on modernc).
//     sqlitedrv.PragmaDSN renders plaintextPragmas in the ACTIVE backend's DSN
//     syntax (mattn `_busy_timeout=N`, modernc `_pragma=busy_timeout(N)`), so WAL +
//     busy_timeout actually apply under BOTH builds. It deliberately does NOT set
//     cache=shared: commerce opens a separate concurrent read pool and a serialized
//     write pool against the same file, and shared-cache table locking turns a
//     retryable SQLITE_BUSY into an un-retryable SQLITE_LOCKED ("database table is
//     locked") the instant the read pool holds a cursor open — exactly what the
//     cache=shared that sqlitedrv.DSN(dbPath, nil) appends did, panicking the test
//     suite's BeforeSuite fixture load.
func encDriverDSN(dbPath string, dek []byte, cfg SQLiteConfig) (string, string) {
	if dek != nil {
		dsn := sqlitedrv.DSN(dbPath, dek) // file:PATH?_busy_timeout=..&_journal_mode=WAL&..&key=x'..'
		if extra := extraEncPragmas(cfg); extra != "" {
			dsn += "&" + extra
		}
		return "sqlite", dsn
	}
	return "sqlite", sqlitedrv.PragmaDSN(dbPath, plaintextPragmas(cfg))
}

// plaintextPragmas maps commerce's SQLiteConfig to the connection pragmas applied
// to an UNENCRYPTED per-tenant store, rendered per-backend by sqlitedrv.PragmaDSN.
// It reproduces the historical buildPragmas floor — a 10s busy_timeout + WAL even
// for a zero-value config, so a second concurrent writer waits instead of failing
// with "database is locked" — and deliberately omits cache=shared (the dual
// read/write pool design requires private per-connection caches; see encDriverDSN).
func plaintextPragmas(cfg SQLiteConfig) []sqlitedrv.Pragma {
	busyTimeout := cfg.BusyTimeout
	if busyTimeout <= 0 {
		busyTimeout = 10000
	}
	journalMode := cfg.JournalMode
	if journalMode == "" {
		journalMode = "WAL"
	}
	// Order matters: busy_timeout leads so a connection blocks on a busy database
	// before journal_mode=WAL is set (WAL cannot be enabled while another
	// connection holds the database).
	pragmas := []sqlitedrv.Pragma{
		{Name: "busy_timeout", Value: strconv.Itoa(busyTimeout)},
		{Name: "journal_mode", Value: journalMode},
	}
	if cfg.Synchronous != "" {
		pragmas = append(pragmas, sqlitedrv.Pragma{Name: "synchronous", Value: cfg.Synchronous})
	}
	if cfg.CacheSize != 0 {
		pragmas = append(pragmas, sqlitedrv.Pragma{Name: "cache_size", Value: strconv.Itoa(cfg.CacheSize)})
	}
	pragmas = append(pragmas,
		sqlitedrv.Pragma{Name: "foreign_keys", Value: "ON"},
		sqlitedrv.Pragma{Name: "temp_store", Value: "MEMORY"},
	)
	return pragmas
}

// extraEncPragmas emits the tuning pragmas the driver's canonical DSN does NOT
// already set (it fixes busy_timeout=10000, WAL, synchronous=NORMAL,
// foreign_keys=ON — identical to commerce's DefaultConfig). Only cache_size and
// temp_store are commerce-specific, so only those are appended.
func extraEncPragmas(cfg SQLiteConfig) string {
	var p []string
	if cfg.CacheSize != 0 {
		p = append(p, fmt.Sprintf("_cache_size=%d", cfg.CacheSize))
	}
	p = append(p, "_temp_store=MEMORY")
	return strings.Join(p, "&")
}

// zeroBytes overwrites a key buffer in place.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
