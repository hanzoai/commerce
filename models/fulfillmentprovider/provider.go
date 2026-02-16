package fulfillmentprovider

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

type FulfillmentProvider struct {
	mixin.Model

	Name      string `json:"name"`
	IsEnabled bool   `json:"isEnabled"`

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (p *FulfillmentProvider) Load(ps []datastore.Property) (err error) {
	p.Defaults()

	if err = datastore.LoadStruct(p, ps); err != nil {
		return err
	}

	if len(p.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(p.Metadata_), &p.Metadata)
	}

	return err
}

func (p *FulfillmentProvider) Save() ([]datastore.Property, error) {
	p.Metadata_ = string(json.EncodeBytes(&p.Metadata))

	return datastore.SaveStruct(p)
}
