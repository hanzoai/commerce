package fixtures

import (
	"log"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/organization"
	"crowdstart.io/models2/product"
	"crowdstart.io/models2/variant"
	"crowdstart.io/util/task"
)

func createVariants(db *datastore.Datastore, prod *product.Product) []*variant.Variant {
	v := variant.New(db)
	v.SKU = "T-SHIRT-M"
	v.Options = []variant.Option{variant.Option{Name: "Size", Value: "Much"}}
	v.ProductId = prod.Id()

	v2 := variant.New(db)
	v2.SKU = "T-SHIRT-W"
	v2.Options = []variant.Option{variant.Option{Name: "Size", Value: "Wow"}}
	v2.ProductId = prod.Id()

	return []*variant.Variant{v, v2}
}

var _ = task.Func("models2-fixtures-variant", func(c *gin.Context) {
	db := datastore.New(c)

	org := organization.New(db)
	org.Name = "suchtees"
	org.GetOrCreate("Name=", org.Name)

	// Use org's namespace
	// Use org's namespace
	if ctx, err := org.Namespace(c); err != nil {
		log.Panic("Failed to get namespace: %v", err)
	} else {
		db = datastore.New(ctx)
	}

	prod := product.New(db)
	prod.GetOrCreate("Slug=", "t-shirt")

	for _, variant := range createVariants(db, prod) {
		variant.Put()
	}
})
