package subscription

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (s Subscription) Kind() string {
	return "subscription"
}

func (s *Subscription) Init(db *datastore.Datastore) {
	s.Model.Init(db, s)
}

func New(db *datastore.Datastore) *Subscription {
	s := new(Subscription)
	s.Init(db)
	return s
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
