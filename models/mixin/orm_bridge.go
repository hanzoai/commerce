package mixin

// EntityBridge[T] wraps orm.Model[T] and implements the full mixin.Entity
// interface. Embed this in model structs instead of mixin.Model to use ORM
// generics while remaining compatible with the REST layer.
//
// Usage:
//
//	type Note struct {
//	    mixin.EntityBridge[Note]
//	    Enabled bool   `json:"enabled" orm:"default:true"`
//	    Message string `json:"message"`
//	}
//	func init() { orm.Register[Note]("note") }

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/datastore/query"
	"github.com/hanzoai/orm"
)

// EntityBridge[T] embeds orm.Model[T] and provides mixin.Entity compatibility.
// The orm.Model[T] must be the first field so self() pointer arithmetic works.
type EntityBridge[T any] struct {
	orm.Model[T]
	ds     *datastore.Datastore `json:"-" datastore:"-"`
	Parent datastore.Key        `json:"-" datastore:"-"`
	Mock   bool                 `json:"-" datastore:"-"`
}

// self returns a pointer to the outermost struct T that embeds this bridge.
// Works because EntityBridge[T] is at offset 0 of T.
func (b *EntityBridge[T]) self() *T {
	return (*T)(reflect.NewAt(
		reflect.TypeOf((*T)(nil)).Elem(),
		reflect.ValueOf(b).UnsafePointer(),
	).UnsafePointer())
}

// --- Initialization ---

// Init wires the bridge to a commerce datastore.
// Creates an ORM adapter, initializes the orm.Model, and applies defaults.
func (b *EntityBridge[T]) Init(db *datastore.Datastore) {
	b.ds = db
	adapter := NewDatastoreAdapter(db)
	b.Model.Init(adapter)
	orm.ApplyDefaults(b.self())
}

// --- Context / Namespace ---

func (b *EntityBridge[T]) Context() context.Context {
	if b.ds != nil {
		return b.ds.Context
	}
	return context.Background()
}

func (b *EntityBridge[T]) SetContext(ctx context.Context) {
	if b.ds == nil {
		b.ds = datastore.New(ctx)
		adapter := NewDatastoreAdapter(b.ds)
		b.Model.Init(adapter)
	} else {
		b.ds.SetContext(ctx)
	}
}

func (b *EntityBridge[T]) SetNamespace(namespace string) {
	if b.ds != nil {
		b.ds.SetNamespace(namespace)
	}
	b.Model.SetNamespace(namespace)
}

func (b *EntityBridge[T]) Namespace() string {
	if b.ds != nil {
		return b.ds.GetNamespace()
	}
	return b.Model.Namespace()
}

// --- Key management ---
// These shadow orm.Model[T]'s methods to return datastore.Key instead of orm.Key.

func (b *EntityBridge[T]) Key() datastore.Key {
	ormKey := b.Model.Key()
	return OrmKeyToDS(ormKey)
}

func (b *EntityBridge[T]) SetKey(key interface{}) error {
	switch v := key.(type) {
	case datastore.Key:
		b.Model.SetKey(DSKeyToOrm(v))
	case orm.Key:
		b.Model.SetKey(v)
	case string:
		b.Model.SetId(v)
	case int64:
		kind := b.Model.Kind()
		ormKey := b.Model.DB().NewKey(kind, "", v, nil)
		b.Model.SetKey(ormKey)
	case int:
		kind := b.Model.Kind()
		ormKey := b.Model.DB().NewKey(kind, "", int64(v), nil)
		b.Model.SetKey(ormKey)
	case nil:
		// no-op
	default:
		return fmt.Errorf("orm bridge: unable to set %v as key", key)
	}
	return nil
}

func (b *EntityBridge[T]) NewKey() datastore.Key {
	if b.ds != nil {
		kind := b.Model.Kind()
		return b.ds.AllocateOrphanKey(kind, b.Parent)
	}
	return OrmKeyToDS(b.Model.Key())
}

// --- CRUD (inherited from orm.Model[T], no override needed for matching sigs) ---
// Put, Create, Update, Delete, MustCreate, MustUpdate, MustDelete are
// inherited from orm.Model[T] with matching signatures.

// Get overrides orm.Model[T].Get to accept datastore.Key.
func (b *EntityBridge[T]) Get(key datastore.Key) error {
	if key != nil {
		b.Model.SetKey(DSKeyToOrm(key))
	}
	return b.Model.Get(b.Model.Key())
}

// GetById delegates to orm.Model[T].GetById (matching signature).
// Inherited directly — no override needed.

// --- Must variants ---

func (b *EntityBridge[T]) MustSetKey(key interface{}) {
	if err := b.SetKey(key); err != nil {
		panic(err)
	}
}

func (b *EntityBridge[T]) MustGet(key datastore.Key) {
	if err := b.Get(key); err != nil {
		panic(err)
	}
}

