package site

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

type Site struct {
	mixin.Model
}

func (s *Site) Init() {
}

func New(db *datastore.Datastore) *Site {
	s := new(Site)
	s.Init()
	s.Model = mixin.Model{Db: db, Entity: s}
	return s
}

func (s Site) Kind() string {
	return "site"
}

func (s Site) Document() mixin.Document {
	return &Document{}
}
