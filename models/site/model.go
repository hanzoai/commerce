package site

import "github.com/hanzoai/commerce/datastore"

var kind = "site"

func (s Site) Kind() string {
	return kind
}

func (s *Site) Init(db *datastore.Datastore) {
	s.BaseModel.Init(db, s)
}

func (s *Site) Defaults() {
}

func New(db *datastore.Datastore) *Site {
	s := new(Site)
	s.Init(db)
	s.Defaults()
	return s
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
