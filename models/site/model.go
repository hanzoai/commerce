package site

import "crowdstart.com/datastore"

var kind = "site"

func (s Site) Kind() string {
	return kind
}

func (s *Site) Init(db *datastore.Datastore) {
	s.Model.Init(db, s)
}

func New(db *datastore.Datastore) *Site {
	s := new(Site)
	s.Init(db)
	return s
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
