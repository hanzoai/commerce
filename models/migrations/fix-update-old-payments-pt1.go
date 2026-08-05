package migrations

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/payment"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("fix-update-old-payments-pt-1",
	func(c *zip.Ctx) []interface{} {
		c.Locals("namespace", "bellabeat")
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		// Ensure that non-deleted payments have deleted set to false
		if !pay.Deleted {
			pay.MustPut()
		}
	},
)
