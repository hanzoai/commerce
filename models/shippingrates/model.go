package shippingrates

import "hanzo.io/datastore"

var kind = "shippingrates"

func (t ShippingRates) Kind() string {
	return kind
}

func (t *ShippingRates) Init(db *datastore.Datastore) {
	t.Model.Init(db, t)
}

func (t *ShippingRates) Defaults() {
}

func New(db *datastore.Datastore) *ShippingRates {
	t := new(ShippingRates)
	t.Init(db)
	t.Defaults()
	return t
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
