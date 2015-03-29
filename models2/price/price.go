package pricebook

import (
	"crowdstart.io/datastore"

	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/types/currency"
)

type Price struct {
	mixin.Model

	StoreId string `json:"storeId"`

	//Ids to filter on
	ProductId    string `json:"productId"`
	VariantId    string `json:"variantId"`
	CollectionId string `json:"collectionId"`

	Price    currency.Cents `json:"price"`
	Currency currency.Type  `json:"currency"`
}

func (p *Price) Init() {
}

func New(db *datastore.Datastore) *Price {
	p := new(Price)
	p.Model = mixin.Model{Db: db, Entity: p}
	return p
}
