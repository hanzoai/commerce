package store

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/types/currency"
)

var kind = "store"

func (s Store) Kind() string {
	return kind
}

func (s *Store) Init(db *datastore.Datastore) {
	s.Model.Init(db, s)
}

func (s *Store) Defaults() {
	s.Listings = make(Listings)
	// s.ShippingRateTable = make(ShippingRateTable)
	s.Currency = currency.USD
}

func New(db *datastore.Datastore) *Store {
	s := new(Store)
	s.Init(db)
	s.Defaults()
	return s
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
