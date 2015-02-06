package models

import (
	"reflect"

	"crowdstart.io/datastore"
)

// Discrete instance of an entity
type Entity interface {
	Kind() string
}

// Mixin methods for models
type Model interface {
	Key() datastore.Key
	SetKey(key interface{}) error
	Put() error
	Get(args ...interface{}) error
}

type model struct {
	Entity
	key datastore.Key
	db  *datastore.Datastore
}

func (m *model) Key() (key datastore.Key) {
	// Create a new incomplete key for this new entity
	if m.key == nil {
		m.key = m.db.NewIncompleteKey(m.Entity.Kind(), nil)
	}

	return m.key
}

func (m *model) SetKey(key interface{}) error {
	switch v := key.(type) {
	case datastore.Key:
		m.key = v
	case string:
		if key, err := m.db.DecodeKey(v); err != nil {
			return err
		} else {
			m.key = key
		}
	case reflect.Value:
		return m.SetKey(v.Interface())
	case nil:
		m.Key()
		return nil
	default:
		return datastore.InvalidKey
	}

	return nil
}

func (m *model) Put() error {
	key, err := m.db.Put(m.Key(), m.Entity)
	m.key = key
	return err
}

func (m *model) Get(args ...interface{}) error {
	var key datastore.Key

	if len(args) == 1 {
		key = args[0].(datastore.Key)
		m.key = key
	} else {
		key = m.key
	}

	m.db.Get(key, m.Entity)

	return nil
}

// Use NewModel inside of a model implementing entity
func NewModel(db *datastore.Datastore, e Entity) *model {
	m := new(model)
	m.db = db
	m.Entity = e
	return m
}

// Usage
type EUser struct {
	Model
	Name string
}

func (u *EUser) Kind() string {
	return "user"
}

func NewEUser(db *datastore.Datastore) *EUser {
	user := new(EUser)
	user.Model = NewModel(db, user)
	// Set any other defaults
	// ...
	return user
}
