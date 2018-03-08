package coupon

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
)

func (c Coupon) Kind() string {
	return "coupon"
}

func (c *Coupon) Init(db *datastore.Datastore) {
	c.Model.Init(db, c)
}

func New(db *datastore.Datastore) *Coupon {
	c := new(Coupon)
	c.Init(db)
	return c
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
