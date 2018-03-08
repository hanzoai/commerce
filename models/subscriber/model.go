package subscriber

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (s Subscriber) Kind() string {
	return "subscriber"
}

func (s *Subscriber) Init(db *datastore.Datastore) {
	s.Model.Init(db, s)
}

func New(db *datastore.Datastore) *Subscriber {
	s := new(Subscriber)
	s.Init(db)
	return s
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
