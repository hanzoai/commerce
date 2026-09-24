// Package db provides a multi-layer database abstraction supporting:
// - User-level SQLite with sqlite-vec for personal data and vector search
// - Organization-level SQLite for shared tenant data
// - Hanzo Datastore (DATASTORE_URL) for deep analytics and parallel queries
//
// Architecture:
//
//	┌─────────────────────────────────────────────────────────────┐
//	│                      Query Layer                            │
//	├─────────────────────────────────────────────────────────────┤
//	│  User SQLite    │   Org SQLite    │    Hanzo Datastore      │
//	│  (per-user)     │   (per-org)     │    (DATASTORE_URL)      │
//	│  + sqlite-vec   │   + sqlite-vec  │    (parallel queries)   │
//	│  Fast queries   │   Shared data   │    Deep analytics       │
//	└─────────────────────────────────────────────────────────────┘
package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hanzoai/namespace"
	ormdb "github.com/hanzoai/orm/db"
)

// Tenant types. They name the two keyspaces the registry keeps apart — a user
// and an org may share an id and are still different tenants — and they are the
// TenantType every store reports.
const (
	tenantUser = "user"
	tenantOrg  = "org"
)

const (
	// defaultMaxOpenTenants bounds how many tenant stores stay open. The bound
	// is what makes this safe on a node serving many tenants: without one, an
	// open-per-tenant cache is a file descriptor leak with extra steps.
	defaultMaxOpenTenants = 256

	// defaultTenantIdleTTL closes stores nobody has touched for this long, so a
	// node that goes quiet hands its descriptors back instead of holding the
	// high-water mark until restart.
	defaultTenantIdleTTL = 5 * time.Minute
)

var (
	// ErrNoSuchEntity is returned when an entity is not found
	ErrNoSuchEntity = errors.New("db: no such entity")

	// ErrInvalidKey is returned when a key is invalid
	ErrInvalidKey = errors.New("db: invalid key")

	// ErrInvalidEntityType is returned when an entity type is invalid
	ErrInvalidEntityType = errors.New("db: invalid entity type")

	// ErrConcurrentModification is returned when optimistic locking fails
	ErrConcurrentModification = errors.New("db: concurrent modification")

	// ErrDatabaseClosed is returned when operating on a closed database
	ErrDatabaseClosed = errors.New("db: database closed")
)

// Layer represents which database layer to use
type Layer int

const (
	// LayerUser uses the user-specific SQLite database
	LayerUser Layer = iota

	// LayerOrg uses the organization-level SQLite database
	LayerOrg

	// LayerDatastore uses the Hanzo Datastore (Datastore) for analytics
	LayerDatastore

	// LayerAll queries all layers (for cross-cutting queries)
	LayerAll
)

