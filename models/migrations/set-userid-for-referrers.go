package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/order"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/referrer"
	"crowdstart.com/util/log"

	ds "crowdstart.com/datastore"
)

var _ = New("set-userid-for-referrers",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "bellabeat")
		return NoArgs
	},
	func(db *ds.Datastore, ref *referrer.Referrer) {
		ctx := db.Context
		orderId := ref.OrderId

		// Look up order referenced by referrer
		ord := order.New(db)
		if err := ord.GetById(orderId); err != nil {
			log.Error("Failed to query for order: %v\n%v", orderId, err, ctx)
			return
		}

		// Same user, just return
		if ord.UserId == ref.UserId {
			log.Debug("Same user")
			return
		}

		// Update the user id on the referrer
		ref.UserId = ord.UserId
		ref.Put()

		// Update associated referrals
		var referrals []*referral.Referral
		if _, err := referral.Query(db).Filter("ReferrerId=", ref.Id()).GetAll(&referrals); err != nil {
			log.Error("Failed to query out referrals, ReferrerId: %v\n%v", ref.Id(), err, ctx)
			return
		}

		for _, refl := range referrals {
			refl.Referrer.UserId = ref.UserId
			refl.Init(db)
			if err := refl.Put(); err != nil {
				log.Error("Failed to update referral: %v", refl, err, ctx)
				return
			}
		}
	},
)
