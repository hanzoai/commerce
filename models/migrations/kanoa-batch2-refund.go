package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/api/checkout/stripe"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/types/currency"
	"hanzo.io/util/log"

	ds "hanzo.io/datastore"
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

		return []interface{}{org.Stripe.Live.AccessToken, org.Stripe.Test.AccessToken}
	},
	func(db *ds.Datastore, ord *order.Order, stripeToken, testStripeToken string) {
		org := organization.New(db)

		org.Live = true
		org.Stripe.AccessToken = stripeToken
		org.Stripe.Live.AccessToken = stripeToken
		org.Stripe.Test.AccessToken = testStripeToken

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

		log.Warn("Trying to refund %v cents, %v cents paid, %v cents discount using code %#v", refund, ord.Paid, ord.Refunded, ord.CouponCodes, db.Context)
		if err := stripe.Refund(org, ord, currency.Cents(refund)); err != nil {
			log.Error("Could not refund %v cents: %v", refund, err, db.Context)
			return
		}

		ord.Metadata["refunded"] = true
		ord.MustPut()
	},
)
