package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// SQLiteDBConfig holds configuration for a SQLite database
type SQLiteDBConfig struct {
	// Path to the SQLite database file
	Path string

	// Config for SQLite options
	Config SQLiteConfig

	// EnableVectorSearch enables sqlite-vec extension
	EnableVectorSearch bool

	// VectorDimensions for embeddings
	VectorDimensions int

	// TenantID for this database (userID or orgID)
	TenantID string

	// TenantType is "user" or "org"
	TenantType string
}

// SQLiteDB implements the DB interface using SQLite
type SQLiteDB struct {
	config *SQLiteDBConfig

	// Concurrent connection for reads
	readDB *sql.DB

	// Serial connection for writes (prevents SQLITE_BUSY)
	writeDB *sql.DB

	// Mutex for write operations
	writeMu sync.Mutex

	// Schema cache
	schemas     map[string]*tableSchema
	schemasMu   sync.RWMutex
	schemaSetup sync.Once

	// Closed flag
	closed bool
	mu     sync.RWMutex
}

// tableSchema holds cached schema information
type tableSchema struct {
	columns map[string]columnInfo
	indexes []string
}

type columnInfo struct {
	name       string
	sqlType    string
	primaryKey bool
	nullable   bool
}

// NewSQLiteDB creates a new SQLite database connection
func NewSQLiteDB(cfg *SQLiteDBConfig) (*SQLiteDB, error) {
	if cfg == nil {
		return nil, errors.New("db: SQLiteDBConfig is required")
	}

	// Ensure directory exists
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("db: failed to create directory %s: %w", dir, err)
	}

	// Build connection string with pragmas
	pragmas := buildPragmas(cfg.Config)

	// Open read connection (concurrent)
	readDB, err := sql.Open("sqlite3", cfg.Path+pragmas)
	if err != nil {
		return nil, fmt.Errorf("db: failed to open read connection: %w", err)
	}

	readDB.SetMaxOpenConns(cfg.Config.MaxOpenConns)
	readDB.SetMaxIdleConns(cfg.Config.MaxIdleConns)

	// Open write connection (single, serialized)
	writeDB, err := sql.Open("sqlite3", cfg.Path+pragmas)
	if err != nil {
		readDB.Close()
		return nil, fmt.Errorf("db: failed to open write connection: %w", err)
	}

	writeDB.SetMaxOpenConns(1)
	writeDB.SetMaxIdleConns(1)

	db := &SQLiteDB{
		config:  cfg,
		readDB:  readDB,
		writeDB: writeDB,
		schemas: make(map[string]*tableSchema),
	}

	// Initialize base schema
	if err := db.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("db: failed to initialize schema: %w", err)
	}

	// Load sqlite-vec if enabled
	if cfg.EnableVectorSearch {
		if err := db.initVectorSearch(); err != nil {
			// Non-fatal, just log and continue without vector search
			// In production, you'd want proper logging here
			fmt.Printf("Warning: sqlite-vec not available: %v\n", err)
		}
	}

	return db, nil
}

// buildPragmas creates the pragma query string
func buildPragmas(cfg SQLiteConfig) string {
	var pragmas []string

	if cfg.BusyTimeout > 0 {
		pragmas = append(pragmas, fmt.Sprintf("_busy_timeout=%d", cfg.BusyTimeout))
	}
	if cfg.JournalMode != "" {
		pragmas = append(pragmas, fmt.Sprintf("_journal_mode=%s", cfg.JournalMode))
	}
	if cfg.Synchronous != "" {
		pragmas = append(pragmas, fmt.Sprintf("_synchronous=%s", cfg.Synchronous))
	}
	if cfg.CacheSize != 0 {
		pragmas = append(pragmas, fmt.Sprintf("_cache_size=%d", cfg.CacheSize))
	}

	// Always enable foreign keys
	pragmas = append(pragmas, "_foreign_keys=ON")

	// Use memory for temp storage
	pragmas = append(pragmas, "_temp_store=MEMORY")

	if len(pragmas) == 0 {
		return ""
	}

	return "?" + strings.Join(pragmas, "&")
}

