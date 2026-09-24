package collection

import (
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/util/fake"
	"github.com/hanzoai/commerce/util/slug"
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
