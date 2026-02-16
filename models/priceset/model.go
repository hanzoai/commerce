package priceset

import "github.com/hanzoai/commerce/datastore"

var kind = "priceset"

func (p PriceSet) Kind() string {
	return kind
}

func (p *PriceSet) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func (p *PriceSet) Defaults() {
}

func New(db *datastore.Datastore) *PriceSet {
	t := new(PriceSet)
	t.Init(db)
	t.Defaults()
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
