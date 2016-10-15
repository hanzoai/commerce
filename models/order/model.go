package order

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/coupon"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/payment"

	. "crowdstart.com/models"
	"crowdstart.com/models/lineitem"
)

func (o Order) Kind() string {
	return "order"
}

func (o *Order) Init(db *datastore.Datastore) {
	o.Model.Init(db, o)
}

func (o *Order) Defaults() {
	o.Status = Open
	o.PaymentStatus = payment.Unpaid
	o.FulfillmentStatus = FulfillmentUnfulfilled
	o.Adjustments = make([]Adjustment, 0)
	o.History = make([]Event, 0)
	o.Items = make([]lineitem.LineItem, 0)
	o.Metadata = make(Map)
	o.Coupons = make([]coupon.Coupon, 0)
}

func New(db *datastore.Datastore) *Order {
	o := new(Order)
	o.Defaults()
	o.Init(db)
	return o
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
