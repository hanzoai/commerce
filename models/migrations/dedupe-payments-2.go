package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/payment"

	ds "crowdstart.com/datastore"
)

var _ = New("dedupe-payments-2",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "bellabeat")
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		// Delete payments which have been marked for deletion
		if pay.Deleted {
			pay.Delete()
		}
	},
)
