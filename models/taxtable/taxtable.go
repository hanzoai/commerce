package taxtable

import (
	aeds "appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/json"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type TaxRate struct {
	Percent     float64        `json:"percent,omitempty"`
	Flat        currency.Cents `json:"flat,omitempty"`
	City        string         `json:"city,omitempty"`
	State       string         `json:"state,omitempty"`
	PostalCode  string         `json:"postalCode,omitempty"`
	CountryCode string         `json:"country,omitempty"`
}

type TaxTable struct {
	mixin.Model

	Rates  []TaxRate `json:"rates" datastore:"-"`
	Rates_ string    `json:"-"`
}

func (t *TaxTable) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	t.Defaults()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(t, c)); err != nil {
		return err
	}

	if len(t.Rates_) > 0 {
		err = json.DecodeBytes([]byte(t.Rates_), &t.Rates)
	}

	return err
}

func (t *TaxTable) Save(c chan<- aeds.Property) (err error) {
	t.Rates_ = string(json.EncodeBytes(t.Rates))

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(t, c))
}
