package mixin

import (
	"reflect"
	"time"

	"appengine"
	aeds "appengine/datastore"

	"crowdstart.io/datastore"
	"crowdstart.io/util/hashid"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
	"crowdstart.io/util/structs"
)

// Discrete instance of an entity
type Entity interface {
	Kind() string
}

// Interface representing Model
type model interface {
	Key() (key datastore.Key)
	Id() string
	Put() error
	MustPut()
	Get(args ...interface{}) error
	MustGet(args ...interface{})
	Delete(args ...interface{}) error
	Query() *Query
	JSON() string
}

// Model is a datastore mixin which adds serialization to/from Datastore as
// well as a few useful fields and extra methods (such as for JSON
// serialization).
type Model struct {
	Db     *datastore.Datastore `json:"-" datastore:"-"`
	Entity Entity               `json:"-" datastore:"-"`

	key datastore.Key

	// Set by our mixin
	Id_       string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Flag used to specify that we're using a string key for this kind
	StringKey_ bool `json:"-" datastore:"-"`
}

// Set's the appengine context to whatev
func (m *Model) SetContext(ctx interface{}) {
	m.Db = datastore.New(ctx)

	// Update key if necessary
	if m.key != nil {
		m.setKey(m.Db.NewKey(m.Kind(), m.key.StringID(), m.key.IntID(), nil))
	}
}

// Return kind of entity
func (m Model) Kind() string {
	return m.Entity.Kind()
}

// Helper to set Id_ correctly
func (m *Model) setId() {
	key := m.Key()

	// Set ID to StringID first, if that is not set, then try the IntID A
	// Datastore key can be either an int or string but not both
	m.Id_ = key.StringID()
	if m.Id_ == "" {
		if id := key.IntID(); id != 0 {
			m.Id_ = hashid.EncodeId(id)
		}
	}
}

// Helper to set key + Id_
func (m *Model) setKey(key datastore.Key) {
	m.key = key
	m.setId()
}

// Returns Key for this entity
func (m *Model) Key() (key datastore.Key) {
	// Create a new incomplete key for this new entity
	if m.key == nil {
		log.Warn("Key is nil, automatically creating a new key.")
		kind := m.Entity.Kind()

		if m.StringKey_ {
			// Id_ will unfortunately not be set first time around...
			m.key = m.Db.NewIncompleteKey(kind, nil)
		} else {
			// We can allocate an id in advance and ensure that Id_ is populated
			id := m.Db.AllocateId(kind)
			m.setKey(m.Db.NewKey(kind, "", id, nil))
		}
	}

	return m.key
}

// Returns string key for entity
func (m *Model) Id() string {
	if m.Id_ == "" {
		m.setId()
	}
	return m.Id_
}

// Set's key for entity.
func (m *Model) SetKey(key interface{}) error {
	var k datastore.Key

	switch v := key.(type) {
	case datastore.Key:
		k = v
	case string:
		if m.StringKey_ {
			// We've declared this model uses string keys.
			k = m.Db.NewKey(m.Entity.Kind(), v, 0, nil)
		} else {
			// By default all keys are int ids internally (but we use hashid to convert them to strings)
			k = m.Db.NewKey(m.Entity.Kind(), "", hashid.DecodeId(v), nil)
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

	// Set key, update Id_, etc.
	m.setKey(k)

	return nil
}

// Put entity in datastore
func (m *Model) Put() error {
	return m.PutEntity(m.Entity)
}

// Put entity in datastore
func (m *Model) MustPut() {
	err := m.Put()
	if err != nil {
		panic(err)
	}
}

func (m *Model) PutEntity(entity interface{}) error {
	// Set CreatedAt, UpdatedAt
	now := time.Now()
	if m.key == nil {
		m.CreatedAt = now
	}
	m.UpdatedAt = now

	// Put entity into datastore
	key, err := m.Db.Put(m.Key(), entity)

	// Update key
	m.setKey(key)

	return err
}

// Get entity from datastore
func (m *Model) Get(args ...interface{}) error {
	// If a key is specified, try to use that, ignore nil keys (which would
	// otherwise create a new incomplete key which makes no sense in this case.
	if len(args) == 1 && args[0] != nil {
		m.SetKey(args[0])
	}

	return m.Db.Get(m.key, m.Entity)
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
	if err != nil && err != aeds.ErrNoSuchEntity {
		return err
	}

	if !ok {
		// Not found, save entity
		m.Put()
	}

	return nil
}

// Get entity from datastore or create new one
func (m *Model) GetOrUpdate(filterStr string, value interface{}) error {
	entity := reflect.ValueOf(m.Entity).Interface()

	q := datastore.NewQuery(m.Kind(), m.Db)
	key, ok, err := q.Filter(filterStr, value).First(entity)

	// Something bad happened
	if err != nil && err != aeds.ErrNoSuchEntity {
		return err
	}

	if ok {
		// Update copy found with our new data, use it's key, and save updated entity
		structs.Copy(m.Entity, entity)
		m.Entity = entity.(Entity)
		m.SetKey(key)
		m.Put()
	} else {
		// Nothing found, save entity
		m.Put()
	}

	return nil
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

// Get entity in datastore
func (m *Model) GetEntity(entity interface{}) error {
	return m.Db.Get(m.key, entity)
}

// Delete entity from Datastore
func (m *Model) Delete(args ...interface{}) error {
	// If a key is specified, try to use that, ignore nil keys (which would
	// otherwise create a new incomplete key which makes no sense in this case.
	if len(args) == 1 && args[0] != nil {
		m.SetKey(args[0])
	}
	return m.Db.Delete(m.key)
}

// Return a query for this entity kind
func (m *Model) Query() *Query {
	return &Query{m.Db.Query2(m.Entity.Kind()), m}
}

// Serialize entity to JSON string
func (m *Model) JSON() string {
	return json.Encode(m.Entity)
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

func (q *Query) First() (bool, error) {
	key, ok, err := q.Query.First(q.model.Entity)
	if ok {
		q.model.setKey(key)
	}
	return ok, err
}
