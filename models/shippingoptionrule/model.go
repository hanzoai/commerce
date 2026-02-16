package shippingoptionrule

import (
	"github.com/hanzoai/commerce/datastore"
)

var kind = "shippingoptionrule"

func (r ShippingOptionRule) Kind() string {
	return kind
}

func (r *ShippingOptionRule) Init(db *datastore.Datastore) {
	r.Model.Init(db, r)
}

func (r *ShippingOptionRule) Defaults() {
}

func New(db *datastore.Datastore) *ShippingOptionRule {
	r := new(ShippingOptionRule)
	r.Init(db)
	r.Defaults()
	return r
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
