package migrations

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/cart"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/thirdparty/mailchimp"
	"github.com/hanzoai/commerce/types/integration"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("damon-sync-carts",
	func(c *zip.Ctx) []interface{} {
		db := ds.New(c.Context())
		org := organization.New(db)
		org.GetById("damon")
		c.Locals("namespace", "damon")
		return []interface{}{org.DefaultStore, org.Mailchimp.APIKey}
	},
	func(db *ds.Datastore, car *cart.Cart, defaultStore, apiKey string) {
		// Don't add carts which have converted into orders
		if car.OrderId != "" {
			return
		}

		mc := integration.Mailchimp{
			APIKey: apiKey,
		}
		// Update Mailchimp cart
		if car.UserId != "" || car.Email != "" {
			client := mailchimp.New(db.Context, mc)
			client.UpdateOrCreateCart(defaultStore, car)
		}
	},
)
