package store

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	. "crowdstart.io/models2"
	"crowdstart.io/models2/types/currency"
)

type Store struct {
	mixin.Model

	// Full name of store
	Name string `json:"name"`

	// Unique human readable id for url <slug>.crowdstart.come
	Slug string `json:"slug"`

	// Default unit of currency set in UI for store admin
	DefaultCurrencyType currency.Type

	// Taxation information
	TaxNexus []Address

	// Shipping Table
	ShippingTable map[string]float64
}

func (s *Store) Init() {
}

func New(db *datastore.Datastore) *Store {
	s := new(Store)
	s.Init()
	s.Model = mixin.Model{Db: db, Entity: s}
	return s
}
