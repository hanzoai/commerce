package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/log"
	"hanzo.io/models/organization"
	"hanzo.io/models/product"
	"hanzo.io/thirdparty/mailchimp"
	"hanzo.io/types/integration"

	ds "hanzo.io/datastore"
)

var _ = New("damon-mailchimp-products",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "damon")

		db := ds.New(c)
		org := organization.New(db)
		if _, err := org.Query().Filter("Name=", "damon").Get(); err != nil {
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

		mc := integration.Mailchimp{
			APIKey: apiKey,
		}

		client := mailchimp.New(db.Context, mc)
		// Create order in mailchimp
		if err := client.CreateProduct(defaultStore, prod); err != nil {
			log.Warn("Failed to create Mailchimp product: %v", err, db.Context)
		}
	},
)
