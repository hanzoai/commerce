package user

import (
	"hanzo.io/datastore"
	. "hanzo.io/models"
	"hanzo.io/util/fake"
)

func Fake(db *datastore.Datastore) *User {
	usr := New(db)
	usr.Username = fake.Username()
	usr.FirstName = fake.FirstName()
	usr.LastName = fake.LastName()
	usr.BillingAddress = Address{Line1: fake.Street(), City: fake.City(), State: fake.State(), PostalCode: fake.Zip(), Country: "US"}
	usr.ShippingAddress = Address{Line1: fake.Street(), City: fake.City(), State: fake.State(), PostalCode: fake.Zip(), Country: "US"}
	usr.Email = fake.EmailAddress()
	usr.Enabled = true
	return usr
}
