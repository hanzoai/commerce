package variant

import (
	"math/rand"

	"hanzo.io/datastore"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/fake"
)

func Fake(db *datastore.Datastore, productId string) *Variant {
	v := New(db)
	v.ProductId = productId
	v.SKU = fake.SKU()
	v.Name = fake.Word()
	v.Available = true
	v.Inventory = rand.Intn(400)
	v.Sold = rand.Intn(400)
	v.Price = currency.Cents(0).Fake()
	v.Taxable = false

	return v
}
