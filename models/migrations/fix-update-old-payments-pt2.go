package migrations

import (
	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"

	ds "crowdstart.com/datastore"
)

// var accessToken = ""

// Update charge in case order/pay id is missing in metadata
func updateChargeFromPayment2(ctx appengine.Context, pay *payment.Payment) {
	// Get a stripe client
	client := stripe.New(ctx, accessToken)

	if _, err := client.UpdateCharge(pay); err != nil {
		// This was a test mode charge
		log.Debug("Failed to update charge '%s', set payment '%s' to test mode", pay.Account.ChargeId, pay.Id(), ctx)
		pay.Test = true
		pay.MustPut()
	} else {
		log.Debug("Updated charge '%s' using payment: %#v", pay.Account.ChargeId, pay, ctx)
	}
}

// // Ensure order has right payment id
// func orderNeedsPaymentId(ctx appengine.Context, ord *order.Order, pay *payment.Payment) error {
// 	if len(ord.PaymentIds) > 0 && ord.PaymentIds[0] != pay.Id() {
// 		log.Warn("Payment '%v' not found in order '%v' PaymentIds: %#v", pay.Id(), ord.Id(), ord.PaymentIds, ctx)
// 		ord.PaymentIds = []string{pay.Id()}

// 		if err := ord.Put(); err != nil {
// 			log.Error("Failed to update order: %#v, bailing: %v", ord, err, ctx)
// 			return err
// 		}
// 	}

// 	return nil
// }

// func deletePayment(ctx appengine.Context, pay *payment.Payment) error {
// 	pay.Deleted = true
// 	if err := pay.Put(); err != nil {
// 		log.Error("Unable to mark payment '%s' as deleted: %v", pay.Id(), err, ctx)
// 		return err
// 	}
// 	return nil
// }

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

		updateChargeFromPayment2(ctx, pay)
	},
)
