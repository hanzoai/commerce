package taxrates

import (
	"github.com/hanzoai/commerce/datastore"
)

var kind = "taxrates"

func (t TaxRates) Kind() string {
	return kind
}

func (t *TaxRates) Init(db *datastore.Datastore) {
	t.Model.Init(db, t)
}

func (t *TaxRates) Defaults() {
	t.GeoRates = make([]GeoRate, 0)
}

func New(db *datastore.Datastore) *TaxRates {
	t := new(TaxRates)
	t.Init(db)
	t.Defaults()
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
