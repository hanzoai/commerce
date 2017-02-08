package site

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (s Site) Kind() string {
	return "site"
}

func (s *Site) Init(db *datastore.Datastore) {
	s.Model.Init(db, s)
}

func New(db *datastore.Datastore) *Site {
	s := new(Site)
	s.Init(db)
	return s
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
