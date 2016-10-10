package payment

import (
	"crowdstart.com/datastore"
	"crowdstart.com/util/fake"

	. "crowdstart.com/models"
)

func Fake(db *datastore.Datastore) *Payment {
	pay := New(db)
	pay.Type = Null
	pay.Buyer = Buyer{
		Email:     fake.EmailAddress(),
		FirstName: fake.FirstName(),
		LastName:  fake.LastName(),
		Address: Address{
			Line1:      fake.Street(),
			City:       fake.City(),
			State:      fake.State(),
			PostalCode: fake.Zip(),
			Country:    "US",
		},
	}
	pay.Status = Unpaid
	return pay
}

func FakeStripe(db *datastore.Datastore) *Payment {
	pay := Fake(db)
	pay.Type = Stripe
	pay.Account.Number = "4242424242424242"
	pay.Account.CVC = "424"
	pay.Account.Month = 12
	pay.Account.Year = 2024
	return pay
}
