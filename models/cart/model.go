package cart

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/coupon"
	"crowdstart.com/models/mixin"

	. "crowdstart.com/models"
	"crowdstart.com/models/lineitem"
)

func (c Cart) Kind() string {
	return "cart"
}

func (c *Cart) Init(db *datastore.Datastore) {
	c.Model.Init(db, c)
}

func (c *Cart) Defaults() {
	c.Items = make([]lineitem.LineItem, 0)
	c.Metadata = make(Map)
	c.Coupons = make([]coupon.Coupon, 0)
	c.Status = Active
}

func New(db *datastore.Datastore) *Cart {
	c := new(Cart)
	c.Init(db)
	return c
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
