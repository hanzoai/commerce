package store

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (s Store) Kind() string {
	return "store"
}

func (s *Store) Init(db *datastore.Datastore) {
	s.Model.Init(db, s)
}

func (s *Store) Defaults() {
	s.ShippingRateTable = make(ShippingRateTable)
	s.Listings = make(Listings)
}

func New(db *datastore.Datastore) *Store {
	s := new(Store)
	s.Init(db)
	s.Defaults()
	return s
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