// Config holds database configuration options
type Config struct {
	// DataDir is the base directory for data storage
	DataDir string

	// UserDataDir is the directory for per-user SQLite databases.
	// Defaults to DataDir/users.
	//
	// There is no OrgDataDir counterpart: an org's file is placed by
	// hanzoai/namespace under DataDir (namespace.Path → DataDir/orgs/<slug>/…),
	// which is the ONE layout every Hanzo service renders. This field survives
	// only because namespace has no layout for a user, so the per-user tree stays
	// commerce's own — see tenantNamespace in encryption.go.
	UserDataDir string

	// MasterKey is the 32-byte at-rest encryption master key, supplied by the
	// EMBEDDER. When non-nil it is used verbatim and COMMERCE_KMS_MASTER_KEY is
	// never read.
	//
	// It exists so a host that already holds a master key does not have to mint a
	// SECOND one and hand it over through the environment. cloud embeds commerce
	// in its own process and already resolves a 32-byte KEK
	// (CLOUD_KMS_MASTER_KEY_REF); making commerce reach independently for an env
	// var meant two keys, two KMS paths and two sync objects for one process —
	// and when the second was never provisioned, commerce refused to boot, Mount
	// never published its handler, and every balance read in the fleet failed.
	// The embedder passing what it already has is one key, one source, and a
	// dependency the type system can see.
	//
	// nil ⇒ unchanged: ResolveMasterKey reads the env, which remains the
	// standalone deployment's path.
	MasterKey []byte

	// MaxOpenTenants bounds how many per-tenant stores are open at once; the
	// least recently used idle one is closed to stay under it.
	// Defaults to defaultMaxOpenTenants.
	MaxOpenTenants int

	// TenantIdleTTL closes a per-tenant store untouched for this long, even
	// below MaxOpenTenants. Defaults to defaultTenantIdleTTL; negative disables.
	TenantIdleTTL time.Duration

	// DatastoreDSN is the connection string for Hanzo Datastore (DATASTORE_URL)
	DatastoreDSN string

	// EnableDatastore enables the Hanzo Datastore layer
	EnableDatastore bool

	// EnableVectorSearch enables sqlite-vec for vector embeddings
	EnableVectorSearch bool

	// VectorDimensions is the default dimension for vector embeddings
	VectorDimensions int

	// SQLite configuration
	SQLite SQLiteConfig

	// Datastore configuration (Hanzo Datastore)
	Datastore DatastoreConfig

	// IsDev enables development mode logging
	IsDev bool
}

// SQLiteConfig holds SQLite-specific configuration
type SQLiteConfig struct {
	// MaxOpenConns for concurrent reads
	MaxOpenConns int

	// MaxIdleConns for connection pooling
	MaxIdleConns int

	// BusyTimeout in milliseconds before giving up on locked DB
	BusyTimeout int

	// JournalMode (WAL recommended for concurrency)
	JournalMode string

	// Synchronous mode (NORMAL for balance of safety/speed)
	Synchronous string

	// CacheSize in KB (negative = KB, positive = pages)
	CacheSize int

	// QueryTimeout for SELECT queries
	QueryTimeout time.Duration
}

// DatastoreConfig holds Hanzo Datastore (Datastore) configuration
type DatastoreConfig struct {
	// MaxOpenConns for parallel queries
	MaxOpenConns int

	// MaxIdleConns for connection pooling
	MaxIdleConns int

	// ConnMaxLifetime for connection recycling
	ConnMaxLifetime time.Duration

	// Compression method (lz4, zstd, etc.)
	Compression string

	// QueryTimeout for datastore queries
	QueryTimeout time.Duration
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		DataDir:            "./data",
		EnableDatastore:    false,
		EnableVectorSearch: true,
		VectorDimensions:   1536, // OpenAI ada-002 dimensions
		SQLite: SQLiteConfig{
			MaxOpenConns: 120,
			MaxIdleConns: 15,
			BusyTimeout:  10000, // 10 seconds
			JournalMode:  "WAL",
			Synchronous:  "NORMAL",
			CacheSize:    -16000, // 16MB
			QueryTimeout: 30 * time.Second,
		},
		Datastore: DatastoreConfig{
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: time.Hour,
			Compression:     "lz4",
			QueryTimeout:    60 * time.Second,
		},
		IsDev: false,
	}
}

// Manager is the main entry point for database operations.
// It manages multiple database layers and provides unified access.
type Manager struct {
	config *Config

	// Per-tenant SQLite stores. The lifecycle — open on demand, bound, evict
	// the coldest idle handle, and the OnOpen/OnClose seam WAL replication
	// attaches to — lives in orm, because it is not commerce's problem and
	// commerce's private version of it (a userDBs and an orgDBs map, closed
	// only by Manager.Close) grew a file descriptor per tenant ever touched
	// and never gave one back.
	tenants *ormdb.Namespaces[DB]

	// Hanzo Datastore (shared)
	datastoreDB Datastore

	// masterKey is the 32-byte KMS-sourced at-rest encryption master key
	// (COMMERCE_KMS_MASTER_KEY). Nil => per-tenant SQLite files are unencrypted
	// (dev/CI). Resolved once in NewManager and passed to every tenant store.
	masterKey []byte
}

