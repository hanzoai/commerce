package fulfillmentset

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

type FulfillmentSet struct {
	mixin.Model

	Name string `json:"name"`
	Type string `json:"type"` // "shipping", "pickup", "digital"

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (f *FulfillmentSet) Load(ps []datastore.Property) (err error) {
	f.Defaults()

	if err = datastore.LoadStruct(f, ps); err != nil {
		return err
	}

	if len(f.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(f.Metadata_), &f.Metadata)
	}

	return err
}

func (f *FulfillmentSet) Save() ([]datastore.Property, error) {
	f.Metadata_ = string(json.EncodeBytes(&f.Metadata))

	return datastore.SaveStruct(f)
}
