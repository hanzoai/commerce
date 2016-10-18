package mixin

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"appengine"
	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/util/cache"
	"crowdstart.com/util/hashid"
	"crowdstart.com/util/log"
	"crowdstart.com/util/rand"
	"crowdstart.com/util/structs"
	"crowdstart.com/util/timeutil"
)

// A datastore kind that is compatible with the Model mixin
type Kind interface {
	Kind() string
}

// A specific datastore entity, with methods inherited from this mixin
type Entity interface {
	// TODO: Should not be embedded in Entity I don't think
	Kind

	// By convention where model is wired to entity
	Init(db *datastore.Datastore)

	// Get, Set context/namespace
	Context() appengine.Context
	SetContext(ctx interface{})
	SetNamespace(namespace string)
	Namespace() string

	// Get, Set keys
	Key() (key datastore.Key)
	SetKey(key interface{}) error
	NewKey() datastore.Key
	Id() string

	// Various existential helpers
	Exists() (bool, error)
	IdExists(id string) (datastore.Key, bool, error)
	KeyExists(key datastore.Key) (bool, error)

	// Get, Put, Delete + Create, Update
	Get(args ...interface{}) error
	GetById(string) error
	Put() error
	Create() error
	Update() error
	Delete() error

	// Must variants
	MustCreate()
	MustDelete()
	MustGet(args ...interface{})
	MustPut()
	MustUpdate()

	// Document
	PutDocument() error
	DeleteDocument() error

	// Get or Create, Update helpers
	GetOrCreate(filterStr string, value interface{}) error
	GetOrUpdate(filterStr string, value interface{}) error

	// Datastore
	Datastore() *datastore.Datastore
	RunInTransaction(fn func() error, opts ...*datastore.TransactionOptions) error

	// Query
	Query() *ModelQuery

	// Various helpers
	Zero() Entity
	Clone() Entity
	CloneFromJSON() Entity
	Slice() interface{}
	JSON() []byte
	JSONString() string
}

