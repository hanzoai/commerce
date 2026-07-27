package db

import (
	"context"

	ormdb "github.com/hanzoai/orm/db"
)

// tenantDB is a DB for one tenant that holds no database open.
//
// The registry hands a handle to a callback and takes it back when the callback
// returns (ormdb.Registry.Do), deliberately: a handle you can keep is a handle
// someone must remember to give back, and the forgotten give-back IS the leak
// this replaced — Manager's old userDBs/orgDBs maps opened a store per tenant
// ever touched and closed them only at shutdown.
//
// Commerce's call sites want a db.DB they can keep: Store.DB() returns one per
// request, datastore's orgDBResolver hands one to every namespaced Datastore,
// models/user holds one per service. tenantDB bridges those to Do without
// reintroducing the leak by being a VALUE — a registry pointer, a tenant name,
// and the namespace its keys carry. Each operation borrows the tenant's handle
// and gives it back before returning, so between operations nothing is pinned:
// the bound and the idle sweep still apply, and a caller may hold a tenantDB
// for the life of the process at a cost of four words.
//
// The one thing a caller must not assume is that two consecutive operations run
// on the same open handle. Nothing needs that — the file is the state, and
// anything that must be atomic across statements is already inside
// RunInTransaction, which runs entirely within one borrow.
type tenantDB struct {
	// Keys are minted from the tenant id alone, so building one never opens
	// the store — see tenantKeys in sqlite.go.
	tenantKeys

	reg *ormdb.Registry[DB]
	t   ormdb.Tenant
}

func newTenantDB(reg *ormdb.Registry[DB], t ormdb.Tenant) tenantDB {
	return tenantDB{tenantKeys: tenantKeys(t.ID), reg: reg, t: t}
}

// do borrows this tenant's handle for exactly one operation.
func (d tenantDB) do(ctx context.Context, fn func(DB) error) error {
	return d.reg.Do(ctx, d.t, fn)
}

func (d tenantDB) TenantID() string   { return d.t.ID }
func (d tenantDB) TenantType() string { return d.t.Type }

// Close is a no-op. The registry owns handle lifetime — it opens, bounds and
// closes them — so a caller closing its view of a tenant must not close the
// store other callers are sharing. Manager.Close closes the registry.
func (d tenantDB) Close() error { return nil }

func (d tenantDB) Get(ctx context.Context, key Key, dst interface{}) error {
	return d.do(ctx, func(db DB) error { return db.Get(ctx, key, dst) })
}

func (d tenantDB) Put(ctx context.Context, key Key, src interface{}) (Key, error) {
	var out Key
	err := d.do(ctx, func(db DB) error {
		var e error
		out, e = db.Put(ctx, key, src)
		return e
	})
	return out, err
}

func (d tenantDB) Delete(ctx context.Context, key Key) error {
	return d.do(ctx, func(db DB) error { return db.Delete(ctx, key) })
}

func (d tenantDB) GetMulti(ctx context.Context, keys []Key, dst interface{}) error {
	return d.do(ctx, func(db DB) error { return db.GetMulti(ctx, keys, dst) })
}

func (d tenantDB) PutMulti(ctx context.Context, keys []Key, src interface{}) ([]Key, error) {
	var out []Key
	err := d.do(ctx, func(db DB) error {
		var e error
		out, e = db.PutMulti(ctx, keys, src)
		return e
	})
	return out, err
}

func (d tenantDB) DeleteMulti(ctx context.Context, keys []Key) error {
	return d.do(ctx, func(db DB) error { return db.DeleteMulti(ctx, keys) })
}

func (d tenantDB) VectorSearch(ctx context.Context, opts *VectorSearchOptions) ([]VectorResult, error) {
	var out []VectorResult
	err := d.do(ctx, func(db DB) error {
		var e error
		out, e = db.VectorSearch(ctx, opts)
		return e
	})
	return out, err
}

func (d tenantDB) PutVector(ctx context.Context, kind string, id string, vector []float32, metadata map[string]interface{}) error {
	return d.do(ctx, func(db DB) error { return db.PutVector(ctx, kind, id, vector, metadata) })
}

func (d tenantDB) RunInTransaction(ctx context.Context, fn func(tx Transaction) error, opts *TransactionOptions) error {
	return d.do(ctx, func(db DB) error { return db.RunInTransaction(ctx, fn, opts) })
}

func (d tenantDB) Query(kind string) Query { return tenantQuery{d: d, kind: kind} }

