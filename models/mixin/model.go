package mixin

import (
	"reflect"
	"strconv"
	"time"

	"crowdstart.io/datastore"
	"crowdstart.io/util/json"
)

// Discrete instance of an entity
type Entity interface {
	Kind() string
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
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Flag used to specify that we're using a string key for this kind
	StringKey_ bool `json:"-" datastore:"-"`
}

// Returns a new Model with minimum configuration.
func NewModel(db *datastore.Datastore, entity Entity) Model {
	return Model{Db: db, Entity: entity}
}

// Helper to set Id_ correctly
func (m *Model) setId() {
	// Set ID to StringID first, if that is not set, then try the IntID A
	// Datastore key can be either an int or string but not both
	m.Id_ = m.key.StringID()
	if m.Id_ == "" {
		if id := m.key.IntID(); id != 0 {
			m.Id_ = strconv.Itoa(int(id))
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
			// By default all keys are int ids, use atoi to convert to an int.
			i, err := strconv.Atoi(v)
			if err != nil {
				return datastore.InvalidKey
			}
			k = m.Db.NewKey(m.Entity.Kind(), "", int64(i), nil)
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
	// Set CreatedAt, UpdatedAt
	now := time.Now()
	if m.key == nil {
		m.CreatedAt = now
	}
	m.UpdatedAt = now

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
		m.SetKey(args[0])
	}

	return m.Db.Get(m.key, m.Entity)
}

// Delete entity from Datastore
func (m *Model) Delete() error {
	return m.Db.Delete(m.key)
}

// Return a query for this entity kind
func (m *Model) Query() datastore.Query {
	return m.Db.Query2(m.Entity.Kind())
}

// Serialize entity to JSON string
func (m *Model) JSON() string {
	return json.Encode(m.Entity)
}
