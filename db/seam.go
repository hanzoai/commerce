package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"time"

	"github.com/hanzoai/namespace"
)

// Seam is how a host that keeps every store in one place gives commerce its tenants'
// databases. The host opens the store of a tenant's namespace wherever it lives —
// a schema on a shared server, or a SQLite file — and may move it from the file to the
// server on the way; commerce supplies open, which builds its store over the handle the
// host chose. postgres says which: true is a connection pool whose search_path is the
// tenant's own schema, false a single-connection handle on the tenant's SQLite file.
// The store open returns is the host's to close, and commerce's to use.
//
// The host knows nothing of commerce's tables: open is the whole of its schema, run on
// the file to bring it to the current shape and on the schema to make its tables, which
// is what lets a file be moved to the server as it is.
//
// A user's store has no namespace and is never opened through it.
type Seam func(ns namespace.Namespace, open func(h *sql.DB, postgres bool) (io.Closer, error)) (io.Closer, error)

// seamTenantColumn is the column of _entities that holds a row's tenant on a store a
// host keeps. It is the SQLite store's name for it, so the file and the schema are the
// same shape and a move carries every row by its column.
const seamTenantColumn = "namespace"

// openSeam opens one org's store through the host.
func (m *Manager) openSeam(typ, id string) (DB, error) {
	ns, err := tenantNamespace(typ, id)
	if err != nil {
		return nil, err
	}
	store, err := m.config.Seam(ns, func(h *sql.DB, postgres bool) (io.Closer, error) {
		if postgres {
			return NewPostgresDBOn(h, &PostgresDBConfig{TenantID: id, TenantType: typ, TenantColumn: seamTenantColumn})
		}
		return NewSQLiteDBOn(h, &SQLiteDBConfig{TenantID: id, TenantType: typ, Config: m.config.SQLite})
	})
	if err != nil {
		return nil, err
	}
	db, ok := store.(DB)
	if !ok {
		_ = store.Close()
		return nil, fmt.Errorf("db: the host's seam answered a %T, not a store", store)
	}
	return db, nil
}

// NewSQLiteDBOn is NewSQLiteDB over a handle the host opened: its file, keyed and
// pooled as the host keeps every file. The handle is one connection, so reads and writes
// take turns on it, and the write mutex still orders writers.
func NewSQLiteDBOn(h *sql.DB, cfg *SQLiteDBConfig) (*SQLiteDB, error) {
	db := &SQLiteDB{
		tenantKeys: tenantKeys(cfg.TenantID),
		config:     cfg,
		readDB:     h,
		writeDB:    h,
		schemas:    make(map[string]*tableSchema),
		encrypted:  true, // the host keys every file it opens
	}
	if err := db.initSchema(); err != nil {
		return nil, fmt.Errorf("db: failed to initialize schema: %w", err)
	}
	return db, nil
}

// NewPostgresDBOn is NewPostgresDB over a pool the host opened, whose search_path is
// the tenant's schema: the tables are made in it and nowhere else, under the shape the
// SQLite store has, tenant column included. cfg.DSN is not read. It never touches a
// table that is there — replicas open one tenant's store while the others write to it,
// and a DROP TRIGGER or a CREATE INDEX on a live table waits for every writer.
func NewPostgresDBOn(h *sql.DB, cfg *PostgresDBConfig) (*PostgresDB, error) {
	pdb := &PostgresDB{config: cfg, db: h, tenantCol: cfg.column()}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := pdb.migrate(ctx); err != nil {
		return nil, fmt.Errorf("db: failed to initialize schema: %w", err)
	}
	if err := pdb.prepareStatements(); err != nil {
		return nil, fmt.Errorf("db: failed to prepare statements: %w", err)
	}
	return pdb, nil
}

// migrate makes the tenant's schema once. It runs in one transaction under an advisory
// lock named for the schema, so two replicas opening a new tenant together make it once,
// and it does nothing when the tables and the guard are there.
func (db *PostgresDB) migrate(ctx context.Context) error {
	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(current_schema() || ':migrate', 0))`); err != nil {
		return err
	}
	var made bool
	if err := tx.QueryRowContext(ctx, `SELECT to_regclass('_entities') IS NOT NULL
		AND EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_reject_bearer_tenant' AND tgrelid = to_regclass('_entities'))`).Scan(&made); err != nil {
		return err
	}
	if made {
		return nil
	}
	for _, stmt := range db.schemaDDL() {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	for _, stmt := range postgresGuardDDL(db.tenantCol) {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("credential guard: %w", err)
		}
	}
	return tx.Commit()
}
