package subscriber

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"

	. "crowdstart.com/models"
)

func (s Subscriber) Kind() string {
	return "subscriber"
}

func (s *Subscriber) Init(db *datastore.Datastore) {
	s.Model.Init(db, s)
}

func (s *Subscriber) Defaults() {
	s.Metadata = make(Map)
}

func New(db *datastore.Datastore) *Subscriber {
	s := new(Subscriber)
	s.Init(db)
	return s
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
