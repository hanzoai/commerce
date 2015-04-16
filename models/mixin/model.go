package mixin

import (
	"reflect"
	"strings"
	"time"

	"appengine"
	aeds "appengine/datastore"

	"crowdstart.io/datastore"
	"crowdstart.io/util/hashid"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/rand"
	"crowdstart.io/util/structs"
	"crowdstart.io/util/val"
)

var zeroTime = time.Time{}

// A datastore kind that is compatible with the Model mixin
type Kind interface {
	Kind() string
}

// A specific datastore entity, with methods inherited from this mixin
type Entity interface {
	Kind
	Context() appengine.Context
	SetContext(ctx interface{})
	SetNamespace(namespace string)
	Key() (key datastore.Key)
	SetKey(key interface{}) (err error)
	Id() string
	Get(args ...interface{}) error
	GetById(string) error
	KeyExists(key interface{}) (datastore.Key, error)
	MustGet(args ...interface{})
	Put() error
	MustPut()
	GetOrCreate(filterStr string, value interface{}) error
	GetOrUpdate(filterStr string, value interface{}) error
	RunInTransaction(fn func() error) error
	Delete(args ...interface{}) error
	Query() *Query
	Validate() error
	Validator() *val.Validator
	JSON() string
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

	// Flag used to specify that we're using a string key for this kind
	StringKey_ bool `json:"-" datastore:"-"`
}

// Get AppEngine context
func (m *Model) Context() appengine.Context {
	return m.Db.Context
}

// Set AppEngine Context
func (m *Model) SetContext(ctx interface{}) {
	// Update context
	m.Db = datastore.New(ctx)

	// Update key if necessary
	if m.key != nil {
		m.setKey(m.Db.NewKey(m.Kind(), m.key.StringID(), m.key.IntID(), m.Parent))
	}
}

// Set's the appengine context to whatev
func (m *Model) SetNamespace(namespace string) {
	ctx, err := appengine.Namespace(m.Db.Context, namespace)
	if err != nil {
		panic(err)
	}

	// Update context
	m.Db.Context = ctx

	// Update key if necessary
	if m.key != nil {
		m.setKey(m.Db.NewKey(m.Kind(), m.key.StringID(), m.key.IntID(), m.Parent))
	}
}

// Return kind of entity
func (m Model) Kind() string {
	return m.Entity.Kind()
}

