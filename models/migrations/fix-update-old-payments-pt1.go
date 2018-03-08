package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/payment"

	ds "hanzo.io/datastore"
)

// var accessToken = ""

// // Update charge in case order/pay id is missing in metadata
// func updateChargeFromPayment(ctx context.Context, pay *payment.Payment) error {
// 	// Get a stripe client
// 	client := stripe.New(ctx, accessToken)

// 	if _, err := client.UpdateCharge(pay); err != nil {
// 		log.Error("Failed to update charge '%s' using payment %#v: %v", pay.Account.ChargeId, pay, err, ctx)
// 		return err
// 	}

// 	log.Debug("Updated charge '%s' using payment: %#v", pay.Account.ChargeId, pay, ctx)
// 	return nil
// }

// // Ensure order has right payment id
// func orderNeedsPaymentId(ctx context.Context, ord *order.Order, pay *payment.Payment) error {
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

// func deletePayment(ctx context.Context, pay *payment.Payment) error {
// 	pay.Deleted = true
// 	if err := pay.Put(); err != nil {
// 		log.Error("Unable to mark payment '%s' as deleted: %v", pay.Id(), err, ctx)
// 		return err
// 	}
// 	return nil
// }

var _ = New("fix-update-old-payments-pt-1",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "bellabeat")
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		// Ensure that non-deleted payments have deleted set to false
		if !pay.Deleted {
			pay.MustPut()
		}
	},
)
