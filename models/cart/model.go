package cart

import (
	"hanzo.io/datastore"
	"hanzo.io/models/coupon"
	"hanzo.io/models/lineitem"

	. "hanzo.io/types"
)

var kind = "cart"

func (c Cart) Kind() string {
	return kind
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
	c.Defaults()
	return c
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
