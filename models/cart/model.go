package cart

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/coupon"
	"crowdstart.com/models/mixin"

	. "crowdstart.com/models"
	. "crowdstart.com/models/lineitem"
)

func (o Cart) Kind() string {
	return "cart"
}

func (o *Cart) Init(db *datastore.Datastore) {
	o.Model.Init(db, o)
}

func (o *Cart) Defaults() {
	o.Items = make([]LineItem, 0)
	o.Metadata = make(Map)
	o.Coupons = make([]coupon.Coupon, 0)
}

func New(db *datastore.Datastore) *Cart {
	o := new(Cart)
	o.Init(db)
	o.Defaults()
	return o
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
