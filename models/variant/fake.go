package variant

import (
	"math/rand"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/fake"
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
