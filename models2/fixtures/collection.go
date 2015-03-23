package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models2/collection"
)

func Collection(c *gin.Context) *collection.Collection {
	db := getDb(c)

	collection := collection.New(db)
	collection.Slug = "such-tees-pack"
	collection.Name = "Such tees pack"
	collection.Description = "Much tees in one pack!"
	collection.Published = true

	collection.MustPut()
	return collection
}
