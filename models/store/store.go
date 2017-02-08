package store

import (
	"reflect"

	"hanzo.io/models/mixin"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/types/shipping"
	"hanzo.io/util/log"
	"hanzo.io/util/structs"

	. "hanzo.io/models"
)

var ListingFields = structs.FieldNames(Listing{})

type Listings map[string]Listing
type ShippingRateTable map[string]shipping.Rates

type Store struct {
	mixin.Model

	// Full name of store
	Name string `json:"name"`

	// Unique human readable id for url <slug>.hanzo.ioe
	Slug string `json:"slug"`

	// Where this is hosted if not on hanzo.io
	Hostname string `json:"hostname"`
	Prefix   string `json:"prefix"`

	// Currency for store
	Currency currency.Type `json:"currency"`

	// Taxation information
	TaxNexus []Address `json:"taxNexus"`

	// Shipping Rate Table, country name to shipping rate
	ShippingRateTable  ShippingRateTable `json:"shippingRates" datastore:"-"`
	ShippingRateTable_ string            `json:"-" datastore:",noindex"`

	// Overrides per item
	Listings  Listings `json:"listings" datastore:"-"`
	Listings_ string   `json:"-" datastore:",noindex"`

	Salesforce struct {
		PriceBookId string `json:"PriceBookId"`
	} `json:"-"`
}

func (s *Store) Defaults() {
	s.ShippingRateTable = make(ShippingRateTable)
	s.Listings = make(Listings)
}

// Add a new listing to the listings map
func (s *Store) AddListing(id string, listing Listing) {
	listing.Currency = s.Currency
	s.Listings[id] = listing
}

// Update product/variant using listing for said item
func (s *Store) UpdateFromListing(entity mixin.Entity) {
	// Check if we have a listing for this product/variant
	listing, ok := s.Listings[entity.Id()]
	if !ok {
		log.Warn("No listing found that matches given %s", entity.Kind())
		return
	}

	ev := reflect.Indirect(reflect.ValueOf(entity))
	lv := reflect.ValueOf(listing)

	// Loop over listing fields and set any that this listing has that are non-nil
	for _, name := range ListingFields {
		field := ev.FieldByName(name)
		val := reflect.Indirect(lv.FieldByName(name))
		if val.IsValid() && field.IsValid() {
			field.Set(val)
		}
	}

	// Ensure currency is set to currency of store
	field := ev.FieldByName("Currency")
	field.Set(reflect.ValueOf(s.Currency))
}
