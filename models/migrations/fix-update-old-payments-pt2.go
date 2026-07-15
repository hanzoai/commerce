package migrations

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/payment"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("fix-update-old-payments-pt-2",
	func(c *zip.Ctx) []interface{} {
		c.Locals("namespace", "bellabeat")
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		if pay.Deleted || pay.Test {
			return
		}

		// Legacy: Stripe charge update removed
		log.Debug("fix-update-old-payments-pt-2: skipped (legacy Stripe migration) for payment %s", pay.Id(), db.Context)
	},
)
