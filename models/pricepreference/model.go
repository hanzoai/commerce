package pricepreference

import "github.com/hanzoai/commerce/datastore"

var kind = "pricepreference"

func (p PricePreference) Kind() string {
	return kind
}

func (p *PricePreference) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func (p *PricePreference) Defaults() {
}

func New(db *datastore.Datastore) *PricePreference {
	t := new(PricePreference)
	t.Init(db)
	t.Defaults()
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
