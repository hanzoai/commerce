package stocklocation

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type StockLocation struct {
	mixin.Model

	// Name of stock location
	Name string `json:"name"`

	// Address fields
	AddressLine1 string `json:"addressLine1"`
	AddressLine2 string `json:"addressLine2"`
	City         string `json:"city"`
	Country      string `json:"country"`
	Province     string `json:"province"`
	PostalCode   string `json:"postalCode"`
	Phone        string `json:"phone"`

	// Arbitrary metadata
	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (s *StockLocation) Load(ps []datastore.Property) (err error) {
	s.Defaults()

	if err = datastore.LoadStruct(s, ps); err != nil {
		return err
	}

	if len(s.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(s.Metadata_), &s.Metadata)
	}

	return err
}

func (s *StockLocation) Save() ([]datastore.Property, error) {
	s.Metadata_ = string(json.EncodeBytes(&s.Metadata))

	return datastore.SaveStruct(s)
}
