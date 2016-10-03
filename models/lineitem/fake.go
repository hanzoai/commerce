package lineitem

import (
	"math/rand"

	"crowdstart.com/models/types/currency"
)

func Fake(variantId, variantName, variantSKU string) LineItem {
	var li LineItem
	li.VariantId = variantId
	li.VariantName = variantName
	li.VariantSKU = variantSKU
	li.Price = currency.Cents(0).Fake()
	li.Quantity = rand.Intn(5) + 1
	li.Taxable = false
	return li
}
