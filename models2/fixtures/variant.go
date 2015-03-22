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

	v := variant.New(db)
	v.SKU = "T-SHIRT-M"
	v.GetOrCreate("SKU=", v.SKU)
	v.Options = []variant.Option{variant.Option{Name: "Size", Value: "Much"}}
	v.ProductId = prod.Id()
	if err := v.Put(); err != nil {
		panic(err)
	}

	v2 := variant.New(db)
	v2.SKU = "T-SHIRT-M"
	v2.GetOrCreate("SKU=", v2.SKU)
	v2.Options = []variant.Option{variant.Option{Name: "Size", Value: "Wow"}}
	v2.ProductId = prod.Id()
	if err := v.Put(); err != nil {
		panic(err)
	}

	return []*variant.Variant{v, v2}
}
