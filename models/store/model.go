package store

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (s Store) Kind() string {
	return "store"
}

func (s *Store) Init(db *datastore.Datastore) {
	s.Model.Init(db, s)
}

func New(db *datastore.Datastore) *Store {
	s := new(Store)
	s.Init(db)
	return s
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