// tenantQuery defers a query to the operation that runs it.
//
// A query is built across arbitrarily many calls and may be parked, branched or
// passed around before anything executes, so there is no borrow that could span
// construction. tenantQuery therefore records the builder calls and replays them
// onto a real query INSIDE the borrow that runs the terminal operation — the
// borrow lasts one statement, not one query object.
type tenantQuery struct {
	d     tenantDB
	kind  string
	steps []func(Query) Query
}

// with returns a query with one more builder step. It copies rather than
// appending in place because db.Query builders return a new query and leave the
// receiver untouched — callers branch off a partially built query relying on it.
func (q tenantQuery) with(step func(Query) Query) Query {
	steps := make([]func(Query) Query, len(q.steps)+1)
	copy(steps, q.steps)
	steps[len(q.steps)] = step
	return tenantQuery{d: q.d, kind: q.kind, steps: steps}
}

// build replays the recorded steps onto a real query from a borrowed handle.
func (q tenantQuery) build(db DB) Query {
	real := db.Query(q.kind)
	for _, step := range q.steps {
		real = step(real)
	}
	return real
}

func (q tenantQuery) Filter(filterStr string, value interface{}) Query {
	return q.with(func(r Query) Query { return r.Filter(filterStr, value) })
}

func (q tenantQuery) FilterField(fieldPath string, op string, value interface{}) Query {
	return q.with(func(r Query) Query { return r.FilterField(fieldPath, op, value) })
}

func (q tenantQuery) Order(fieldPath string) Query {
	return q.with(func(r Query) Query { return r.Order(fieldPath) })
}

func (q tenantQuery) OrderDesc(fieldPath string) Query {
	return q.with(func(r Query) Query { return r.OrderDesc(fieldPath) })
}

func (q tenantQuery) Limit(limit int) Query {
	return q.with(func(r Query) Query { return r.Limit(limit) })
}

func (q tenantQuery) Offset(offset int) Query {
	return q.with(func(r Query) Query { return r.Offset(offset) })
}

func (q tenantQuery) Project(fieldNames ...string) Query {
	return q.with(func(r Query) Query { return r.Project(fieldNames...) })
}

func (q tenantQuery) Distinct() Query {
	return q.with(func(r Query) Query { return r.Distinct() })
}

func (q tenantQuery) Ancestor(ancestor Key) Query {
	return q.with(func(r Query) Query { return r.Ancestor(ancestor) })
}

func (q tenantQuery) Start(cursor Cursor) Query {
	return q.with(func(r Query) Query { return r.Start(cursor) })
}

func (q tenantQuery) End(cursor Cursor) Query {
	return q.with(func(r Query) Query { return r.End(cursor) })
}

func (q tenantQuery) GetAll(ctx context.Context, dst interface{}) ([]Key, error) {
	var keys []Key
	err := q.d.do(ctx, func(db DB) error {
		var e error
		keys, e = q.build(db).GetAll(ctx, dst)
		return e
	})
	return keys, err
}

func (q tenantQuery) First(ctx context.Context, dst interface{}) (Key, error) {
	var key Key
	err := q.d.do(ctx, func(db DB) error {
		var e error
		key, e = q.build(db).First(ctx, dst)
		return e
	})
	return key, err
}

func (q tenantQuery) Count(ctx context.Context) (int, error) {
	var n int
	err := q.d.do(ctx, func(db DB) error {
		var e error
		n, e = q.build(db).Count(ctx)
		return e
	})
	return n, err
}

func (q tenantQuery) Keys(ctx context.Context) ([]Key, error) {
	var keys []Key
	err := q.d.do(ctx, func(db DB) error {
		var e error
		keys, e = q.build(db).Keys(ctx)
		return e
	})
	return keys, err
}

// Run returns the store's own iterator, not a wrapper, for two reasons.
//
// It must stay the real iterator: callers close it through an
// `interface{ Close() error }` assertion (datastore/query.Query.First) to hand
// the pooled connection back, and a wrapper that did not forward Close would
// reinstate the connection leak that starved the pool in production.
//
// And it is safe for it to outlive the borrow: the iterator holds a *sql.Rows,
// which keeps its own connection alive until the rows are done even if the
// pool it came from is closed underneath it. So a mid-iteration eviction cannot
// truncate a result set — TestIteratorSurvivesHandleEviction is the guard.
func (q tenantQuery) Run(ctx context.Context) Iterator {
	var it Iterator
	if err := q.d.do(ctx, func(db DB) error {
		it = q.build(db).Run(ctx)
		return nil
	}); err != nil {
		// Same shape the store itself uses for a query that never ran: an
		// iterator carrying the error, surfaced on the first Next.
		return &sqliteIterator{err: err, kind: q.kind}
	}
	return it
}
