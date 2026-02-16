package pricerule

import "github.com/hanzoai/commerce/datastore"

var kind = "pricerule"

func (p PriceRule) Kind() string {
	return kind
}

func (p *PriceRule) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func (p *PriceRule) Defaults() {
}

func New(db *datastore.Datastore) *PriceRule {
	t := new(PriceRule)
	t.Init(db)
	t.Defaults()
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
