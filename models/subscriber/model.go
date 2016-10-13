package subscriber

import (
	"crowdstart.com/datastore"

	. "crowdstart.com/models"
)

var kind = "subscriber"

func (s Subscriber) Kind() string {
	return kind
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
	s.Defaults()
	return s
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
