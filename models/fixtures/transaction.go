package fixtures

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
)

var Transaction = New("transaction", func(c *zip.Ctx) *transaction.Transaction {
	// Get namespaced db
	db := getNamespaceDb(c)

	u := User(c)

	tran := transaction.New(db)
	tran.DestinationId = u.Id()
	tran.GetOrCreate("DestinationId=", tran.DestinationId)
	tran.Type = "deposit"
	tran.Currency = currency.USD
	tran.Amount = 1000
	tran.MustPut()

	return tran
})
