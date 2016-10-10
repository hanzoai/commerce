package lineitem

import (
	"math/rand"

	"crowdstart.com/models/product"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/models/variant"
)

func Fake(item interface{}) LineItem {
	var li LineItem

	switch v := item.(type) {
	case *product.Product:
		li.ProductId = v.Id()
		li.ProductName = v.Name
		li.ProductSlug = v.Slug
	case *variant.Variant:
		li.VariantId = v.Id()
		li.VariantName = v.Name
		li.VariantSKU = v.SKU
	default:
		panic("Unsupported item")
	}

	li.Price = currency.Cents(0).Fake()
	li.Quantity = rand.Intn(5) + 1
	li.Taxable = false
	return li
}
