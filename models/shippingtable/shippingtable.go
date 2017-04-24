package shippingtable

import (
	aeds "appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/json"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type ShippingRate struct {
	Percent     float64        `json:"percent,omitempty"`
	Weight      float64        `json:"weight,omitempty`
	Cost        float64        `json:"cost,omitempty`
	Flat        currency.Cents `json:"flat,omitempty"`
	City        string         `json:"city,omitempty"`
	State       string         `json:"state,omitempty"`
	PostalCode  string         `json:"postalCode,omitempty"`
	CountryCode string         `json:"country,omitempty"`
}

type ShippingTable struct {
	mixin.Model

	StoreId string `json:"storeId,omitempty`

	Rates  []ShippingRate `json:"rates" datastore:"-"`
	Rates_ string         `json:"-"`
}

func (s *ShippingTable) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	s.Defaults()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(s, c)); err != nil {
		return err
	}

	if len(s.Rates_) > 0 {
		err = json.DecodeBytes([]byte(s.Rates_), &s.Rates)
	}

	return err
}

func (s *ShippingTable) Save(c chan<- aeds.Property) (err error) {
	s.Rates_ = string(json.EncodeBytes(s.Rates))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(s, c))
}
