package migrations

import (
	"context"
	"strings"

	"google.golang.org/appengine/delay"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/log"
	"hanzo.io/models/order"
	"hanzo.io/models/payment"
	"hanzo.io/thirdparty/stripe"

	ds "hanzo.io/datastore"
)

// var accessToken = ""

func testModeError(err error) bool {
	return strings.Contains(err.Error(), "a similar object exists in test mode, but a live mode key was used to make this request")
}

// Update charge in case order/pay id is missing in metadata
var updateChargeAndFixTestMode = delay.Func("update-charge-and-fix-test-mode", func(ctx context.Context, payId string) {
	db := datastore.New(ctx)
	pay := payment.New(db)
	if err := pay.GetById(payId); err != nil {
		log.Error("Unable to get payment: %v", err, ctx)
		return
	}

	ord := order.New(db)
	if err := ord.GetById(pay.OrderId); err != nil {
		log.Error("Unable to get order for payment '%s': %v", payId, err, ctx)
		return
	}

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

	// This was a test mode charge, update payment and order
	log.Debug("Found test payment '%s' and associated order '%s'", pay.Id(), pay.OrderId, ctx)
	pay.Test = true
	pay.MustPut()

	ord.Test = true
	ord.MustPut()
})

var _ = New("fix-update-old-payments-pt-2",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "bellabeat")
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		if pay.Deleted || pay.Test {
			return
		}

		// Mostly just want to ensure metadata is right and test mode stuff is flagged correctly.
		updateChargeAndFixTestMode.Call(db.Context, pay.Id())
	},
)
