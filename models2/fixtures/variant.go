package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models2/variant"
)

func Variant(c *gin.Context) []*variant.Variant {
	// Get namespaced db
	db := getDb(c)

	// Get a product
	prod := Product(c)
	prod.Slug = "t-shirt"
	prod.GetOrCreate("Slug=", prod.Slug)

	v := variant.New(db)
	v.SKU = "T-SHIRT-M"
	v.GetOrCreate("SKU=", v.SKU)
	v.ProductId = prod.Id()
	v.Options = []variant.Option{variant.Option{Name: "Size", Value: "Much"}}
	v.ProductId = prod.Id()
	v.Put()

	v2 := variant.New(db)
	v2.SKU = "T-SHIRT-M"
	v2.GetOrCreate("SKU=", v2.SKU)
	v2.ProductId = prod.Id()
	v2.Options = []variant.Option{variant.Option{Name: "Size", Value: "Wow"}}
	v2.ProductId = prod.Id()
	v2.Put()

	prod.Variants = []*variant.Variant{v, v2}
	prod.Put()

	return []*variant.Variant{v, v2}
}
