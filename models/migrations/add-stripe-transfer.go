package migrations

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/payment"

	ds "github.com/hanzoai/commerce/datastore"
)

// Legacy migration: Stripe charge lookups removed.
// This migration is historical and will no-op.
var _ = New("add-stripe-transfer",
	func(c *zip.Ctx) []interface{} {
		c.Locals("namespace", "cycliq")
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		log.Debug("add-stripe-transfer: skipped (legacy Stripe migration) for payment %s", pay.Id(), db.Context)
	},
)
