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
	s.Listings = make(Listings)
	s.ShippingRateTable = make(ShippingRateTable)
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
