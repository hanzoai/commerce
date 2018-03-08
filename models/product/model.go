package product

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/variant"
)

func (p Product) Kind() string {
	return "product"
}

func (p *Product) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func (p *Product) Defaults() {
	p.Variants = make([]*variant.Variant, 0)
	p.Options = make([]*Option, 0)
}

func New(db *datastore.Datastore) *Product {
	p := new(Product)
	p.Init(db)
	return p
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
