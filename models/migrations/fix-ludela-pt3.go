package migrations

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/payment"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("fix-ludela-pt3",
	func(c *zip.Ctx) []interface{} {
		c.Locals("namespace", "ludela")
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		if pay.Buyer.FirstName == "\u263A" {
			pay.Buyer.FirstName = ""
		}

		if pay.Buyer.LastName == "\u263A" {
			pay.Buyer.LastName = ""
		}

		if pay.Buyer.FirstName == "☺" {
			pay.Buyer.FirstName = ""
		}

		if pay.Buyer.LastName == "☺" {
			pay.Buyer.LastName = ""
		}

		pay.MustPut()
	},
)
