package datastore

import (
	"appengine"

	aeds "appengine/datastore"

	"github.com/gin-gonic/gin"
	"github.com/qedus/nds"

	"hanzo.io/config"
	"hanzo.io/util/log"

	"hanzo.io/datastore/query"
	"hanzo.io/datastore/utils"
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
	Context             appengine.Context
	IgnoreFieldMismatch bool
	Warn                bool
}

func New(ctx interface{}) *Datastore {
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
func (d *Datastore) SetContext(ctx interface{}) {
	switch ctx := ctx.(type) {
	case appengine.Context:
		d.Context = ctx
	case *gin.Context:
		d.Context = ctx.MustGet("appengine").(appengine.Context)
	}
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

func (d *Datastore) RunInTransaction(fn func(db *Datastore) error, opts ...TransactionOptions) error {
	return RunInTransaction(d.Context, fn, opts...)
}

func (d *Datastore) DecodeCursor(cursor string) (aeds.Cursor, error) {
	return aeds.DecodeCursor(cursor)
}

type TransactionOptions aeds.TransactionOptions

func RunInTransaction(ctx appengine.Context, fn func(db *Datastore) error, opts ...TransactionOptions) error {
	aeopts := new(aeds.TransactionOptions)

	if len(opts) > 0 {
		aeopts.XG = opts[0].XG
		aeopts.Attempts = opts[0].Attempts
	}

	return nds.RunInTransaction(ctx, func(ctx appengine.Context) error {
		return fn(New(ctx))
	}, aeopts)
}
