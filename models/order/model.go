package order

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (o Order) Kind() string {
	return "order"
}

func (o *Order) Init(db *datastore.Datastore) {
	o.Model.Init(db, o)
}

func New(db *datastore.Datastore) *Order {
	o := new(Order)
	o.Init(db)
	return o
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
