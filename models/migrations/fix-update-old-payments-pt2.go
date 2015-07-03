package migrations

import (
	"strings"

	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"

	ds "crowdstart.com/datastore"
)

// var accessToken = ""

func testModeError(err error) bool {
	return strings.Contains(err.Error(), "a similar object exists in test mode, but a live mode key was used to make this request")
}

// Update charge in case order/pay id is missing in metadata
func updateChargeFromPayment2(ctx appengine.Context, pay *payment.Payment) {
	// Get a stripe client
	client := stripe.New(ctx, accessToken)

	_, err := client.UpdateCharge(pay)
	if err == nil {
		log.Debug("Updated charge '%s' using payment: %#v", pay.Account.ChargeId, pay, ctx)
		return
	}

	if !testModeError(err) {
		log.Error("Failed to update charge '%s' from payment '%s': %v", pay.Account.ChargeId, pay.Id(), err, ctx)
		return
	}

	// This was a test mode charge, update payment
	log.Debug("Found test payment '%s' and associated order '%s'", pay.Id(), pay.OrderId, ctx)
	pay.Test = true
	pay.MustPut()

	// Update order
	ord := order.New(pay.Db)
	if err := ord.Get(pay.OrderId); err != nil {
		log.Error("Failed to set order '%s' to test mode: %v", ord.Id(), err, ctx)
	}
	ord.Test = true
	ord.MustPut()
}

var _ = New("fix-update-old-payments-pt-2",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "bellabeat")
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		if pay.Deleted || pay.Test {
			return
		}

		ctx := db.Context

		ord := order.New(db)
		if err := ord.Get(pay.OrderId); err != nil {
			log.Error("Found broken payment: %#v", pay, ctx)
			return
		}

		// Mostly just want to ensure metadata is right and test mode stuff is flagged correctly.
		updateChargeFromPayment2(ctx, pay)
	},
)
