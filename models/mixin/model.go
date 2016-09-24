package mixin

import (
	"reflect"
	"strings"
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

var (
	zeroTime = time.Time{}
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
	SetKey(key interface{}) (err error)
	NewKey() datastore.Key
	Id() string

	// Various existential helpers
	Exists() (bool, error)
	IdExists(id string) (bool, error)
	KeyById(string) (datastore.Key, bool, error)
	KeyExists(key interface{}) (datastore.Key, bool, error)

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
	RunInTransaction(fn func() error) error

	// Query
	Query() *Query

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
	Id_       string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Deleted   bool      `json:"deleted,omitempty"`

	// Flag used to specify that we're using a string key for this kind
	UseStringKey bool `json:"-" datastore:"-"`
}

// Wire up model
func (m *Model) Init(db *datastore.Datastore, kind Kind) {
	m.Db = db
	m.Entity = kind

	// Automatically call defaults on init
	if m.CreatedAt == zeroTime {
		if hook, ok := (m.Entity).(Defaults); ok {
			hook.Defaults()
		}
	}
}

// Get AppEngine context
func (m *Model) Context() appengine.Context {
	return m.Db.Context
}

// Set AppEngine Context
func (m *Model) SetContext(ctx interface{}) {
	// Update context
	m.Db.SetContext(ctx)

	// Update key if necessary
	if m.key != nil {
		m.NewKey()
	}
}

// Set's the appengine context to whatev
func (m *Model) SetNamespace(namespace string) {
	ctx, err := appengine.Namespace(m.Db.Context, namespace)
	if err != nil {
		panic(err)
	}

	m.SetContext(ctx)
}

func (m *Model) Namespace() string {
	return m.Key().Namespace()
}

// Return kind of entity
func (m Model) Kind() string {
	return m.Entity.Kind()
}

// Helper to set Id_ correctly
func (m *Model) setId() {
	key := m.Key()

	if m.UseStringKey {
		m.Id_ = key.StringID()
	} else {
		m.Id_ = hashid.EncodeKey(m.Db.Context, key)
	}
}

// Returns string key for entity
func (m *Model) Id() string {
	if m.Id_ == "" {
		m.setId()
	}
	return m.Id_
}

// Helper to set key + Id_
func (m *Model) setKey(key datastore.Key) {
	m.key = key
	m.setId()
}

// Set's key for entity.
func (m *Model) SetKey(key interface{}) (err error) {
	var k datastore.Key

	switch v := key.(type) {
	case datastore.Key:
		k = v
	case string:
		if m.UseStringKey {
			// We've declared this model uses string keys.
			k = m.Db.NewKey(m.Entity.Kind(), v, 0, m.Parent)
		} else {
			// By default all keys are int ids internally (but we use hashid to convert them to strings)
			k, err = hashid.DecodeKey(m.Db.Context, v)
			if err != nil {
				return datastore.InvalidKey
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
		return datastore.InvalidKey
	}

	// Make sure this is a valid key for this kind of entity
	if k.Kind() != m.Kind() {
		return datastore.InvalidKey
	}

	// Set key, update Id_, etc.
	m.setKey(k)

	return nil
}

// Returns Key for this entity
func (m *Model) Key() (key datastore.Key) {
	// Create a new incomplete key for this new entity
	if m.key == nil {
		kind := m.Entity.Kind()

		if m.UseStringKey {
			// Id_ will unfortunately not be set first time around...
			m.key = m.Db.NewIncompleteKey(kind, m.Parent)
		} else {
			// We can allocate an id in advance and ensure that Id_ is populated
			id := m.Db.AllocateId(kind)
			m.setKey(m.Db.NewKey(kind, "", id, m.Parent))
		}
	}

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
	m.setKey(key)

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
	m.setKey(key)

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
	_, _, err := m.KeyById(id)
	return err
}

// Check if entity is in datastore.
func (m *Model) Exists() (bool, error) {
	_, ok, err := m.KeyExists(nil)
	return ok, err
}

// Check if key is in datastore.
func (m *Model) IdExists(id string) (bool, error) {
	_, ok, err := m.KeyById(id)
	return ok, err
}

func (m *Model) KeyById(id string) (datastore.Key, bool, error) {
	// Try to decode key
	key, err := hashid.DecodeKey(m.Db.Context, id)

	// Use key if we have one
	if err == nil {
		err = m.Get(key)
		return m.Key(), err != nil, err
	}

	// Set err to nil and try to use filter
	err = nil
	filterStr := ""

	// Use unique filter based on model type
	switch m.Kind() {
	case "store", "product", "collection":
		filterStr = "Slug"
	case "variant":
		filterStr = "SKU"
	case "organization", "mailinglist":
		filterStr = "Name"
	case "aggregate":
		filterStr = "Instance"
	case "site":
		filterStr = "Name"
	case "user":
		if strings.Contains(id, "@") {
			filterStr = "Email"
		} else {
			filterStr = "Username"
		}
	case "coupon":
		return couponFromId(m, id)
	case "order":
		return orderFromId(m, id)
	default:
		return nil, false, datastore.InvalidKey
	}

	// Try and fetch by filterStr
	ok, err := m.Query().Filter(filterStr+"=", id).First()
	if !ok {
		return nil, false, datastore.KeyNotFound
	}

	return m.Key(), true, nil
}

// Get's key only (ensures key is good)
func (m *Model) KeyExists(key interface{}) (datastore.Key, bool, error) {
	// If a key is specified, try to use that, ignore nil keys (which would
	// otherwise create a new incomplete key which makes no sense in this case.
	if key != nil {
		if err := m.SetKey(key); err != nil {
			return nil, false, err
		}
	}

	keys, err := m.Query().Filter("__key__=", m.key).GetKeys()
	// Something bad happened
	if err != nil {
		return nil, false, err
	}

	// We couldn't find it
	if len(keys) != 1 {
		return nil, false, datastore.KeyNotFound
	}

	m.SetKey(keys[0])
	return keys[0], true, nil
}

// Update new entity (should already exist)
func (m *Model) Update() error {
	// Get previous entity
	getPrevious := cache.Once(m.Clone)

	// Execute BeforeUpdate hook if defined on entity.
	method, ok := getHook("BeforeUpdate", m)
	if ok {
		previous := getPrevious()
		err := callHook(m.Entity, method, previous)
		if err != nil {
			return err
		}
	}

	if err := m.Put(); err != nil {
		return err
	}

	// Execute AfterUpdate hook if defined on entity.
	method, ok = getHook("AfterUpdate", m)
	if ok {
		previous := getPrevious()
		err := callHook(m.Entity, method, previous)
		if err != nil {
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
	ok, err := m.Query().Filter(filterStr, value).First()

	// Something bad happened
	if err != nil {
		return err
	}

	// Not found, save entity
	if !ok {
		// What were we filtering on? Make sure the field is set to value of
		// filter. This prevents any duplicate attempts from creating new
		// models as well.

		// name := strings.TrimSpace(strings.Split(filterStr, "=")[0])
		// field := reflect.Indirect(reflect.ValueOf(m.Entity)).FieldByName(name)
		// field.Set(reflect.ValueOf(value))

		return m.Create()
	}

	return nil
}

// Get entity from datastore or create new one
func (m *Model) GetOrUpdate(filterStr string, value interface{}) error {
	entity := reflect.ValueOf(m.Entity).Interface()

	q := m.Db.Query(m.Kind())
	key, ok, err := q.Filter(filterStr, value).First(entity)

	// Something bad happened
	if err != nil {
		return err
	}

	// Not found create
	if !ok {
		name := strings.TrimSpace(strings.Split(filterStr, "=")[0])
		field := reflect.Indirect(reflect.ValueOf(m.Entity)).FieldByName(name)
		field.Set(reflect.ValueOf(value))
		return m.Create()
	}

	// Update copy found with our new data, use it's key, and save updated entity
	structs.Copy(m.Entity, entity)
	m.Entity = entity.(Entity)
	m.SetKey(key)
	return m.Update()
}

// NOTE: This is not thread-safe
func (m *Model) RunInTransaction(fn func() error) error {
	ctx := m.Db.Context

	err := aeds.RunInTransaction(ctx, func(c appengine.Context) error {
		m.Db.Context = c
		return fn()
	}, &aeds.TransactionOptions{XG: true})

	// Should I set old context back?
	m.Db.Context = ctx

	return err
}

// Return Datastore
func (m *Model) Datastore() *datastore.Datastore {
	return m.Db
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
	m.setKey(m.mockKey())
	return nil
}

func (m *Model) mockDelete() error {
	return nil
}
