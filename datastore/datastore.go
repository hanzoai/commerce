package datastore

import (
	"context"

	"google.golang.org/appengine"
	aeds "google.golang.org/appengine/datastore"

	"github.com/gin-gonic/gin"
	"github.com/qedus/nds"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/datastore/query"
	"github.com/hanzoai/commerce/datastore/utils"
	"github.com/hanzoai/commerce/log"
)

var (
	// Alias appengine types
	Done                 = aeds.Done
	ErrNoSuchEntity      = aeds.ErrNoSuchEntity
	ErrInvalidEntityType = aeds.ErrInvalidEntityType
	ErrInvalidKey        = aeds.ErrInvalidKey

	// Alias utils
	IgnoreFieldMismatch = utils.IgnoreFieldMismatch
)

type Datastore struct {
	Context             context.Context
	IgnoreFieldMismatch bool
	Warn                bool
}

func New(ctx context.Context) *Datastore {
	d := new(Datastore)
	d.IgnoreFieldMismatch = true
	d.Warn = config.DatastoreWarn
	d.SetContext(ctx)
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
		ctx = c.MustGet("appengine").(context.Context)
	}
	d.Context = ctx
}

// Set context for datastore
func (d *Datastore) SetNamespace(ns string) {
	if ctx, err := appengine.Namespace(d.Context, ns); err != nil {
		log.Error("Unable to set namespace for datastore: %v", err, d.Context)
	} else {
		d.Context = ctx
	}
}

// Return a *datastore.Query
func (d *Datastore) Query(kind string) Query {
	return query.New(d.Context, kind)
}

func (d *Datastore) RunInTransaction(fn func(db *Datastore) error, opts *TransactionOptions) error {
	return RunInTransaction(d.Context, fn, opts)
}

func (d *Datastore) DecodeCursor(cursor string) (aeds.Cursor, error) {
	return aeds.DecodeCursor(cursor)
}

type TransactionOptions aeds.TransactionOptions

func RunInTransaction(ctx context.Context, fn func(db *Datastore) error, opts *TransactionOptions) error {
	aeopts := (*aeds.TransactionOptions)(opts)
	return nds.RunInTransaction(ctx, func(ctx context.Context) error {
		return fn(New(ctx))
	}, aeopts)
}