func (b *EntityBridge[T]) MustGetById(id string) {
	if err := b.Model.GetById(id); err != nil {
		panic(err)
	}
}

func (b *EntityBridge[T]) MustPut() {
	if err := b.Model.Put(); err != nil {
		panic(err)
	}
}

// --- Existence checks ---

func (b *EntityBridge[T]) Exists() (bool, error) {
	return b.Model.Exists()
}

func (b *EntityBridge[T]) IdExists(id string) (datastore.Key, bool, error) {
	q := b.queryDS()
	return q.IdExists(id)
}

func (b *EntityBridge[T]) KeyExists(key datastore.Key) (bool, error) {
	q := b.queryDS()
	return q.KeyExists(key)
}

// --- Document/Search ---

func (b *EntityBridge[T]) PutDocument() error {
	entity := b.self()
	hook, ok := any(entity).(Searchable)
	if !ok {
		return nil
	}
	if doc := hook.Document(); doc != nil {
		if searchIndexProvider == nil {
			return nil
		}
		return searchIndexProvider.Put(b.Context(), b.Model.Id(), doc)
	}
	return nil
}

func (b *EntityBridge[T]) DeleteDocument() error {
	entity := b.self()
	hook, ok := any(entity).(Searchable)
	if !ok {
		return nil
	}
	if doc := hook.Document(); doc != nil {
		if searchIndexProvider == nil {
			return nil
		}
		return searchIndexProvider.Delete(b.Context(), b.Model.Id())
	}
	return nil
}

// --- GetOrCreate / GetOrUpdate ---

func (b *EntityBridge[T]) GetOrCreate(filterStr string, value interface{}) error {
	q := b.Query()
	ok, err := q.Filter(filterStr, value).Get()
	if err != nil {
		return err
	}
	if !ok {
		return b.Model.Create()
	}
	return nil
}

func (b *EntityBridge[T]) GetOrUpdate(filterStr string, value interface{}) error {
	q := b.Query()
	ok, err := q.Filter(filterStr, value).Get()
	if err != nil {
		return err
	}
	if !ok {
		return b.Model.Create()
	}
	return b.Model.Update()
}

// --- Datastore / Transaction ---

func (b *EntityBridge[T]) Datastore() *datastore.Datastore {
	return b.ds
}

func (b *EntityBridge[T]) RunInTransaction(fn func() error, opts *datastore.TransactionOptions) error {
	return datastore.RunInTransaction(b.Context(), func(db *datastore.Datastore) error {
		return fn()
	}, opts)
}

// --- Query ---

// queryDS returns a raw datastore query (for internal use).
func (b *EntityBridge[T]) queryDS() datastore.Query {
	return query.New(b.Context(), b.Model.Kind())
}

// Query returns a mixin.ModelQuery for this entity.
func (b *EntityBridge[T]) Query() *ModelQuery {
	q := new(ModelQuery)
	entity, ok := any(b.self()).(Entity)
	if ok {
		q.entity = entity
	}
	q.dsq = b.queryDS()
	q.db = b.ds
	return q
}

// --- Utility methods ---

func (b *EntityBridge[T]) Zero() Entity {
	typ := reflect.TypeOf((*T)(nil)).Elem()
	entity := reflect.New(typ).Interface()
	if e, ok := entity.(Entity); ok {
		return e
	}
	return nil
}

func (b *EntityBridge[T]) Clone() Entity {
	entity := b.self()
	data, err := json.Marshal(entity)
	if err != nil {
		return b.Zero()
	}
	clone := new(T)
	json.Unmarshal(data, clone)
	if e, ok := any(clone).(Entity); ok {
		return e
	}
	return nil
}

func (b *EntityBridge[T]) CloneFromJSON() Entity {
	data := b.JSON()
	clone := new(T)
	json.Unmarshal(data, clone)
	if e, ok := any(clone).(Entity); ok {
		return e
	}
	return nil
}

func (b *EntityBridge[T]) Slice() interface{} {
	typ := reflect.TypeOf((*T)(nil))
	sliceType := reflect.SliceOf(typ)
	slice := reflect.MakeSlice(sliceType, 0, 0)
	ptr := reflect.New(slice.Type())
	ptr.Elem().Set(slice)
	return ptr.Interface()
}

func (b *EntityBridge[T]) JSON() []byte {
	data, _ := json.Marshal(b.self())
	return data
}

func (b *EntityBridge[T]) JSONString() string {
	return string(b.JSON())
}

// --- Timestamp accessors for compatibility ---

func (b *EntityBridge[T]) Created() bool {
	return !b.Model.CreatedAt.IsZero()
}

func (b *EntityBridge[T]) GetCreatedAt() time.Time {
	return b.Model.CreatedAt
}

func (b *EntityBridge[T]) GetUpdatedAt() time.Time {
	return b.Model.UpdatedAt
}

// Compile-time verification that EntityBridge satisfies Entity.
// We can't do this generically, but each migrated model will verify via the REST layer.