// Helper to set Id_ correctly
func (m *Model) setId() {
	key := m.Key()

	if m.StringKey_ {
		m.Id_ = key.StringID()
	} else {
		m.Id_ = hashid.EncodeKey(key)
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
		if m.StringKey_ {
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
		log.Warn("Key is nil, automatically creating a new key.")
		kind := m.Entity.Kind()

		if m.StringKey_ {
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
	if m.key == nil || m.CreatedAt == zeroTime {
		m.CreatedAt = now
	}
	m.UpdatedAt = now

	if m.Mock { // Need mock Put
		return m.mockPut()
	}

	// Put entity into datastore
	key, err := m.Db.Put(m.Key(), m.Entity)

	// Update key
	m.setKey(key)

	return err
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

// Helper that will retrieve entity by id (which may be an encoded key/slug/sku)
func (m *Model) GetById(id string) error {
	// Try to decode key
	key, err := hashid.DecodeKey(m.Db.Context, id)

	// Use key if we have one
	if err == nil {
		return m.Get(key)
	}

	// Set err to nil and try to use filter
	err = nil
	filterStr := ""

	// Use unique filter based on model type
	switch m.Kind() {
	case "store", "product":
		filterStr = "Slug"
	case "variant":
		filterStr = "SKU"
	case "coupon":
		filterStr = "Code"
	case "organization":
		filterStr = "Name"
	case "user":
		filterStr = "Username"
	default:
		return datastore.InvalidKey
	}

	// Try and fetch by filterStr
	_, err = m.Query().Filter(filterStr+"=", id).First()
	return err
}

// Get's key only (ensures key is good)
func (m *Model) KeyExists(key interface{}) (datastore.Key, error) {
	// If a key is specified, try to use that, ignore nil keys (which would
	// otherwise create a new incomplete key which makes no sense in this case.
	if key != nil {
		if err := m.SetKey(key); err != nil {
			return nil, err
		}
	}

	keys, err := m.Query().Filter("__key__=", m.key).KeysOnly().GetAll(nil)
	// Something bad happened
	if err != nil {
		return nil, err
	}

	// We couldn't find it
	if len(keys) != 1 {
		return nil, datastore.KeyNotFound
	}

	m.SetKey(keys[0])
	return keys[0], nil
}

func (m *Model) MustGet(args ...interface{}) {
	err := m.Get(args...)
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
		name := strings.TrimSpace(strings.Split(filterStr, "=")[0])
		field := reflect.Indirect(reflect.ValueOf(m.Entity)).FieldByName(name)
		field.Set(reflect.ValueOf(value))

		return m.Put()
	}

	return nil
}

// Get entity from datastore or create new one
func (m *Model) GetOrUpdate(filterStr string, value interface{}) error {
	entity := reflect.ValueOf(m.Entity).Interface()

	q := datastore.NewQuery(m.Kind(), m.Db)
	key, ok, err := q.Filter(filterStr, value).First(entity)

	// Something bad happened
	if err != nil {
		return err
	}

	if !ok {
		name := strings.TrimSpace(strings.Split(filterStr, "=")[0])
		field := reflect.Indirect(reflect.ValueOf(m.Entity)).FieldByName(name)
		field.Set(reflect.ValueOf(value))
		return m.Put()
	}

	// Update copy found with our new data, use it's key, and save updated entity
	structs.Copy(m.Entity, entity)
	m.Entity = entity.(Entity)
	m.SetKey(key)
	return m.Put()
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

// Delete entity from Datastore
func (m *Model) Delete(args ...interface{}) error {
	if m.Mock { // Need mock Delete
		return m.mockDelete()
	}

	// If a key is specified, try to use that, ignore nil keys (which would
	// otherwise create a new incomplete key which makes no sense in this case.
	if len(args) == 1 && args[0] != nil {
		if err := m.SetKey(args[0]); err != nil {
			return err
		}
	}
	return m.Db.Delete(m.key)
}

// Return a query for this entity kind
func (m *Model) Query() *Query {
	return &Query{m.Db.Query2(m.Entity.Kind()), m}
}

// Validate a model
func (m *Model) Validator() *val.Validator {
	return val.New(nil)
}

func (m *Model) Validate() error {
	// val := m.Entity.Validator()
	// errs := val.Check(m).Errors()
	// if len(errs)

	// err := val.NewError("Failed to validate " + m.Kind())
	// err.Fields = errs
	// return err
	return nil
}

// Serialize entity to JSON string
func (m *Model) JSON() string {
	return json.Encode(m.Entity)
}

// Mock methods for test keys. Does everything against datastore except create/update/delete/allocate ids.
func (m *Model) mockKey() datastore.Key {
	if m.StringKey_ {
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

// Wrap Query so we don't need to pass in entity to First() and key is updated
// properly.
type Query struct {
	datastore.Query
	model *Model
}

func (q *Query) Limit(limit int) *Query {
	q.Query = q.Query.Limit(limit)
	return q
}

func (q *Query) Offset(offset int) *Query {
	q.Query = q.Query.Offset(offset)
	return q
}

func (q *Query) Filter(filterStr string, value interface{}) *Query {
	q.Query = q.Query.Filter(filterStr, value)
	return q
}

func (q *Query) KeysOnly() *Query {
	q.Query = q.Query.KeysOnly()
	return q
}

func (q *Query) First() (bool, error) {
	key, ok, err := q.Query.First(q.model.Entity)
	if ok {
		q.model.setKey(key)
	}
	return ok, err
}
