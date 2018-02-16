package migrations

import (
	"github.com/gin-gonic/gin"

	ds "hanzo.io/datastore"
	"hanzo.io/models/transaction"
)

var _ = New("fix-missing-transaction-fields",
	func(c *context.Context) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, t *transaction.Transaction) {
		if t.Type == "" {
			t.Type = transaction.Deposit
		}
		t.Put()
	},
)
