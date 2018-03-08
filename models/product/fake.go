package product

import (
	"hanzo.io/datastore"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/fake"
	"hanzo.io/util/slug"
)

func Fake(db *datastore.Datastore) *Product {
	prod := New(db)
	prod.Name = fake.ProductName()
	prod.Headline = fake.Sentence()
	prod.Description = prod.Headline + " " + fake.Sentences(3)
	prod.Slug = slug.Slugify(prod.Name)
	prod.Currency = currency.USD
	prod.Price = currency.Cents(0).Fake()
	prod.Shipping = currency.Cents(0).FakeN(990)
	prod.ListPrice = prod.Price * 2
	return prod
}
