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

var _ = New("update-old-payments",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "bellabeat")
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		var payments []*payment.Payment

		keys, err := payment.Query(db).Filter("Account.ChargeId=", pay.Account.ChargeId).GetAll(&payments)
		if err != nil {
			log.Warn("Unable to query out payments: %v", err, db.Context)
			return
		}

		// Singular payment! Hooray
		if len(payments) == 1 {
			return
		}

		// You've been duped!
		newest := pay
		oldest := pay
		for i, p := range payments {
			p.Mixin(db, p)
			p.SetKey(keys[i])
			if p.CreatedAt.Before(oldest.CreatedAt) {
				oldest = p
			}

			if p.CreatedAt.After(newest.CreatedAt) {
				newest = p
			}
		}

		// Make sure order is right
		ord := order.New(db)
		err = ord.Get(newest.OrderId)
		if err == nil {
			// The newest order is correct
			oldest.Buyer.UserId = newest.Buyer.UserId
			oldest.OrderId = newest.OrderId
		} else {
			ord = order.New(db)
			err := ord.Get(oldest.OrderId)
			if err != nil {
				log.Error("Unable to find an order for either payment! oldest: %#v, newest: %#v", oldest, newest, db.Context)
				return
			}
		}

		// Update order
		seen := false
		for _, id := range ord.PaymentIds {
			if id == oldest.Id() {
				seen = true
			}
		}

		if !seen {
			ord.PaymentIds = append(ord.PaymentIds, oldest.Id())
		}

		if err := oldest.Put(); err != nil {
			log.Debug("Unable to update oldest payment: %v", err, db.Context)
		}

		if err := ord.Put(); err != nil {
			log.Error("Unable to save order: %v", err, db.Context)
		}

		// Make stripe charge match this payment
		err = updateChargeFromPayment(db.Context, oldest)
		if err != nil {
			log.Error("Unable to update charge from payent: %v", err, db.Context)
		}
	},
)
