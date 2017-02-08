package store

import (
	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/util/json"
)

func (s *Store) Load(properties []aeds.Property) (err error) {
	// Ensure we're initialized
	s.Defaults()

	// Load supported properties
	err = datastore.LoadStruct(s, properties)
	if err != nil {
		return err
	}

	// Deserialize from datastore
	if len(s.Listings_) > 0 {
		err = json.DecodeBytes([]byte(s.Listings_), &s.Listings)
	}

	if len(s.ShippingRateTable_) > 0 {
		err = json.DecodeBytes([]byte(s.ShippingRateTable_), &s.ShippingRateTable)
	}

	return err
}

func (s *Store) Save() ([]aeds.Property, error) {
	// Serialize unsupported properties
	s.Listings_ = string(json.EncodeBytes(&s.Listings))
	s.ShippingRateTable_ = string(json.EncodeBytes(&s.ShippingRateTable))

	// Save properties
	return datastore.SaveStruct(s)
}
