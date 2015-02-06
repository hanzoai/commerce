package models

import (
	"crowdstart.io/datastore"
)

// Discrete instance of an entity
type Entity interface {
	Kind() string
}

// Mixin methods for models
type Model interface {
	Key() (datastore.Key, error)
	Put(*datastore.Datastore) error
	Get(*datastore.Datastore, ...interface{}) error
}

type model struct {
	Entity
	key datastore.Key
}

func (m *model) Key() (key datastore.Key, err error) {
	return m.key, nil
}

func (m *model) Put(db *datastore.Datastore) error {
	key, err := db.Put(m.Key())
	m.key = key
	return err
}

func (m *model) Get(db *datastore.Datastore, args ...interface{}) error {
	var key datastore.Key

	if len(args) == 1 {
		key = args[0].(datastore.Key)
	} else {
		key = m.key
	}

	db.Get(key, m.Entity)

	return nil
}

func NewModel(e Entity) *model {
	m := new(model)
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

func NewEUser() *EUser {
	user := new(EUser)
	user.Model = NewModel(user)
	return user
}
