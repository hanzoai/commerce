package product

import (
	"hanzo.io/datastore"
	"hanzo.io/models/variant"
)

var kind = "product"

func (p Product) Kind() string {
	return kind
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
	p.Defaults()
	return p
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