// Model is a mixin which adds Datastore/Validation/Serialization methods to
// any Kind that it has been embedded in.
type Model struct {
	Db     *datastore.Datastore `json:"-" datastore:"-"`
	Entity Kind                 `json:"-" datastore:"-"`
	Parent datastore.Key        `json:"-" datastore:"-"`
	Mock   bool                 `json:"-" datastore:"-"`

	key datastore.Key

	// Set by our mixin
	Id_       string    `json:"id,omitempty"`
	Loaded_   bool      `json:"-" datastore:"-"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
	Deleted   bool      `json:"deleted,omitempty"`

	// Flag used to specify that we're using a string key for this kind
	UseStringKey bool `json:"-" datastore:"-"`
}

// Helper to prevent duplicate deserialization
func (m *Model) Loaded() bool {
	if m.Loaded_ {
		return true
	}
	m.Loaded_ = true
	return false
}

// Wire up model
func (m *Model) Init(db *datastore.Datastore, entity Kind) {
	m.Db = db
	m.Entity = entity
}

// Get appengine.Context
func (m *Model) Context() appengine.Context {
	return m.Db.Context
}

// Set entity on mixin so it can be referenced later
func (m *Model) SetEntity(entity interface{}) {
	m.Entity = entity.(Kind)
}

// Set appengine.Context
func (m *Model) SetContext(ctx interface{}) {
	if m.Db == nil {
		m.Db = datastore.New(ctx)
	} else {
		m.Db.SetContext(ctx)
	}
}

// Set appengine.Context namespace
func (m *Model) SetNamespace(namespace string) {
	ctx, err := appengine.Namespace(m.Context(), namespace)
	if err != nil {
		panic(err)
	}

	m.SetContext(ctx)
}

// Returns namespace for this model
func (m *Model) Namespace() string {
	return m.Key().Namespace()
}

// Return Kind
func (m Model) Kind() string {
	return m.Entity.Kind()
}

// Returns ID for model
func (m *Model) Id() string {
	if m.Id_ == "" {
		// Create a new key
		m.Key()
	}
	return m.Id_
}

// Helper to set Id_ correctly
func (m *Model) setId() {
	if m.UseStringKey {
		m.Id_ = m.key.StringID()
	} else {
		m.Id_ = hashid.EncodeKey(m.Db.Context, m.key)
	}
}

// Helper to update key and id
func (m *Model) setKey(key datastore.Key) {
	m.key = key
	m.setId()
}

// Set's key for entity.
func (m *Model) SetKey(key interface{}) (err error) {
	var k datastore.Key
	var id string

	switch v := key.(type) {
	case datastore.Key:
		k = v
	case string:
		if m.UseStringKey {
			// We've declared this model uses string keys.
			k = m.Db.NewKey(m.Entity.Kind(), v, 0, m.Parent)
		} else {
			// Try to decode key as hashid
			k, err = hashid.DecodeKey(m.Db.Context, v)
			if err != nil {
				// Try to decode key as encoded key
				k, err = m.Db.DecodeKey(v)
				if err != nil {
					return fmt.Errorf("Unable to decode %v, %v", v, err)
				}
			} else {
				id = v
			}
		}
	case int64:
		k = m.Db.NewKey(m.Entity.Kind(), "", v, nil)
	case int:
		k = m.Db.NewKey(m.Entity.Kind(), "", int64(v), nil)
	case nil:
		k = m.Key()
	case reflect.Value:
		return m.SetKey(v.Interface())
	default:
		return fmt.Errorf("Unable to set %v as key", key)
	}

	// Make sure this is a valid key for this kind of entity
	if k.Kind() != m.Kind() {
		return fmt.Errorf("Not a valid key for kind %v: %v", m.Kind(), k)
	}

	// Bail out if already set with same key
	if m.key != nil && m.key.Equal(k.(*aeds.Key)) {
		return nil
	}

	// Set key
	m.key = k

	// Update id
	if id != "" {
		m.Id_ = id
	} else {
		m.setId()
	}

	return nil
}

// Returns Key for this entity
func (m *Model) Key() (key datastore.Key) {
	// Return key if we've already allocated or set one
	if m.key != nil {
		return m.key
	}

	// Regenerate key from Id_ if it exists
	if id := m.Id_; id != "" {
		if err := m.SetKey(m.Id_); err != nil {
			panic(errors.New("Failed to decode ID"))
		}
		return m.key
	}

	// Create new key
	kind := m.Kind()

	if m.UseStringKey {
		// Id_ will unfortunately not be set first time around...
		m.key = m.Db.NewIncompleteKey(kind, m.Parent)
	} else {
		m.key = m.Db.AllocateKey(kind, m.Parent)
	}

	// Update ID
	m.setId()

	return m.key
}

// Create a new key for this object
func (m *Model) NewKey() datastore.Key {
	kind := m.Kind()

	if m.key == nil {
		m.key = m.Db.NewIncompleteKey(kind, m.Parent)
		return m.key
	}

	// intid := m.Db.AllocateId(kind)
	intid := m.key.IntID()
	stringid := m.key.StringID()

	key := m.Db.NewKey(kind, stringid, intid, m.Parent)
	m.setKey(key)
	return key
}

// Put entity in datastore
func (m *Model) MustPut() {
	err := m.Put()
	if err != nil {
		panic(err)
	}
}

// Put entity in datastore
func (m *Model) Put() error {
	// Set CreatedAt, UpdatedAt
	now := time.Now()
	if m.key == nil || timeutil.IsZero(m.CreatedAt) {
		m.CreatedAt = now
	}
	m.UpdatedAt = now

	if m.Mock { // Need mock Put
		return m.mockPut()
	}

	// Put entity into datastore
	key, err := m.Db.Put(m.Key(), m.Entity)
	if err != nil {
		return err
	}

	// Update key
	if m.key == nil {
		m.setKey(key)
	}

	// Errors are ignored
	m.PutDocument()

	return nil
}

func (m *Model) PutWithoutSideEffects() error {
	if m.Mock { // Need mock Put
		return m.mockPut()
	}

	// Put entity into datastore
	key, err := m.Db.Put(m.Key(), m.Entity)
	if err != nil {
		return err
	}

	// Update key
	if m.key == nil {
		m.setKey(key)
	}

	// Errors are ignored
	m.PutDocument()

	return nil
}

// Create new entity (should not exist yet)
func (m *Model) Create() error {
	// Execute BeforeCreate hook if defined on entity.
	if hook, ok := m.Entity.(BeforeCreate); ok {
		if err := hook.BeforeCreate(); err != nil {
			return err
		}
	}

	if err := m.Put(); err != nil {
		return err
	}

	// Execute AfterCreate hook if defined on entity.
	if hook, ok := m.Entity.(AfterCreate); ok {
		if err := hook.AfterCreate(); err != nil {
			return err
		}
	}

	return nil
}

// Create new entity or panic
func (m *Model) MustCreate() {
	err := m.Create()
	if err != nil {
		panic(err)
	}
}

// Get entity from datastore
func (m *Model) Get(args ...interface{}) error {
	// If a key is specified, try to use that, ignore nil keys (which would
	// otherwise create a new incomplete key which makes no sense in this case.
	if len(args) == 1 && args[0] != nil {
		if err := m.SetKey(args[0]); err != nil {
			return err
		}
	}
	return m.Db.Get(m.key, m.Entity)
}

// Get or panic
func (m *Model) MustGet(args ...interface{}) {
	err := m.Get(args...)
	if err != nil {
		panic(err)
	}
}

// Helper that will retrieve entity by id (which may be an encoded key/slug/sku)
func (m *Model) GetById(id string) error {
	ok, err := m.Query().ById(id)
	if err != nil {
		return err
	}

	if !ok {
		return datastore.ErrNoSuchEntity
	}
	return nil
}

// Check if entity is in datastore.
func (m *Model) Exists() (bool, error) {
	return m.Query().KeyExists(m.Key())
}

// Check if entity is in datastore.
func (m *Model) IdExists(id string) (datastore.Key, bool, error) {
	return m.Query().IdExists(id)
}

// Check if entity is in datastore.
func (m *Model) KeyExists(key datastore.Key) (bool, error) {
	return m.Query().KeyExists(key)
}

// Update new entity (should already exist)
func (m *Model) Update() error {
	// Cache results of m.Clone() call in case it's needed in both hooks
	prev := cache.Once(m.Clone)

	// Execute BeforeUpdate hook if defined on entity.
	if hook, ok := getHook("BeforeUpdate", m.Entity); ok {
		if err := callHook(m.Entity, hook, prev()); err != nil {
			return err
		}
	}

	if err := m.Put(); err != nil {
		return err
	}

	// Execute AfterUpdate hook if defined on entity.
	if hook, ok := getHook("AfterUpdate", m.Entity); ok {
		if err := callHook(m.Entity, hook, prev()); err != nil {
			return err
		}
	}

	return nil
}

// Update new entity or panic
func (m *Model) MustUpdate() {
	err := m.Update()
	if err != nil {
		log.Panic(err)
	}
}

// Delete entity from Datastore
func (m *Model) Delete() error {
	if m.Mock { // Need mock Delete
		return m.mockDelete()
	}

	// Execute BeforeDelete hook if defined on entity.
	if hook, ok := m.Entity.(BeforeDelete); ok {
		if err := hook.BeforeDelete(); err != nil {
			return err
		}
	}

	// Errors are ignored
	m.DeleteDocument()

	if err := m.Db.Delete(m.key); err != nil {
		return err
	}

	// Execute AfterDelete hook if defined on entity.
	if hook, ok := m.Entity.(AfterDelete); ok {
		if err := hook.AfterDelete(); err != nil {
			return err
		}
	}

	return nil
}

// Delete or panic
func (m *Model) MustDelete() {
	err := m.Delete()
	if err != nil {
		panic(err)
	}
}

// Get entity from datastore or create new one
func (m *Model) GetOrCreate(filterStr string, value interface{}) error {
	ok, err := m.Query().Filter(filterStr, value).Get()
	if err != nil {
		return err
	}

	// Not found, save entity
	if !ok {
		return m.Create()
	}

	return nil
}

// Get entity from datastore or create new one
func (m *Model) GetOrUpdate(filterStr string, value interface{}) error {
	// Save reference to updated state of entity
	update := m.Clone()

	// Fetch whatever is in datastore
	ok, err := m.Query().Filter(filterStr, value).Get()
	if err != nil {
		return err
	}

	// Not found, create
	if !ok {
		return m.Create()
	}

	// Update fetched entity
	structs.Copy(update, m.Entity)

	// Persist
	return m.Update()
}

// Return datastore
func (m *Model) Datastore() *datastore.Datastore {
	return m.Db
}

// Run in transaction using model's current context
func (m *Model) RunInTransaction(fn func() error, opts ...*datastore.TransactionOptions) error {
	return datastore.RunInTransaction(m.Context(), func(db *datastore.Datastore) error {
		return fn()
	}, opts...)
}

// Mock methods for test keys. Does everything against datastore except create/update/delete/allocate ids.
func (m *Model) mockKey() datastore.Key {
	if m.UseStringKey {
		return m.Db.NewKey(m.Kind(), rand.ShortId(), 0, m.Parent)
	}
	return m.Db.NewKey(m.Kind(), "", rand.Int64(), m.Parent)
}

func (m *Model) mockPut() error {
	// set key, id
	if m.key == nil {
		m.setKey(m.mockKey())
	}
	return nil
}

func (m *Model) mockDelete() error {
	return nil
}
