package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/api/checkout/stripe"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/log"

	ds "crowdstart.com/datastore"
)

var oldPrice = currency.Cents(17900)
var discount = currency.Cents(2000)

var _ = New("kanoa-batch2-refund",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "kanoa")

		db := ds.New(c)
		org := organization.New(db)
		if err := org.GetById("kanoa"); err != nil {
			panic(err)
		}

		return []interface{}{org.Stripe.AccessToken}
	},
	func(db *ds.Datastore, ord *order.Order, stripeToken string) {
		org := organization.New(db)
		org.Stripe.AccessToken = stripeToken

		if v, ok := ord.Metadata["batch"]; !ok || v.(string) != "2" {
			return
		}

		if v, ok := ord.Metadata["refunded"]; ok && v.(bool) {
			return
		}

		if ord.LineTotal%oldPrice != 0 {
			return
		}

		lineTotal := ord.LineTotal
		multiplier := lineTotal / oldPrice
		if multiplier <= 0 {
			log.Error("Multiplier was less than 1", db.Context)
			return
		}

		refund := multiplier*discount - ord.Discount - ord.Refunded
		if refund <= 0 {
			log.Warn("Refund was less than 1", db.Context)
			return
		}

		// log.Error("Trying to refund %v cents, %v cents paid, %v cents discount using code %#v", refund, ord.Paid, ord.Refunded, ord.CouponCodes, db.Context)
		if err := stripe.Refund(org, ord, currency.Cents(refund)); err != nil {
			log.Error("Could not refund %v cents: %v", refund, err, db.Context)
			return
		}

		ord.Metadata["refunded"] = true
		ord.MustPut()
	},
)
