package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/transaction"
	"hanzo.io/models/types/currency"
)

var Transaction = New("transaction", func(c *gin.Context) *transaction.Transaction {
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
