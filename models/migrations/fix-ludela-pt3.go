package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/payment"

	ds "hanzo.io/datastore"
)

var _ = New("fix-ludela-pt3",
	func(c *context.Context) []interface{} {
		c.Set("namespace", "ludela")
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
