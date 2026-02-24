package mixin

// Model[T] wraps orm.Model[T] and implements the full mixin.Entity
// interface. Embed this in model structs instead of mixin.BaseModel to use ORM
// generics while remaining compatible with the REST layer.
//
// Usage:
//
//	type Note struct {
//	    mixin.Model[Note]
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

// Model[T] embeds orm.Model[T] and provides mixin.Entity compatibility.
// The orm.Model[T] must be the first field so self() pointer arithmetic works.
type Model[T any] struct {
	orm.Model[T]
	ds      *datastore.Datastore `json:"-" datastore:"-"`
	Parent  datastore.Key       `json:"-" datastore:"-"`
	Mock    bool                `json:"-" datastore:"-"`
	loaded_ bool                `json:"-" datastore:"-"`
}

// self returns a pointer to the outermost struct T that embeds this bridge.
// Works because Model[T] is at offset 0 of T.
func (b *Model[T]) self() *T {
	return (*T)(reflect.NewAt(
		reflect.TypeOf((*T)(nil)).Elem(),
		reflect.ValueOf(b).UnsafePointer(),
	).UnsafePointer())
}

// --- Initialization ---

// Init wires the bridge to a commerce datastore.
// Creates an ORM adapter, initializes the orm.Model, and applies defaults.
func (b *Model[T]) Init(db *datastore.Datastore) {
	b.ds = db
	adapter := NewDatastoreAdapter(db)
	b.Model.Init(adapter)
	orm.ApplyDefaults(b.self())
}

// --- Context / Namespace ---

func (b *Model[T]) Context() context.Context {
	if b.ds != nil {
		return b.ds.Context
	}
	return context.Background()
}

func (b *Model[T]) SetContext(ctx context.Context) {
	if b.ds == nil {
		b.ds = datastore.New(ctx)
		adapter := NewDatastoreAdapter(b.ds)
		b.Model.Init(adapter)
	} else {
		b.ds.SetContext(ctx)
	}
}

func (b *Model[T]) SetNamespace(namespace string) {
	if b.ds != nil {
		b.ds.SetNamespace(namespace)
	}
	b.Model.SetNamespace(namespace)
}

func (b *Model[T]) Namespace() string {
	if b.ds != nil {
		return b.ds.GetNamespace()
	}
	return b.Model.Namespace()
}

// --- Key management ---
// These shadow orm.Model[T]'s methods to return datastore.Key instead of orm.Key.

func (b *Model[T]) Key() datastore.Key {
	ormKey := b.Model.Key()
	return OrmKeyToDS(ormKey)
}

func (b *Model[T]) SetKey(key interface{}) error {
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

func (b *Model[T]) NewKey() datastore.Key {
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
func (b *Model[T]) Get(key datastore.Key) error {
	if key != nil {
		b.Model.SetKey(DSKeyToOrm(key))
	}
	return b.Model.Get(b.Model.Key())
}

// GetById delegates to orm.Model[T].GetById (matching signature).
// Inherited directly — no override needed.

// --- Must variants ---

func (b *Model[T]) MustSetKey(key interface{}) {
	if err := b.SetKey(key); err != nil {
		panic(err)
	}
}

func (b *Model[T]) MustGet(key datastore.Key) {
	if err := b.Get(key); err != nil {
		panic(err)
	}
}

func (b *Model[T]) MustGetById(id string) {
	if err := b.Model.GetById(id); err != nil {
		panic(err)
	}
}

func (b *Model[T]) MustPut() {
	if err := b.Model.Put(); err != nil {
		panic(err)
	}
}

// --- Existence checks ---

func (b *Model[T]) Exists() (bool, error) {
	return b.Model.Exists()
}

func (b *Model[T]) IdExists(id string) (datastore.Key, bool, error) {
	q := b.queryDS()
	return q.IdExists(id)
}

func (b *Model[T]) KeyExists(key datastore.Key) (bool, error) {
	q := b.queryDS()
	return q.KeyExists(key)
}

// --- Document/Search ---

func (b *Model[T]) PutDocument() error {
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

func (b *Model[T]) DeleteDocument() error {
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

func (b *Model[T]) GetOrCreate(filterStr string, value interface{}) error {
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

func (b *Model[T]) GetOrUpdate(filterStr string, value interface{}) error {
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

func (b *Model[T]) Datastore() *datastore.Datastore {
	return b.ds
}

func (b *Model[T]) RunInTransaction(fn func() error, opts *datastore.TransactionOptions) error {
	return datastore.RunInTransaction(b.Context(), func(db *datastore.Datastore) error {
		return fn()
	}, opts)
}

// --- Query ---

// queryDS returns a raw datastore query (for internal use).
func (b *Model[T]) queryDS() datastore.Query {
	return query.New(b.Context(), b.Model.Kind())
}

// Query returns a mixin.ModelQuery for this entity.
func (b *Model[T]) Query() *ModelQuery {
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

func (b *Model[T]) Zero() Entity {
	typ := reflect.TypeOf((*T)(nil)).Elem()
	entity := reflect.New(typ).Interface()
	if e, ok := entity.(Entity); ok {
		return e
	}
	return nil
}

func (b *Model[T]) Clone() Entity {
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

func (b *Model[T]) CloneFromJSON() Entity {
	data := b.JSON()
	clone := new(T)
	json.Unmarshal(data, clone)
	if e, ok := any(clone).(Entity); ok {
		return e
	}
	return nil
}

func (b *Model[T]) Slice() interface{} {
	typ := reflect.TypeOf((*T)(nil))
	sliceType := reflect.SliceOf(typ)
	slice := reflect.MakeSlice(sliceType, 0, 0)
	ptr := reflect.New(slice.Type())
	ptr.Elem().Set(slice)
	return ptr.Interface()
}

func (b *Model[T]) JSON() []byte {
	data, _ := json.Marshal(b.self())
	return data
}

func (b *Model[T]) JSONString() string {
	return string(b.JSON())
}

// --- Timestamp accessors for compatibility ---

func (b *Model[T]) Created() bool {
	return !b.Model.CreatedAt.IsZero()
}

func (b *Model[T]) GetCreatedAt() time.Time {
	return b.Model.CreatedAt
}

func (b *Model[T]) GetUpdatedAt() time.Time {
	return b.Model.UpdatedAt
}

// --- Load guard ---

// loaded_ prevents duplicate deserialization in Load() methods.
// Matches mixin.BaseModel.Loaded() semantics: returns true if already loaded,
// otherwise marks as loaded and returns false.
func (b *Model[T]) Loaded() bool {
	if b.loaded_ {
		return true
	}
	b.loaded_ = true
	return false
}

// Compile-time verification that Model satisfies Entity.
// We can't do this generically, but each migrated model will verify via the REST layer.
