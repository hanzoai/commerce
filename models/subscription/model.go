package subscription

import (
	"hanzo.io/datastore"
)

var kind = "subscription"

func (s Subscription) Kind() string {
	return kind
}

func (s *Subscription) Init(db *datastore.Datastore) {
	s.Model.Init(db, s)
}

func (s *Subscription) Defaults() {
	if s != nil {
		s.Metadata = make(map[string]interface{})
	}
}

func New(db *datastore.Datastore) *Subscription {
	s := new(Subscription)
	s.Init(db)
	s.Defaults()
	return s
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
