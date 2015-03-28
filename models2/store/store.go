package store

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/types/currency"
	"crowdstart.io/models2/types/shipping"

	. "crowdstart.io/models2"
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

	// Shipping Rate Table
	ShippingRateTable map[string]shipping.Rates

	Salesforce struct {
		PriceBookId string `json:"PriceBookId"`
	} `json:"-"`
}

func (s *Store) Init() {
	s.ShippingRateTable = make(map[string]shipping.Rates)
}

func New(db *datastore.Datastore) *Store {
	s := new(Store)
	s.Init()
	s.Model = mixin.Model{Db: db, Entity: s}
	return s
}
