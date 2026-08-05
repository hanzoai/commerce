package fixtures

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/collection"
)

var Collection = New("collection", func(c *zip.Ctx) *collection.Collection {
	db := getNamespaceDb(c)

	collection := collection.New(db)
	collection.Slug = "such-tees-pack"
	collection.GetOrCreate("Slug=", collection.Slug)
	collection.Name = "Such tees pack"
	collection.Description = "Much tees in one pack!"
	collection.Published = true
	collection.Available = true

	collection.MustPut()
	return collection
})
