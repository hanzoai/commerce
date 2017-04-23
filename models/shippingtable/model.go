package shippingtable

import (
	"hanzo.io/datastore"
)

var kind = "shippingtable"

func (o ShippingTable) Kind() string {
	return kind
}

func (s *ShippingTable) Init(db *datastore.Datastore) {
	s.Model.Init(db, s)
}

func (s *ShippingTable) Defaults() {
	s.Rates = make([]ShippingRate, 0)
}

func New(db *datastore.Datastore) *ShippingTable {
	s := new(ShippingTable)
	s.Init(db)
	s.Defaults()
	return s
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
