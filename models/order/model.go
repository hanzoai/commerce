package order

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/coupon"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/types/fulfillment"

	"github.com/hanzoai/commerce/models/lineitem"
	. "github.com/hanzoai/commerce/types"
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
	o.Notifications.Email.Enabled = true
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
