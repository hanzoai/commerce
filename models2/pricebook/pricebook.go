package pricebook

import (
	"crowdstart.io/datastore"

	"crowdstart.io/models/mixin"
	"crowdstart.io/models2/types/currency"
)

type Price struct {
	//Ids to filter on
	ProductId    string `json:"productId"`
	VariantId    string `json:"variantId"`
	CollectionId string `json:"collectionId"`

	Price     currency.Cents `json:"price"`
	Shipping  currency.Cents `json:"shipping"`
	Tax       currency.Cents `json:"tax"`
	Inclusive bool           `json:"inclusive"`
}

type PriceBook struct {
	mixin.Model

	Currency currency.Type `json:"currency"`
	Prices   []Price       `json:"prices"`
}

func (p *PriceBook) Init() {
	p.Prices = make([]Price, 0)
}

func New(db *datastore.Datastore) *PriceBook {
	p := new(PriceBook)
	p.Init()
	p.Model = mixin.Model{Db: db, Entity: p}
	return p
}
