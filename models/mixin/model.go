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

// Model is our datastore mixin which adds serialization to/from Datastore,
// JSON, etc.
type Model struct {
	Id_       string    `json:"id" datastore:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Flag used to specify that we're using a string key for this kind
	StringKey_ bool `json:"-" datastore:"-"`

	Entity Entity `json:"-" datastore:"-"`
	key    datastore.Key
	db     *datastore.Datastore
}

func (m *Model) Key() (key datastore.Key) {
	// Create a new incomplete key for this new entity
	if m.key == nil {
		m.key = m.db.NewIncompleteKey(m.Entity.Kind(), nil)
	}

	return m.key
}

func (m *Model) setKey(key datastore.Key) {
	// Set ID to StringID first, if that is not set, then try the IntID
	// A Datastore key can be either an int or string but not both
	m.key = key
	m.Id_ = key.StringID()
	if m.Id_ == "" {
		if id := key.IntID(); id != 0 {
			m.Id_ = strconv.Itoa(int(id))
		}
	}
}

func (m *Model) SetKey(key interface{}) error {
	var k datastore.Key

	switch v := key.(type) {
	case datastore.Key:
		k = v
	case string:
		if m.StringKey_ {
			// We've declared this model uses string keys.
			k = m.db.NewKey(m.Entity.Kind(), v, 0, nil)
		} else {
			// By default all keys are int ids, use atoi to convert to an int.
			i, err := strconv.Atoi(v)
			if err != nil {
				return datastore.InvalidKey
			}
			k = m.db.NewKey(m.Entity.Kind(), "", int64(i), nil)
		}
	case int64:
		k = m.db.NewKey(m.Entity.Kind(), "", v, nil)
	case int:
		k = m.db.NewKey(m.Entity.Kind(), "", int64(v), nil)
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

func (m *Model) Put() error {
	// Set CreatedAt, UpdatedAt
	now := time.Now()
	if m.key == nil {
		m.CreatedAt = now
	}
	m.UpdatedAt = now

	// Put entity into datastore
	key, err := m.db.Put(m.Key(), m.Entity)

	// Update key
	m.setKey(key)
	return err
}

func (m *Model) Get(args ...interface{}) error {
	// If a key is specified, try to use that, ignore nil keys (which would
	// otherwise create a new incomplete key which makes no sense in this case.
	if len(args) == 1 && args[0] != nil {
		m.SetKey(args[0])
	}

	return m.db.Get(m.key, m.Entity)
}

func (m *Model) Query() datastore.Query {
	return m.db.Query2(m.Entity.Kind())
}

// Return JSON representation of model
func (m *Model) JSON() string {
	return json.Encode(&m)
}

// Use NewModel inside of a model implementing entity
func NewModel(db *datastore.Datastore, e Entity) *Model {
	m := new(Model)
	m.db = db
	m.Entity = e
	return m
}
