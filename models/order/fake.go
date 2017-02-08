package order

import (
	"hanzo.io/datastore"
	"hanzo.io/models/lineitem"
	"hanzo.io/models/payment"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/fake"

	. "hanzo.io/models"
)

func Fake(db *datastore.Datastore, lis ...lineitem.LineItem) *Order {
	linetotal := 0
	for _, v := range lis {
		linetotal += int(v.Price) * v.Quantity
	}
	o := New(db)
	o.Email = fake.EmailAddress()
	o.Items = lis
	o.Status = Open
	o.PaymentStatus = payment.Unpaid
	o.Currency = currency.USD
	o.LineTotal = currency.Cents(linetotal)
	o.Subtotal = o.LineTotal
	o.Total = o.Subtotal + o.Shipping + o.Tax
	o.Balance = o.Total
	o.Company = fake.Company()
	o.BillingAddress = Address{Line1: fake.Street(), City: fake.City(), State: fake.State(), PostalCode: fake.Zip(), Country: "US"}
	o.ShippingAddress = Address{Line1: fake.Street(), City: fake.City(), State: fake.State(), PostalCode: fake.Zip(), Country: "US"}

	return o
}
