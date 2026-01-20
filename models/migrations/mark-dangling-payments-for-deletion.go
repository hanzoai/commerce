package migrations

import (
	"errors"

	"github.com/gin-gonic/gin"

	ds "github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/payment"
)

var errNoSuchEntity = errors.New("datastore: no such entity")

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
		case errNoSuchEntity:
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
