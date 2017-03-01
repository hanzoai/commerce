package order

import (
	"hanzo.io/datastore"
	"hanzo.io/models/coupon"
	"hanzo.io/models/payment"
	"hanzo.io/models/types/fulfillment"

	. "hanzo.io/models"
	"hanzo.io/models/lineitem"
)

var kind = "order"

func (o Order) Kind() string {
	return kind
}

func (o *Order) Init(db *datastore.Datastore) {
	o.Model.Init(db, o)
}

func (o *Order) Defaults() {
	o.Status = Open
	o.PaymentStatus = payment.Unpaid
	o.Fulfillment.Status = fulfillment.Pending
	o.Adjustments = make([]Adjustment, 0)
	o.History = make([]Event, 0)
	o.Items = make([]lineitem.LineItem, 0)
	o.Metadata = make(Map)
	o.Coupons = make([]coupon.Coupon, 0)
}

func New(db *datastore.Datastore) *Order {
	o := new(Order)
	o.Init(db)
	o.Defaults()
	return o
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
