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

	if _, err := client.UpdateCharge(pay); err != nil {
		log.Error("Failed to update charge '%s' using payment %#v: %v", pay.Account.ChargeId, pay, err, ctx)
		return err
	}

	log.Debug("Updated charge '%s' using payment: %#v", pay.Account.ChargeId, pay, ctx)
	return nil
}

// Ensure order has right payment id
func orderNeedsPaymentId(ctx appengine.Context, ord *order.Order, pay *payment.Payment) error {
	if len(ord.PaymentIds) > 0 && ord.PaymentIds[0] != pay.Id() {
		log.Warn("Payment '%v' not found in order '%v' PaymentIds: %#v", pay.Id(), ord.Id(), ord.PaymentIds, ctx)
		ord.PaymentIds = []string{pay.Id()}

		if err := ord.Put(); err != nil {
			log.Error("Failed to update order: %#v, bailing: %v", ord, err, ctx)
			return err
		}
	}

	return nil
}

func deletePayment(ctx appengine.Context, pay *payment.Payment) error {
	pay.Deleted = true
	if err := pay.Put(); err != nil {
		log.Error("Unable to mark payment '%s' as deleted: %#v", pay.Id(), pay, err, ctx)
		return err
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

			// if err := updateChargeFromPayment(ctx, pay); err != nil {
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

			// if err := updateChargeFromPayment(ctx, newest); err != nil {
			// 	return
			// }

			// Delete older payment
			if err := deletePayment(ctx, oldest); err != nil {
				return
			}

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

		// if err := updateChargeFromPayment(ctx, newest); err != nil {
		// 	return
		// }

		// Delete newest payment
		if err := deletePayment(ctx, newest); err != nil {
			return
		}

		log.Debug("Deleted newest payment: %#v", newest, ctx)
		log.Debug("Oldest payment '%v' associated with order '%v'", oldest.Id(), ord.Id(), ctx)
	},
)
