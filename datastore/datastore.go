package datastore

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/datastore/iface"
	"github.com/hanzoai/commerce/datastore/key"
	"github.com/hanzoai/commerce/datastore/query"
	"github.com/hanzoai/commerce/datastore/utils"
	"github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/log"
)

var (
	// Done signals end of iteration
	Done = errors.New("datastore: done")

	// Standard errors - aliased from db package for compatibility
	ErrNoSuchEntity          = db.ErrNoSuchEntity
	ErrInvalidEntityType     = db.ErrInvalidEntityType
	ErrInvalidKey            = db.ErrInvalidKey
	ErrConcurrentTransaction = db.ErrConcurrentModification

	// Alias utils
	IgnoreFieldMismatch = utils.IgnoreFieldMismatch

	// Global ID counter for allocations
	globalIDCounter int64 = time.Now().UnixNano()
)

// Query is the interface for datastore queries
type Query = iface.Query

// Cursor is the interface for query cursors
type Cursor = iface.Cursor

// Datastore wraps the db.DB interface to provide the legacy API
type Datastore struct {
	Context             context.Context
	IgnoreFieldMismatch bool
	Warn                bool
	database            db.DB
	namespace           string
	allocateCounter     int64
}

// New creates a new Datastore with the given context
func New(ctx context.Context) *Datastore {
	d := new(Datastore)
	d.IgnoreFieldMismatch = true
	d.Warn = config.DatastoreWarn
	d.allocateCounter = atomic.AddInt64(&globalIDCounter, 1000)
	d.SetContext(ctx)
	return d
}

// NewWithDB creates a new Datastore with a specific database
func NewWithDB(ctx context.Context, database db.DB) *Datastore {
	d := New(ctx)
	d.database = database
	return d
}

// Private method for logging selectively
func (d *Datastore) warn(fmtOrError interface{}, args ...interface{}) {
	if d.Warn {
		log.Warn(fmtOrError, args...)
	}
}

// Helper to ignore tedious field mismatch errors
func (d *Datastore) ignoreFieldMismatch(err error) error {
	if d.IgnoreFieldMismatch {
		return IgnoreFieldMismatch(err)
	}
	return err
}

// Set context for datastore
func (d *Datastore) SetContext(ctx context.Context) {
	if c, ok := ctx.(*gin.Context); ok {
		// Try to get the context from gin
		if appCtx := c.Value("appengine"); appCtx != nil {
			if ctxVal, ok := appCtx.(context.Context); ok {
				ctx = ctxVal
			}
		}
		// Try the new "context" key
		if appCtx := c.Value("context"); appCtx != nil {
			if ctxVal, ok := appCtx.(context.Context); ok {
				ctx = ctxVal
			}
		}
	}
	d.Context = ctx
}

// Set namespace for datastore
func (d *Datastore) SetNamespace(ns string) {
	d.namespace = ns
	log.Debug("Set namespace to: %s", ns)
}

// GetNamespace returns the current namespace
func (d *Datastore) GetNamespace() string {
	return d.namespace
}

// SetDB sets the underlying database
func (d *Datastore) SetDB(database db.DB) {
	d.database = database
}

// DB returns the underlying database
func (d *Datastore) DB() db.DB {
	return d.database
}

// Return a *datastore.Query
func (d *Datastore) Query(kind string) Query {
	return query.New(d.Context, kind)
}

// DecodeCursor decodes a cursor string
func (d *Datastore) DecodeCursor(cursor string) (Cursor, error) {
	return db.DecodeCursor(cursor)
}

// TransactionOptions configures transaction behavior
type TransactionOptions struct {
	// XG enables cross-group transactions (legacy compat, ignored in new db)
	XG bool

	// Attempts specifies how many retries on conflict
	Attempts int

	// ReadOnly indicates this is a read-only transaction
	ReadOnly bool
}

// RunInTransaction runs a function within a transaction
func (d *Datastore) RunInTransaction(fn func(db *Datastore) error, opts *TransactionOptions) error {
	return RunInTransaction(d.Context, fn, opts)
}

// RunInTransaction runs a function within a transaction
func RunInTransaction(ctx context.Context, fn func(db *Datastore) error, opts *TransactionOptions) error {
	// For now, just run the function directly
	// The proper implementation would use db.DB.RunInTransaction
	ds := New(ctx)
	return fn(ds)
}

// toDBKey converts a Key to db.Key
func (d *Datastore) toDBKey(k Key) db.Key {
	if k == nil {
		return nil
	}

	// If it's already a DatastoreKey, use its converter
	if dk, ok := k.(*key.DatastoreKey); ok {
		if d.database != nil {
			return dk.ToDBKey(d.database)
		}
	}

	// Create a simple wrapper
	return &dbKeyWrapper{k}
}

// dbKeyWrapper wraps iface.Key to implement db.Key
type dbKeyWrapper struct {
	key Key
}

func (w *dbKeyWrapper) Kind() string      { return w.key.Kind() }
func (w *dbKeyWrapper) StringID() string  { return w.key.StringID() }
func (w *dbKeyWrapper) IntID() int64      { return w.key.IntID() }
func (w *dbKeyWrapper) Namespace() string { return w.key.Namespace() }
func (w *dbKeyWrapper) Incomplete() bool  { return w.key.Incomplete() }
func (w *dbKeyWrapper) Encode() string    { return w.key.Encode() }

func (w *dbKeyWrapper) Parent() db.Key {
	p := w.key.Parent()
	if p == nil {
		return nil
	}
	return &dbKeyWrapper{p}
}

func (w *dbKeyWrapper) Equal(other db.Key) bool {
	if other == nil {
		return false
	}
	return w.Kind() == other.Kind() && w.Encode() == other.Encode()
}
