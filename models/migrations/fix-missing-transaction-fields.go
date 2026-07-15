package migrations

import (
	"github.com/zap-proto/zip"

	ds "github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/transaction"
)

var _ = New("fix-missing-transaction-fields",
	func(c *zip.Ctx) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, t *transaction.Transaction) {
		if t.Type == "" {
			t.Type = transaction.Deposit
		}
		t.Put()
	},
)
