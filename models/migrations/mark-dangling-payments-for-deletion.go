package migrations

import (
	"github.com/gin-gonic/gin"

	"appengine/datastore"

	ds "crowdstart.com/datastore"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/util/log"
)

var _ = New("mark-dangling-payments-for-deletion",
	func(c *gin.Context) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		ctx := db.Context
		oid := pay.OrderId
		ord := order.New(db)

		// Try and lookup order
		err := ord.GetById(oid)

		// Update payment accordingly
		switch err {
		case nil:
			pay.Deleted = false
		case datastore.ErrNoSuchEntity:
			pay.Deleted = true
		default:
			log.Error("Failed to query for order: %v", err, ctx)
			return
		}

		// Update payment
		if err := pay.Put(); err != nil {
			log.Error("Failed to save payment: %v", err, ctx)
		}
	})
