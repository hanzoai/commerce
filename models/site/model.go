package site

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (s Site) Kind() string {
	return "site"
}

func (s *Site) Init(db *datastore.Datastore) {
	s.Model = mixin.Model{Db: db, Entity: s}
}

func New(db *datastore.Datastore) *Site {
	return new(Site).New(db).(*Site)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
