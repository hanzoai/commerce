package coupon

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
)

func (c Coupon) Kind() string {
	return "coupon"
}

func (c *Coupon) Init(db *datastore.Datastore) {
	c.Model.Init(db, c)
}

func (c *Coupon) Defaults() {
	c.Enabled = true
	// c.Buyers = make([]string, 0)
}

func New(db *datastore.Datastore) *Coupon {
	c := new(Coupon)
	c.Init(db)
	return c
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
