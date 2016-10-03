package store

import (
	"crowdstart.com/datastore"
	. "crowdstart.com/models"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/fake"
	"crowdstart.com/util/slug"
)

func Fake(db *datastore.Datastore) *Store {
	s := New(db)
	s.Name = fake.Company()
	s.Slug = slug.Slugify(s.Name)
	s.Domain = s.Name + ".com"
	s.Prefix = fake.Word()
	s.Currency = currency.USD
	s.Address = Address{Line1: fake.Street(), City: fake.City(), State: fake.State(), PostalCode: fake.Zip(), Country: "US"}
	s.Email = fake.EmailAddress()

	return s
}
