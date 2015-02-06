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
	Put() error
	Get() error
}

type model struct {
	Entity
	key datastore.Key
}

func (m *model) Key() (key datastore.Key, err error) {
	return key, nil
}

func (m *model) Put() error {
	return nil
}

func (m *model) Get() error {
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