// NewManager creates a new database manager
func NewManager(cfg *Config) (*Manager, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	if cfg.UserDataDir == "" {
		cfg.UserDataDir = cfg.DataDir + "/users"
	}
	if cfg.MaxOpenTenants <= 0 {
		cfg.MaxOpenTenants = defaultMaxOpenTenants
	}
	if cfg.TenantIdleTTL == 0 {
		cfg.TenantIdleTTL = defaultTenantIdleTTL
	}

	// Decide at-rest encryption posture ONCE, from COMMERCE_KMS_MASTER_KEY. On a
	// non-encrypting (pure-Go) build this hard-errors when the key is set rather
	// than silently persisting plaintext money data.
	// An embedder-supplied key wins and skips the env entirely; otherwise the
	// posture is decided from COMMERCE_KMS_MASTER_KEY exactly as before.
	var err error
	masterKey := cfg.MasterKey
	if masterKey == nil {
		if masterKey, err = ResolveMasterKey(); err != nil {
			return nil, err
		}
	} else if len(masterKey) != 32 {
		return nil, fmt.Errorf("commerce: embedder MasterKey must be 32 bytes, got %d", len(masterKey))
	}

	m := &Manager{
		config:    cfg,
		masterKey: masterKey,
	}

	m.tenants, err = ormdb.NewNamespaces(ormdb.NamespacesConfig[DB]{
		Dir:     cfg.DataDir,
		MaxOpen: cfg.MaxOpenTenants,
		IdleTTL: cfg.TenantIdleTTL,
		PathFor: m.tenantPath,
		Open:    m.openTenant,
	})
	if err != nil {
		return nil, err
	}

	// Initialize Hanzo Datastore if enabled
	if cfg.EnableDatastore && cfg.DatastoreDSN != "" {
		datastore, err := NewDatastore(cfg)
		if err != nil {
			return nil, err
		}
		m.datastoreDB = datastore
	}

	return m, nil
}

// tenantPath places a tenant's file.
//
// An ORG's file is placed by hanzoai/namespace — DataDir/orgs/<slug>/commerce.db
// — so the path and the at-rest key are two renderings of ONE name (see
// tenantNamespace) and cannot drift apart. It is also the layout every other
// Hanzo service writes, so an org's commerce store sits beside its other stores
// under one directory instead of in a tree only commerce knows about.
//
// A USER's file stays commerce's own <UserDataDir>/<id>/data.db, because
// namespace has no layout for a user and inventing one here would be a second
// convention. That half is unencryptable for the same reason.
//
// The registry's single Dir is ignored, which means opting out of its
// containment check — so the id is validated here instead. A PathFor that leaves
// the registry's tree owns the guarantee the registry can no longer make.
func (m *Manager) tenantPath(_ string, n ormdb.Namespace) (string, error) {
	typ, id, ok := splitTenant(n)
	if !ok || !isSafeTenantID(id) {
		return "", fmt.Errorf("db: unsafe namespace %q", n)
	}
	if typ == tenantUser {
		return m.config.UserDataDir + "/" + id + "/data.db", nil
	}
	ns, err := tenantNamespace(typ, id)
	if err != nil {
		return "", err
	}
	return namespace.Path(m.config.DataDir, ns, tenantSubsystem)
}

// splitTenant reads a namespace back as the (type, id) pair commerce stores
// under. A namespace is opaque to orm — one string, one file — so the pair
// lives here, where the two roots and the SQLite tenant columns need it.
func splitTenant(n ormdb.Namespace) (typ, id string, ok bool) {
	typ, id, ok = strings.Cut(string(n), "/")
	return typ, id, ok && typ != "" && id != ""
}

