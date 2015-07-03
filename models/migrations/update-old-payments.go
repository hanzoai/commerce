package migrations

import (
	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/util/log"

	ds "crowdstart.com/datastore"

	"crowdstart.com/thirdparty/stripe"
)

var accessToken = ""

// Update charge in case order/pay id is missing in metadata
func updateChargeFromPayment(ctx appengine.Context, pay *payment.Payment) error {
	// Get a stripe client
	client := stripe.New(ctx, accessToken)

	_, err := client.UpdateCharge(pay)
	return err
}

// Ensure order has right payment id
func orderNeedsPaymentId(ctx appengine.Context, ord *order.Order, pay *payment.Payment) error {
	if len(ord.PaymentIds) > 0 && ord.PaymentIds[0] != pay.Id() {
		log.Warn("Single payment '%v' not found in order '%v' PaymentIds: %#v", pay.Id(), ord.Id(), ord.PaymentIds, ctx)
		ord.PaymentIds = []string{pay.Id()}

		if err := ord.Put(); err != nil {
			log.Error("Failed to update order: %#v, bailing: %v", ord, err, ctx)
			return err
		}
	}

	return nil
}

var _ = New("update-old-payments",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "bellabeat")
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		var payments []*payment.Payment

		ctx := db.Context // Cache context

		// Query out payments with matching chargeId's, only one is linked to a
		// real order, and the charge should be pointed at that one.
		keys, err := payment.Query(db).Filter("Account.ChargeId=", pay.Account.ChargeId).GetAll(&payments)
		if err != nil {
			log.Error("Unable to query out payments: %v", err, ctx)
			return
		}

		if len(payments) == 1 {
			log.Debug("Single payment found '%v'", pay.Id(), ctx)

			// Check if this payment has a matching order
			ord := order.New(db)
			if err := ord.Get(pay.OrderId); err != nil {
				log.Error("Did not find order associated with: %#v, bailing: %v", pay, err, ctx)
				return
			}

			// Update order if necessary
			if err := orderNeedsPaymentId(ctx, ord, pay); err != nil {
				return
			}

			log.Debug("Single payment '%v' associated with order '%v'", pay.Id(), ord.Id(), ctx)

			// log.Debug("Updating charge from payment %#v", pay, ctx)
			// err = updateChargeFromPayment(ctx, pay)
			// if err != nil {
			// 	log.Error("Failed to update charge using payment: %#v", pay, ctx)
			// 	return
			// }

			return
		}

		log.Warn("Found multiple payments: %v", payments, ctx)

		// Find newest/oldest payments
		newest := pay
		oldest := pay
		for i, p := range payments {
			// Make sure we have a payment we can work with
			p.Mixin(db, p)
			p.SetKey(keys[i])

			// Find the oldest
			if p.CreatedAt.Before(oldest.CreatedAt) {
				oldest = p
			}

			// Find the youngest
			if p.CreatedAt.After(newest.CreatedAt) {
				newest = p
			}
		}

		// Check newest order
		ord := order.New(db)
		err = ord.Get(newest.OrderId)
		if err == nil {
			log.Debug("Newest payment has order: %#v", newest, ctx)

			// Update order if necessary
			if err := orderNeedsPaymentId(ctx, ord, newest); err != nil {
				return
			}

			// Update CreatedAt
			newest.CreatedAt = oldest.CreatedAt
			if err := newest.Put(); err != nil {
				log.Error("Unable to update payment %#v: %v", newest, err, ctx)
				return
			}

			// log.Debug("Updating charge from payment %#v", newest, ctx)
			// err = updateChargeFromPayment(ctx, newest)
			// if err != nil {
			// 	log.Error("Unable to update charge from payment: %#v", err, ctx)
			// }

			// Delete older payment
			// if err := oldest.Delete(); err != nil {
			// 	log.Error("Unable to delete older payment: %#v, #v", oldest, err, ctx)
			// 	return
			// }

			log.Debug("Deleted oldest payment: %#v", oldest, ctx)
			log.Debug("Newest payment '%v' associated with order '%v'", newest.Id(), ord.Id(), ctx)
			return
		}

		// Check oldest order
		ord = order.New(db)
		err = ord.Get(oldest.OrderId)
		if err != nil {
			log.Error("Unable to find an order for either payment! oldest: %#v, newest: %#v", oldest, newest, ctx)
			return
		}

		log.Debug("Oldest payment has order: %v", oldest, ctx)

		// Update order if necessary
		if err := orderNeedsPaymentId(ctx, ord, pay); err != nil {
			return
		}

		// log.Debug("Updating charge from payment %#v", oldest, ctx)
		// err = updateChargeFromPayment(ctx, newest)
		// if err != nil {
		// 	log.Error("Unable to update charge from payment: %#v", err, ctx)
		// }

		// Delete newest payment
		// if err := newest.Delete(); err != nil {
		// 	log.Error("Unable to delete older payment: %#v, #v", newest, err, ctx)
		// 	return
		// }
		log.Debug("Deleted newest payment: %#v", newest, ctx)
		log.Debug("Oldest payment '%v' associated with order '%v'", oldest.Id(), ord.Id(), ctx)
	},
)
