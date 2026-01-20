package store

import (
	"github.com/hanzoai/commerce/datastore"
	. "github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/fake"
	"github.com/hanzoai/commerce/util/slug"
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
