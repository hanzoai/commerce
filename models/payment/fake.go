package payment

import (
	"hanzo.io/datastore"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/types/accounts"
	"hanzo.io/util/fake"

	. "hanzo.io/models"
)

func Fake(db *datastore.Datastore) *Payment {
	pay := New(db)
	pay.Amount = currency.Cents(0).Fake()
	pay.Account.Type = accounts.NullType
	pay.Buyer = Buyer{
		Email:     fake.EmailAddress(),
		FirstName: fake.FirstName(),
		LastName:  fake.LastName(),
		BillingAddress: Address{
			Line1:      fake.Street(),
			City:       fake.City(),
			State:      fake.State(),
			PostalCode: fake.Zip(),
			Country:    "US",
		},
		ShippingAddress: Address{
			Line1:      fake.Street(),
			City:       fake.City(),
			State:      fake.State(),
			PostalCode: fake.Zip(),
			Country:    "US",
		},
	}
	pay.Status = Unpaid
	pay.Currency = currency.USD

	pay.Account.Number = "4242424242424242"
	pay.Account.CVC = "424"
	pay.Account.Month = 12
	pay.Account.Year = 2024

	return pay
}
