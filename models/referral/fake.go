package referral

import (
	"hanzo.io/datastore"
	"hanzo.io/models/types/currency"
)

func Fake(db *datastore.Datastore, userId, orderId string) *Referral {
	r := New(db)
	r.Type = NewOrder
	r.OrderId = orderId
	r.Referrer = Referrer{UserId: userId}
	r.Fee = Fee{Currency: currency.USD, Amount: currency.Cents(0).FakeN(90)}

	return r
}