// openTenant opens one tenant's store. The registry decides WHEN a store is
// open; this decides WHAT one is — same pragmas, same vector settings, same
// at-rest key for every tenant.
func (m *Manager) openTenant(n ormdb.Namespace, path string) (DB, error) {
	typ, id, ok := splitTenant(n)
	if !ok {
		return nil, fmt.Errorf("db: unsafe namespace %q", n)
	}
	return NewSQLiteDB(&SQLiteDBConfig{
		Path:               path,
		Config:             m.config.SQLite,
		EnableVectorSearch: m.config.EnableVectorSearch,
		VectorDimensions:   m.config.VectorDimensions,
		TenantID:           id,
		TenantType:         typ,
		MasterKey:          m.masterKey,
	})
}

// isSafeTenantID reports whether id is safe to use as a single filesystem path
// segment for a per-tenant SQLite store. The tenant id is the org (or user)
// namespace, which derives from a request-supplied — gateway/JWT-verified —
// owner/subject value, so it must never be able to escape the data dir (e.g.
// "..", "a/b", "/etc/passwd"). Reject anything that is not a clean single
// segment; callers fail closed on the returned error.
func isSafeTenantID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	if strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return false
	}
	// Leading-dot names (dotfiles / hidden dirs) are rejected — an org "."-prefixed
	// name shouldn't be able to create a hidden per-tenant dir (Red LOW-1). This is
	// pure path-safety, so it also applies to the app's own Org() calls (which
	// never use dot-names). The RESERVED-namespace policy (system/admin/default) is
	// intentionally NOT here: this function guards BOTH untrusted tenant requests
	// AND the app's legitimate Manager.Org("system") systemDB-fallback + test
	// harness, so rejecting "system" here would break boot. That policy lives at
	// the tenant boundary — datastore.NewNamespaced.
	if strings.HasPrefix(id, ".") {
		return false
	}
	return true
}

// User returns the database for a specific user
func (m *Manager) User(userID string) (DB, error) { return m.tenant(tenantUser, userID) }

// Org returns the database for a specific organization
func (m *Manager) Org(orgID string) (DB, error) { return m.tenant(tenantOrg, orgID) }

// tenant returns a DB view of one tenant's store.
//
// The returned value pins nothing (see tenantDB): it borrows the tenant's
// handle per operation, so callers may hold it for as long as they like without
// keeping a file descriptor. The store is opened here rather than on first use
// so this still reports an unopenable tenant as an error — the namespaced
// datastore resolver depends on that to fail closed rather than route a
// merchant's rows somewhere shared.
func (m *Manager) tenant(typ, id string) (DB, error) {
	if !isSafeTenantID(id) {
		return nil, fmt.Errorf("db: unsafe %s id %q", typ, id)
	}
	t := ormdb.Namespace(typ + "/" + id)
	if err := m.tenants.With(context.Background(), t, func(DB) error { return nil }); err != nil {
		if errors.Is(err, ormdb.ErrClosed) {
			return nil, ErrDatabaseClosed
		}
		return nil, err
	}
	return newTenantDB(m.tenants, t), nil
}

// Datastore returns the Hanzo Datastore for deep analytics queries
func (m *Manager) Datastore() Datastore {
	return m.datastoreDB
}

// Encrypted reports whether per-tenant SQLite files are SQLCipher-encrypted at
// rest (i.e. COMMERCE_KMS_MASTER_KEY was supplied and this build can encrypt).
func (m *Manager) Encrypted() bool {
	return m.masterKey != nil
}

