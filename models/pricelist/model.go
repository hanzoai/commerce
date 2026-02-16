package pricelist

import "github.com/hanzoai/commerce/datastore"

var kind = "pricelist"

func (p PriceList) Kind() string {
	return kind
}

func (p *PriceList) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func (p *PriceList) Defaults() {
	if p.Status == "" {
		p.Status = "draft"
	}
}

func New(db *datastore.Datastore) *PriceList {
	t := new(PriceList)
	t.Init(db)
	t.Defaults()
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
