package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/cart"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/thirdparty/mailchimp"
	"hanzo.io/util/log"

	ds "hanzo.io/datastore"
)

var _ = New("mailchimp-orders",
	func(c *context.Context) []interface{} {
		c.Set("namespace", "stoned")

		db := ds.New(c)
		org := organization.New(db)
		if _, err := org.Query().Filter("Name=", "stoned").Get(); err != nil {
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
		// Create order in mailchimp
		if err := client.CreateOrder(defaultStore, ord); err != nil {
			log.Warn("Failed to create Mailchimp order: %v", err, db.Context)
		}

		// Update cart
		car := cart.New(ord.Db)

		if ord.CartId != "" {
			if err := car.GetById(ord.CartId); err != nil {
				log.Warn("Unable to find cart: %v", err, db.Context)
			} else {
				// Delete cart in mailchimp
				if err := client.DeleteCart(defaultStore, car); err != nil {
					log.Warn("Failed to create Mailchimp cart: %v", err, db.Context)
				}
			}
		}

		// pay := payment.New(db)

		// if _, err := pay.Query().Filter("OrderId=", ord.Id()).Get(); err != nil {
		// 	log.Warn("No Payment Found for %v: %v", ord.Id(), err, db.Context)
		// 	return
		// }

		// Just get buyer off first payment
		// if err := client.SubscribeCustomer(listId, pay.Buyer, ""); err != nil {
		// 	log.Warn("Failed to subscribe '%s' to Mailchimp list '%s': %v", pay.Buyer.Email, listId, err, db.Context)
		// }
	},
)