// Close closes all database connections
func (m *Manager) Close() error {
	var lastErr error

	// Close every open tenant store. Further User/Org calls report
	// ErrDatabaseClosed.
	if err := m.tenants.Close(); err != nil {
		lastErr = err
	}

	// Close Hanzo Datastore
	if m.datastoreDB != nil {
		if err := m.datastoreDB.Close(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// DB is the main database interface for user/org SQLite databases
type DB interface {
	// Core operations
	Get(ctx context.Context, key Key, dst interface{}) error
	Put(ctx context.Context, key Key, src interface{}) (Key, error)
	Delete(ctx context.Context, key Key) error

	// Batch operations
	GetMulti(ctx context.Context, keys []Key, dst interface{}) error
	PutMulti(ctx context.Context, keys []Key, src interface{}) ([]Key, error)
	DeleteMulti(ctx context.Context, keys []Key) error

	// Query
	Query(kind string) Query

	// Vector search (sqlite-vec)
	VectorSearch(ctx context.Context, opts *VectorSearchOptions) ([]VectorResult, error)
	PutVector(ctx context.Context, kind string, id string, vector []float32, metadata map[string]interface{}) error

	// Key management
	NewKey(kind string, stringID string, intID int64, parent Key) Key
	NewIncompleteKey(kind string, parent Key) Key
	AllocateIDs(kind string, parent Key, n int) ([]Key, error)

	// Transactions
	RunInTransaction(ctx context.Context, fn func(tx Transaction) error, opts *TransactionOptions) error

	// Lifecycle
	Close() error

	// Tenant info
	TenantID() string
	TenantType() string
}

// Sequencer is a backend that can hand out a durable, strictly increasing
// number under a name — ATOMICALLY, so that two concurrent callers, in two
// goroutines or in two replicas, can never receive the same one.
//
// It exists because nothing else in this store can allocate a unique number.
// Put is a blind upsert (ON CONFLICT … DO UPDATE), so a second writer
// OVERWRITES rather than being refused; datastore.RunInTransaction is a no-op
// that opens no transaction at all; and DB.RunInTransaction, while real, runs
// at the Postgres default isolation (READ COMMITTED), where read-modify-write
// on a counter row lets two transactions both read N and both commit N+1. So
// every "unique" identifier in this codebase above the storage layer is either
// a deterministic hash (which makes duplicates COLLAPSE onto one row rather
// than allocate) or a check-then-write with a TOCTOU window.
//
// That is fine for idempotent dedup, where collapsing is the goal. It is not
// fine for allocation, where two callers must walk away with two DIFFERENT
// numbers. Where the number identifies whose money is whose — an XRPL
// destination tag on a pooled custody account — a check-then-write is not a
// guarantee, and this is the primitive that makes it one.
//
// It is deliberately NOT part of the DB interface. Allocation is a capability,
// not a requirement: a backend that cannot do it atomically must fail to
// satisfy this rather than offer a racy imitation, so callers type-assert and
// REFUSE when the assertion fails, instead of degrading to a check-then-write.
//
// The counter starts at 0 and 0 IS a value it hands out — the first call
// returns 0, not 1. Callers that treat 0 as "unset" must say so with a
// separate presence flag, never by skipping it.
type Sequencer interface {
	// NextSequence atomically increments the named counter and returns its new
	// value. The first call for a name returns 0.
	NextSequence(ctx context.Context, name string) (uint64, error)
}

// sequenceDDL is the ONE definition of the counter table, identical in shape on
// both backends: a name, and the last value handed out under it.
const sequenceDDL = `CREATE TABLE IF NOT EXISTS _sequences (
		name TEXT PRIMARY KEY,
		value BIGINT NOT NULL DEFAULT 0
	)`

// Datastore is the interface for Hanzo Datastore (Datastore) analytics queries
type Datastore interface {
	// Query executes datastore queries
	Query(ctx context.Context, query string, args ...interface{}) (DatastoreRows, error)

	// Select scans results into a destination slice
	Select(ctx context.Context, dest interface{}, query string, args ...interface{}) error

	// Exec executes a non-query statement
	Exec(ctx context.Context, query string, args ...interface{}) error

	// Batch insert for high-throughput data ingestion
	PrepareBatch(ctx context.Context, query string) (DatastoreBatch, error)

	// AsyncInsert for fire-and-forget event logging
	AsyncInsert(ctx context.Context, query string, wait bool, args ...interface{}) error

	// Close closes the datastore connection
	Close() error
}

// DatastoreRows represents datastore query results
type DatastoreRows interface {
	Next() bool
	Scan(dest ...interface{}) error
	ScanStruct(dest interface{}) error
	Columns() []string
	Close() error
	Err() error
}

// DatastoreBatch for bulk inserts into Hanzo Datastore
type DatastoreBatch interface {
	Append(v ...interface{}) error
	AppendStruct(v interface{}) error
	Flush() error
	Send() error
	Abort() error
	Rows() int
	Close() error
}

// VectorSearchOptions configures vector similarity search
type VectorSearchOptions struct {
	// Kind is the entity type to search
	Kind string

	// Vector is the query vector
	Vector []float32

	// Limit is the maximum number of results
	Limit int

	// MinScore filters results below this similarity score
	MinScore float32

	// Filters are additional SQL conditions
	Filters map[string]interface{}
}

// VectorResult represents a vector search result
type VectorResult struct {
	// ID is the entity identifier
	ID string

	// Score is the similarity score (0-1, higher is more similar)
	Score float32

	// Metadata is additional data stored with the vector
	Metadata map[string]interface{}
}

// Transaction represents a database transaction
type Transaction interface {
	Get(key Key, dst interface{}) error
	Put(key Key, src interface{}) (Key, error)
	Delete(key Key) error
	Query(kind string) Query
}

// TransactionOptions configures transaction behavior
type TransactionOptions struct {
	// ReadOnly indicates this is a read-only transaction
	ReadOnly bool

	// MaxAttempts for retries on conflict
	MaxAttempts int

	// Isolation level (for SQL databases)
	Isolation IsolationLevel
}

// IsolationLevel represents transaction isolation levels
type IsolationLevel int

const (
	IsolationDefault IsolationLevel = iota
	IsolationReadUncommitted
	IsolationReadCommitted
	IsolationRepeatableRead
	IsolationSerializable
)

// Key represents a unique identifier for an entity
type Key interface {
	// Kind returns the entity kind/table name
	Kind() string

	// StringID returns the string identifier (if any)
	StringID() string

	// IntID returns the integer identifier (if any)
	IntID() int64

	// Parent returns the parent key (for hierarchical keys)
	Parent() Key

	// Namespace returns the namespace/tenant
	Namespace() string

	// Incomplete returns true if this key needs an ID assigned
	Incomplete() bool

	// Encode returns an encoded string representation
	Encode() string

	// Equal checks if two keys are the same
	Equal(other Key) bool
}

// Query provides a fluent interface for querying entities
type Query interface {
	// Filtering
	Filter(filterStr string, value interface{}) Query
	FilterField(fieldPath string, op string, value interface{}) Query

	// Ordering
	Order(fieldPath string) Query
	OrderDesc(fieldPath string) Query

	// Pagination
	Limit(limit int) Query
	Offset(offset int) Query

	// Projection
	Project(fieldNames ...string) Query
	Distinct() Query

	// Ancestor queries (for hierarchical data)
	Ancestor(ancestor Key) Query

	// Execution
	GetAll(ctx context.Context, dst interface{}) ([]Key, error)
	First(ctx context.Context, dst interface{}) (Key, error)
	Count(ctx context.Context) (int, error)
	Keys(ctx context.Context) ([]Key, error)
	Run(ctx context.Context) Iterator

	// Cursors for pagination
	Start(cursor Cursor) Query
	End(cursor Cursor) Query
}

// Iterator allows iterating over query results
type Iterator interface {
	Next(dst interface{}) (Key, error)
	Cursor() (Cursor, error)
}

// Cursor represents a position in a result set
type Cursor interface {
	String() string
}

// Entity is the interface that all model entities should implement
type Entity interface {
	// Kind returns the entity kind/table name
	Kind() string
}

// Syncable entities can be synced to Hanzo Datastore
type Syncable interface {
	Entity

	// SyncToDatastore returns true if this entity should be synced
	SyncToDatastore() bool
}
