package product

import (
	"crowdstart.com/datastore"
	"crowdstart.com/util/fake"
	"crowdstart.com/util/slug"
)

func Fake(db *datastore.Datastore) *Product {
	prod := New(db)
	prod.Name = fake.ProductName()
	prod.Slug = slug.Slugify(prod.Name)
	return prod
}
