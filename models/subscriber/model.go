package subscriber

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"

	. "crowdstart.com/models"
)

func (s Subscriber) Kind() string {
	return "subscriber"
}

func (s *Subscriber) Defaults() {
	s.Metadata = make(Map)
}

func (s *Subscriber) Init(db *datastore.Datastore) {
	s.Model = mixin.Model{Db: db, Entity: s}
}

func New(db *datastore.Datastore) *Subscriber {
	return new(Subscriber).New(db).(*Subscriber)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
