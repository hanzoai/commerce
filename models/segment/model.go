package segment

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (s Segment) Kind() string {
	return "segment"
}

func (s *Segment) Init(db *datastore.Datastore) {
	s.Model.Init(db, s)
}

func New(db *datastore.Datastore) *Segment {
	s := new(Segment)
	s.Init(db)
	return s
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