// initSchema creates the base tables
func (db *SQLiteDB) initSchema() error {
	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	// Create metadata table
	_, err := db.writeDB.Exec(`
		CREATE TABLE IF NOT EXISTS _metadata (
			key TEXT PRIMARY KEY,
			value TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Create generic entity storage table
	_, err = db.writeDB.Exec(`
		CREATE TABLE IF NOT EXISTS _entities (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			parent_id TEXT,
			data JSON NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted INTEGER DEFAULT 0
		)
	`)
	if err != nil {
		return err
	}

	// Create indexes
	_, err = db.writeDB.Exec(`
		CREATE INDEX IF NOT EXISTS idx_entities_kind ON _entities(kind);
		CREATE INDEX IF NOT EXISTS idx_entities_parent ON _entities(parent_id);
		CREATE INDEX IF NOT EXISTS idx_entities_deleted ON _entities(deleted);
	`)
	if err != nil {
		return err
	}

	return nil
}

// initVectorSearch initializes sqlite-vec extension
func (db *SQLiteDB) initVectorSearch() error {
	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	// Try to load sqlite-vec extension
	// This requires the extension to be installed on the system
	_, err := db.writeDB.Exec(`SELECT load_extension('vec0')`)
	if err != nil {
		// Try alternative paths
		paths := []string{
			"vec0",
			"/usr/local/lib/sqlite-vec/vec0",
			"/usr/lib/sqlite-vec/vec0",
		}

		var loadErr error
		for _, path := range paths {
			_, loadErr = db.writeDB.Exec(fmt.Sprintf(`SELECT load_extension('%s')`, path))
			if loadErr == nil {
				break
			}
		}

		if loadErr != nil {
			return fmt.Errorf("failed to load sqlite-vec: %w", loadErr)
		}
	}

	// Create vectors table
	dims := db.config.VectorDimensions
	if dims == 0 {
		dims = 1536 // Default to OpenAI ada-002
	}

	_, err = db.writeDB.Exec(fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS _vectors USING vec0(
			id TEXT PRIMARY KEY,
			kind TEXT,
			embedding FLOAT[%d],
			metadata JSON
		)
	`, dims))
	if err != nil {
		return fmt.Errorf("failed to create vectors table: %w", err)
	}

	return nil
}

// TenantID returns the tenant identifier
func (db *SQLiteDB) TenantID() string {
	return db.config.TenantID
}

// TenantType returns "user" or "org"
func (db *SQLiteDB) TenantType() string {
	return db.config.TenantType
}

// Close closes the database connections
func (db *SQLiteDB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return nil
	}
	db.closed = true

	var errs []error
	if err := db.readDB.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := db.writeDB.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Get retrieves an entity by key
func (db *SQLiteDB) Get(ctx context.Context, key Key, dst interface{}) error {
	if key == nil {
		return ErrInvalidKey
	}

	query := `SELECT data FROM _entities WHERE id = ? AND kind = ? AND deleted = 0`
	row := db.readDB.QueryRowContext(ctx, query, key.Encode(), key.Kind())

	var data []byte
	if err := row.Scan(&data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoSuchEntity
		}
		return err
	}

	return json.Unmarshal(data, dst)
}

// Put stores an entity
func (db *SQLiteDB) Put(ctx context.Context, key Key, src interface{}) (Key, error) {
	if key == nil {
		return nil, ErrInvalidKey
	}

	data, err := json.Marshal(src)
	if err != nil {
		return nil, fmt.Errorf("db: failed to marshal entity: %w", err)
	}

	var parentID *string
	if p := key.Parent(); p != nil {
		id := p.Encode()
		parentID = &id
	}

	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	_, err = db.writeDB.ExecContext(ctx, `
		INSERT INTO _entities (id, kind, parent_id, data, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			data = excluded.data,
			updated_at = CURRENT_TIMESTAMP
	`, key.Encode(), key.Kind(), parentID, data)

	if err != nil {
		return nil, err
	}

	return key, nil
}

// Delete removes an entity (soft delete)
func (db *SQLiteDB) Delete(ctx context.Context, key Key) error {
	if key == nil {
		return ErrInvalidKey
	}

	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	_, err := db.writeDB.ExecContext(ctx, `
		UPDATE _entities SET deleted = 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND kind = ?
	`, key.Encode(), key.Kind())

	return err
}

