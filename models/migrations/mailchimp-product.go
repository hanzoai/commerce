package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/organization"
	"hanzo.io/models/product"
	"hanzo.io/thirdparty/mailchimp"
	"hanzo.io/util/log"

	ds "hanzo.io/datastore"
)

var _ = New("mailchimp-products",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "cover")

		db := ds.New(c)
		org := organization.New(db)
		if _, err := org.Query().Filter("Name=", "cover").Get(); err != nil {
			panic(err)
		}
		return []interface{}{org.Mailchimp.APIKey, org.Mailchimp.ListId, org.DefaultStore}
	},
	func(db *ds.Datastore, prod *product.Product, apiKey, listId, defaultStore string) {
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
		if err := client.CreateProduct(defaultStore, prod); err != nil {
			log.Warn("Failed to create Mailchimp product: %v", err, db.Context)
		}
	},
)
