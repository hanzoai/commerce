package price

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json"

	. "github.com/hanzoai/commerce/types"
)

type Price struct {
	mixin.Model

	PriceSetId   string         `json:"priceSetId"`
	CurrencyCode string         `json:"currencyCode"`
	Amount       currency.Cents `json:"amount"`
	MinQuantity  int            `json:"minQuantity"`
	MaxQuantity  int            `json:"maxQuantity"`
	PriceListId  string         `json:"priceListId"`

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (p *Price) Load(ps []datastore.Property) (err error) {
	p.Defaults()

	if err = datastore.LoadStruct(p, ps); err != nil {
		return err
	}

	if len(p.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(p.Metadata_), &p.Metadata)
	}

	return err
}

func (p *Price) Save() ([]datastore.Property, error) {
	p.Metadata_ = string(json.EncodeBytes(&p.Metadata))

	return datastore.SaveStruct(p)
}
