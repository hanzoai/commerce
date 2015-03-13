package mixin

import (
	"reflect"
	"strconv"

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
	Id     string `json:"id" datastore:"-"`
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
	m.Id = key.StringID()
	if m.Id == "" {
		if id := key.IntID(); id != 0 {
			m.Id = strconv.Itoa(int(id))
		}
	}
}

func (m *Model) SetKey(key interface{}) error {
	switch v := key.(type) {
	case datastore.Key:
		m.key = v
	case string:
		if key, err := m.db.DecodeKey(v); err != nil {
			return err
		} else {
			m.setKey(key)
		}
	case reflect.Value:
		return m.SetKey(v.Interface())
	case nil:
		m.Key()
	default:
		return datastore.InvalidKey
	}

	return nil
}

func (m *Model) Put() error {
	key, err := m.db.Put(m.Key(), m.Entity)
	m.setKey(key)
	return err
}

func (m *Model) Get(args ...interface{}) error {
	var key datastore.Key

	if len(args) == 1 {
		key = args[0].(datastore.Key)
	} else {
		key = m.key
	}

	err := m.db.Get(key, m.Entity)
	m.setKey(key)

	return err
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