// GetMulti retrieves multiple entities
func (db *SQLiteDB) GetMulti(ctx context.Context, keys []Key, dst interface{}) error {
	if len(keys) == 0 {
		return nil
	}

	// Build query with placeholders
	placeholders := make([]string, len(keys))
	args := make([]interface{}, len(keys)*2)
	for i, k := range keys {
		placeholders[i] = "(?, ?)"
		args[i*2] = k.Encode()
		args[i*2+1] = k.Kind()
	}

	query := fmt.Sprintf(`
		SELECT id, data FROM _entities
		WHERE (id, kind) IN (%s) AND deleted = 0
	`, strings.Join(placeholders, ","))

	rows, err := db.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Build result map
	results := make(map[string][]byte)
	for rows.Next() {
		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			return err
		}
		results[id] = data
	}

	// Unmarshal into destination slice
	dstVal := reflect.ValueOf(dst)
	if dstVal.Kind() != reflect.Ptr || dstVal.Elem().Kind() != reflect.Slice {
		return errors.New("db: dst must be a pointer to a slice")
	}

	sliceVal := dstVal.Elem()
	elemType := sliceVal.Type().Elem()

	for _, k := range keys {
		data, ok := results[k.Encode()]
		if !ok {
			// Append nil/zero value for missing entities
			sliceVal = reflect.Append(sliceVal, reflect.Zero(elemType))
			continue
		}

		elem := reflect.New(elemType.Elem())
		if err := json.Unmarshal(data, elem.Interface()); err != nil {
			return err
		}
		sliceVal = reflect.Append(sliceVal, elem)
	}

	dstVal.Elem().Set(sliceVal)
	return nil
}

