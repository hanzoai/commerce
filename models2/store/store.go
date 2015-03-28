package store

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
)

type Store struct {
	mixin.Model

	// Full name of store
	Name string `json:"name"`

	// Unique human readable id for url <slug>.crowdstart.come
	Slug string `json:"slug"`

	//
}

func (s *Store) Init() {
}

func New(db *datastore.Datastore) *Store {
	s := new(Store)
	s.Init()
	s.Model = mixin.Model{Db: db, Entity: s}
	return s
}
