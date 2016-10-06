package variant

import (
	"math/rand"

	"crowdstart.com/datastore"
	"crowdstart.com/util/fake"
)

func Fake(db *datastore.Datastore, productId string) *Variant {
	v := New(db)
	v.ProductId = productId
	v.SKU = fake.SKU()
	v.Name = fake.Word()
	v.Available = true
	v.Inventory = rand.Intn(400)
	v.Sold = rand.Intn(400)
	v.Taxable = false

	return v
}