// PutMulti stores multiple entities
func (db *SQLiteDB) PutMulti(ctx context.Context, keys []Key, src interface{}) ([]Key, error) {
	if len(keys) == 0 {
		return keys, nil
	}

	srcVal := reflect.ValueOf(src)
	if srcVal.Kind() != reflect.Slice {
		return nil, errors.New("db: src must be a slice")
	}

	if srcVal.Len() != len(keys) {
		return nil, errors.New("db: keys and src must have same length")
	}

	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	tx, err := db.writeDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO _entities (id, kind, parent_id, data, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			data = excluded.data,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	for i, key := range keys {
		data, err := json.Marshal(srcVal.Index(i).Interface())
		if err != nil {
			return nil, err
		}

		var parentID *string
		if p := key.Parent(); p != nil {
			id := p.Encode()
			parentID = &id
		}

		_, err = stmt.ExecContext(ctx, key.Encode(), key.Kind(), parentID, data)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return keys, nil
}

// DeleteMulti removes multiple entities
func (db *SQLiteDB) DeleteMulti(ctx context.Context, keys []Key) error {
	if len(keys) == 0 {
		return nil
	}

	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	tx, err := db.writeDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		UPDATE _entities SET deleted = 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND kind = ?
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, key := range keys {
		_, err = stmt.ExecContext(ctx, key.Encode(), key.Kind())
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Query returns a new query for the given kind
func (db *SQLiteDB) Query(kind string) Query {
	return &sqliteQuery{
		db:   db,
		kind: kind,
	}
}

// VectorSearch performs similarity search using sqlite-vec
func (db *SQLiteDB) VectorSearch(ctx context.Context, opts *VectorSearchOptions) ([]VectorResult, error) {
	if opts == nil || len(opts.Vector) == 0 {
		return nil, errors.New("db: VectorSearchOptions with Vector is required")
	}

	// Convert vector to JSON array
	vectorJSON, err := json.Marshal(opts.Vector)
	if err != nil {
		return nil, err
	}

	limit := opts.Limit
	if limit == 0 {
		limit = 10
	}

	// Build query
	query := `
		SELECT id, distance, metadata
		FROM _vectors
		WHERE embedding MATCH ?
	`

	args := []interface{}{string(vectorJSON)}

	if opts.Kind != "" {
		query += " AND kind = ?"
		args = append(args, opts.Kind)
	}

	query += fmt.Sprintf(" ORDER BY distance LIMIT %d", limit)

	rows, err := db.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []VectorResult
	for rows.Next() {
		var r VectorResult
		var distance float32
		var metadataJSON []byte

		if err := rows.Scan(&r.ID, &distance, &metadataJSON); err != nil {
			return nil, err
		}

		// Convert distance to similarity score (1 / (1 + distance))
		r.Score = 1 / (1 + distance)

		if opts.MinScore > 0 && r.Score < opts.MinScore {
			continue
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &r.Metadata)
		}

		results = append(results, r)
	}

	return results, rows.Err()
}

// PutVector stores a vector embedding
func (db *SQLiteDB) PutVector(ctx context.Context, kind string, id string, vector []float32, metadata map[string]interface{}) error {
	vectorJSON, err := json.Marshal(vector)
	if err != nil {
		return err
	}

	var metadataJSON []byte
	if metadata != nil {
		metadataJSON, err = json.Marshal(metadata)
		if err != nil {
			return err
		}
	}

	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	_, err = db.writeDB.ExecContext(ctx, `
		INSERT OR REPLACE INTO _vectors (id, kind, embedding, metadata)
		VALUES (?, ?, ?, ?)
	`, id, kind, string(vectorJSON), metadataJSON)

	return err
}

// NewKey creates a new key
func (db *SQLiteDB) NewKey(kind string, stringID string, intID int64, parent Key) Key {
	return &sqliteKey{
		kind:      kind,
		stringID:  stringID,
		intID:     intID,
		parent:    parent,
		namespace: db.config.TenantID,
	}
}

// NewIncompleteKey creates a key that will be assigned an ID on Put
func (db *SQLiteDB) NewIncompleteKey(kind string, parent Key) Key {
	return &sqliteKey{
		kind:       kind,
		parent:     parent,
		namespace:  db.config.TenantID,
		incomplete: true,
	}
}

// AllocateIDs pre-allocates entity IDs
func (db *SQLiteDB) AllocateIDs(kind string, parent Key, n int) ([]Key, error) {
	keys := make([]Key, n)
	for i := 0; i < n; i++ {
		keys[i] = &sqliteKey{
			kind:      kind,
			stringID:  generateID(),
			parent:    parent,
			namespace: db.config.TenantID,
		}
	}
	return keys, nil
}

// RunInTransaction executes a function within a transaction
func (db *SQLiteDB) RunInTransaction(ctx context.Context, fn func(tx Transaction) error, opts *TransactionOptions) error {
	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	sqlTx, err := db.writeDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer sqlTx.Rollback()

	tx := &sqliteTransaction{
		db: db,
		tx: sqlTx,
	}

	if err := fn(tx); err != nil {
		return err
	}

	return sqlTx.Commit()
}

// sqliteKey implements the Key interface
type sqliteKey struct {
	kind       string
	stringID   string
	intID      int64
	parent     Key
	namespace  string
	incomplete bool
}

func (k *sqliteKey) Kind() string      { return k.kind }
func (k *sqliteKey) StringID() string  { return k.stringID }
func (k *sqliteKey) IntID() int64      { return k.intID }
func (k *sqliteKey) Parent() Key       { return k.parent }
func (k *sqliteKey) Namespace() string { return k.namespace }
func (k *sqliteKey) Incomplete() bool  { return k.incomplete }

func (k *sqliteKey) Encode() string {
	if k.stringID != "" {
		return k.stringID
	}
	if k.intID != 0 {
		return fmt.Sprintf("%d", k.intID)
	}
	// Generate new ID for incomplete keys
	if k.incomplete {
		k.stringID = generateID()
		k.incomplete = false
	}
	return k.stringID
}

func (k *sqliteKey) Equal(other Key) bool {
	if other == nil {
		return false
	}
	return k.Kind() == other.Kind() && k.Encode() == other.Encode()
}

// generateID creates a unique ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// sqliteTransaction implements Transaction
type sqliteTransaction struct {
	db *SQLiteDB
	tx *sql.Tx
}

func (t *sqliteTransaction) Get(key Key, dst interface{}) error {
	query := `SELECT data FROM _entities WHERE id = ? AND kind = ? AND deleted = 0`
	row := t.tx.QueryRow(query, key.Encode(), key.Kind())

	var data []byte
	if err := row.Scan(&data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoSuchEntity
		}
		return err
	}

	return json.Unmarshal(data, dst)
}

func (t *sqliteTransaction) Put(key Key, src interface{}) (Key, error) {
	data, err := json.Marshal(src)
	if err != nil {
		return nil, err
	}

	var parentID *string
	if p := key.Parent(); p != nil {
		id := p.Encode()
		parentID = &id
	}

	_, err = t.tx.Exec(`
		INSERT INTO _entities (id, kind, parent_id, data, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			data = excluded.data,
			updated_at = CURRENT_TIMESTAMP
	`, key.Encode(), key.Kind(), parentID, data)

	return key, err
}

func (t *sqliteTransaction) Delete(key Key) error {
	_, err := t.tx.Exec(`
		UPDATE _entities SET deleted = 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND kind = ?
	`, key.Encode(), key.Kind())
	return err
}

func (t *sqliteTransaction) Query(kind string) Query {
	return &sqliteQuery{
		db:   t.db,
		kind: kind,
		tx:   t.tx,
	}
}
