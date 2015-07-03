package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/payment"
	"crowdstart.com/util/log"

	ds "crowdstart.com/datastore"
)

var _ = New("update-old-payments",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "bellabeat")
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		var payments []*payment.Payment

		if _, err := payment.Query(db).Filter("Account.ChargeId=", pay.Account.ChargeId).GetAll(payments); err != nil {
			log.Warn("Unable to query out payments?????", db.Context)
			return
		}

		// Singular payment! Hooray
		if len(payments) == 1 {
			return
		}

		// You've been duped!
		newest := pay
		oldest := pay
		for _, p := range payments {
			if p.CreatedAt.Before(oldest.CreatedAt) {
				oldest = p
			}

			if p.CreatedAt.After(newest.CreatedAt) {
				newest = p
			}
		}

		// oldest is old. he is a good guy
		oldest.Buyer.UserId = newest.Buyer.UserId
		oldest.OrderId = newest.OrderId

		if err := oldest.Put(); err != nil {
			log.Debug("Unable to update oldest payment: %v", err, db.Context)
		}
	},
)
