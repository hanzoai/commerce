package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/product"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/variant"
)

var Variant = New("variant", func(c *context.Context) *variant.Variant {
	// Get namespaced db
	db := getNamespaceDb(c)

	// Get a product
	prod := Product(c).(*product.Product)

	v := variant.New(db)
	v.Parent = prod.Key()
	v.SKU = "T-SHIRT-M"
	v.GetOrCreate("SKU=", v.SKU)
	v.ProductId = prod.Id()
	v.Options = []variant.Option{variant.Option{Name: "Size", Value: "Much"}}
	v.ProductId = prod.Id()
	v.Price = 2000
	v.Currency = currency.USD
	v.MustPut()

	v2 := variant.New(db)
	v2.Parent = prod.Key()
	v2.SKU = "T-SHIRT-W"
	v2.GetOrCreate("SKU=", v2.SKU)
	v2.ProductId = prod.Id()
	v2.Options = []variant.Option{variant.Option{Name: "Size", Value: "Wow"}}
	v2.ProductId = prod.Id()
	v2.Price = 2000
	v2.Currency = currency.USD
	v2.MustPut()

	prod.Variants = []*variant.Variant{v, v2}
	prod.Put()

	return v
})
