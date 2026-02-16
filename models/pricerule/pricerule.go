package pricerule

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
)

type PriceRule struct {
	mixin.Model

	PriceId  string `json:"priceId"`
	Attribute string `json:"attribute"`
	Value     string `json:"value"`
	Operator  string `json:"operator"`
	Priority  int    `json:"priority"`
}

func (p *PriceRule) Load(ps []datastore.Property) (err error) {
	p.Defaults()

	if err = datastore.LoadStruct(p, ps); err != nil {
		return err
	}

	return err
}

func (p *PriceRule) Save() ([]datastore.Property, error) {
	return datastore.SaveStruct(p)
}
