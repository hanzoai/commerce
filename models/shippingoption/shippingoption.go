package shippingoption

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

type PriceType string

const (
	FlatRate   PriceType = "flat"
	Calculated PriceType = "calculated"
)

type ShippingOption struct {
	mixin.Model

	Name          string         `json:"name"`
	PriceType     PriceType      `json:"priceType"`
	Amount        currency.Cents `json:"amount"`
	ServiceZoneId string         `json:"serviceZoneId"`
	ProviderId    string         `json:"providerId"`
	ProfileId     string         `json:"profileId"`
	DataJSON      string         `json:"data" datastore:",noindex"`

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (s *ShippingOption) Load(ps []datastore.Property) (err error) {
	s.Defaults()

	if err = datastore.LoadStruct(s, ps); err != nil {
		return err
	}

	if len(s.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(s.Metadata_), &s.Metadata)
	}

	return err
}

func (s *ShippingOption) Save() ([]datastore.Property, error) {
	s.Metadata_ = string(json.EncodeBytes(&s.Metadata))

	return datastore.SaveStruct(s)
}
