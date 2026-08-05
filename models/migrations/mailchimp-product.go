package migrations

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/thirdparty/mailchimp"
	"github.com/hanzoai/commerce/types/integration"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("mailchimp-products",
	func(c *zip.Ctx) []interface{} {
		c.Locals("namespace", "cover")

		db := ds.New(c.Context())
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

		mc := integration.Mailchimp{
			APIKey: apiKey,
		}

		client := mailchimp.New(db.Context, mc)
		// Create order in mailchimp
		if err := client.CreateProduct("rdtXY3AUj3zbX", prod); err != nil {
			log.Warn("Failed to create Mailchimp product: %v", err, db.Context)
		}
		if err := client.CreateProduct("petm7PEohWk8bm", prod); err != nil {
			log.Warn("Failed to create Mailchimp product: %v", err, db.Context)
		}
	},
)
