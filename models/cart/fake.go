package cart

import (
	"crowdstart.com/datastore"
	. "crowdstart.com/models"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/fake"
)

func Fake(db *datastore.Datastore, userId string) *Cart {
	c := New(db)
	c.UserId = userId
	c.Email = fake.EmailAddress()
	c.Status = Active
	c.Currency = currency.USD
	c.LineTotal = currency.Cents(0).Fake()
	c.Discount = currency.Cents(0).FakeN(990)
	c.Subtotal = c.LineTotal - c.Discount
	c.Shipping = currency.Cents(0).FakeN(990)
	c.Tax = currency.Cents(0).FakeN(990)
	c.Total = c.Subtotal + c.Tax + c.Shipping
	c.Company = fake.Company()
	c.BillingAddress = Address{Line1: fake.Street(), City: fake.City(), State: fake.State(), PostalCode: fake.Zip(), Country: "US"}
	c.ShippingAddress = Address{Line1: fake.Street(), City: fake.City(), State: fake.State(), PostalCode: fake.Zip(), Country: "US"}

	return c
}
