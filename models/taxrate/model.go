package taxrate

import "github.com/hanzoai/commerce/datastore"

var kind = "taxrate"

func (t TaxRate) Kind() string {
	return kind
}

func (t *TaxRate) Init(db *datastore.Datastore) {
	t.Model.Init(db, t)
}

func (t *TaxRate) Defaults() {
}

func New(db *datastore.Datastore) *TaxRate {
	t := new(TaxRate)
	t.Init(db)
	t.Defaults()
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
