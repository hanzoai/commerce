package migrations

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/payment"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("dedupe-payments-2",
	func(c *zip.Ctx) []interface{} {
		c.Locals("namespace", "kanoa")
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		// Delete payments which have been marked for deletion
		if pay.Deleted {
			if err := pay.Delete(); err != nil {
				log.Error("Failed to delete payment '%s': %v", pay.Id(), err, db.Context)
			}
		}
	},
)
