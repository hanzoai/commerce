package shippingprofile

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

type ShippingProfile struct {
	mixin.Model

	Name string `json:"name"`
	Type string `json:"type"` // "default", "gift_card", "custom"

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (s *ShippingProfile) Load(ps []datastore.Property) (err error) {
	s.Defaults()

	if err = datastore.LoadStruct(s, ps); err != nil {
		return err
	}

	if len(s.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(s.Metadata_), &s.Metadata)
	}

	return err
}

func (s *ShippingProfile) Save() ([]datastore.Property, error) {
	s.Metadata_ = string(json.EncodeBytes(&s.Metadata))

	return datastore.SaveStruct(s)
}
