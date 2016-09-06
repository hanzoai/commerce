package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/mailchimp"
	"crowdstart.com/util/log"

	ds "crowdstart.com/datastore"
)

var _ = New("mailchimp-orders",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "kanoa")

		db := ds.New(c)
		org := organization.New(db)
		if _, err := org.Query().Filter("Name=", "kanoa").First(); err != nil {
			panic(err)
		}
		return []interface{}{org.Mailchimp.APIKey, org.Mailchimp.ListId, org.DefaultStore}
	},
	func(db *ds.Datastore, ord *order.Order, apiKey, listId, defaultStore string) {
		if apiKey == "" {
			log.Warn("No MailChimp API Key", db.Context)
			return
		}

		if defaultStore == "" {
			log.Warn("No Default Store", db.Context)
			return
		}

		if listId == "" {
			log.Warn("No ListId", db.Context)
			return
		}

		client := mailchimp.New(db.Context, apiKey)
		client.CreateOrder(defaultStore, ord)

		pay := payment.New(db)

		if _, err := pay.Query().Filter("OrderId=", ord.Id()).First(); err != nil {
			log.Warn("No Payment Found for %v: %v", ord.Id(), err, db.Context)
			return
		}

		// Just get buyer off first payment
		if err := client.SubscribeCustomer(listId, pay.Buyer); err != nil {
			log.Warn("Failed to subscribe '%s' to Mailchimp list '%s': %v", pay.Buyer.Email, listId, err, db.Context)
		}
	},
)
