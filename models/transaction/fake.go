package transaction

import (
	"hanzo.io/models/types/currency"
	"math/rand"

	"hanzo.io/datastore"
	"hanzo.io/util/fake"
)

func Fake(db *datastore.Datastore) *Transaction {
	t := New(db)
	t.DestinationId = fake.Id()
	t.DestinationKind = "User"
	t.SourceId = fake.Id()
	t.Test = true
	t.Type = "deposit"
	t.Amount = currency.Cents(rand.Intn(10000))
	t.Currency = currency.Fake()
	return t
}
