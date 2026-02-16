package price

import "github.com/hanzoai/commerce/datastore"

var kind = "price"

func (p Price) Kind() string {
	return kind
}

func (p *Price) Init(db *datastore.Datastore) {
	p.Model.Init(db, p)
}

func (p *Price) Defaults() {
}

func New(db *datastore.Datastore) *Price {
	t := new(Price)
	t.Init(db)
	t.Defaults()
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
