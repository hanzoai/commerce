package taxtable

import (
	"hanzo.io/datastore"
)

var kind = "taxtable"

func (o TaxTable) Kind() string {
	return kind
}

func (t *TaxTable) Init(db *datastore.Datastore) {
	t.Model.Init(db, t)
}

func (t *TaxTable) Defaults() {
	t.Rates = make([]TaxRate, 0)
}

func New(db *datastore.Datastore) *TaxTable {
	t := new(TaxTable)
	t.Init(db)
	t.Defaults()
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
