package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/order"
	"hanzo.io/models/referral"
	"hanzo.io/util/log"

	ds "hanzo.io/datastore"
)

var _ = New("set-userid-for-referrals",
	func(c *context.Context) []interface{} {
		c.Set("namespace", "bellabeat")
		return NoArgs
	},
	func(db *ds.Datastore, refl *referral.Referral) {
		ctx := db.Context
		orderId := refl.OrderId

		// Look up order referenced by referral
		ord := order.New(db)
		if err := ord.GetById(orderId); err != nil {
			log.Error("Failed to query for order: %v\n%v", orderId, err, ctx)
			return
		}

		// Same user, just return
		if ord.UserId == refl.UserId {
			log.Debug("Same user")
			return
		}

		// Update the user id on the referrer
		refl.UserId = ord.UserId
		refl.Put()
	},
)
