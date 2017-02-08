package collection

import (
	"hanzo.io/datastore"
	"hanzo.io/util/fake"
	"hanzo.io/util/slug"
)

func Fake(db *datastore.Datastore, itemIdType string, itemIds ...string) *Collection {
	coll := New(db)
	coll.Name = fake.ProductName()
	coll.Description = fake.Sentences(3)
	coll.Slug = slug.Slugify(coll.Name)
	coll.Available = true
	if itemIdType == "product" {
		coll.ProductIds = itemIds
	} else {
		coll.VariantIds = itemIds
	}
	return coll
}
