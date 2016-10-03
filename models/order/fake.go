package order

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/fake"

	. "crowdstart.com/models"
)

func Fake(db *datastore.Datastore) *Order {
	o := New(db)
	o.Email = fake.EmailAddress()
	o.Status = Open
	o.PaymentStatus = payment.Unpaid
	o.Preorder = fake.Bool
	o.Unconfirmed = fake.Bool
	o.Currency = currency.USD
	o.LineTotal = currency.Cents(0).Fake()
	o.Discount = currency.Cents(0).FakeN(990)
	o.Subtotal = o.LineTotal - o.Discount
	o.Shipping = currency.Cents(0).FakeN(990)
	o.Tax = currency.Cents(0).FakeN(990)
	o.Total = o.Subtotal + o.Shipping + o.Tax
	o.Balance = o.Total
	o.Paid = currency.Cents(0)
	o.Refunded = currency.Cents(0)
	o.Company = fake.Company()
	o.BillingAddress = Address{Line1: fake.Street(), City: fake.City(), State: fake.State(), PostalCode: fake.Zip(), Country: "US"}
	o.ShippingAddress = Address{Line1: fake.Street(), City: fake.City(), State: fake.State(), PostalCode: fake.Zip(), Country: "US"}
	o.Gift = fake.Bool
	o.GiftMessage = fake.Sentence()
	o.GiftEmail = fake.EmailAddress()

	return o
}
