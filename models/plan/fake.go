package plan

import (
	"hanzo.io/datastore"
	"hanzo.io/util/fake"
	"hanzo.io/models/types/currency"
)

func Fake(db *datastore.Datastore) *Plan {
	p := New(db)
	p.Slug = fake.Slug()
	p.SKU = fake.SKU()
	p.StripeId = "p_" + fake.Id()
	p.Name = fake.Word()
	p.Description = fake.Sentence()
	p.Price = currency.Cents(0).Fake()
	p.Currency = currency.USD
	p.Interval = Monthly
	return p
}
