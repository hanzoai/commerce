package tokensale

import (
	"hanzo.io/datastore"
	"hanzo.io/util/fake"
)

func Fake(db *datastore.Datastore) *TokenSale {
	ts := New(db)
	ts.Name = fake.ProductName()
	ts.TotalTokens = fake.Number(100000)

	return ts
}
