package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/cart"
	"hanzo.io/models/organization"
	"hanzo.io/thirdparty/mailchimp"
	"hanzo.io/types/integration"

	ds "hanzo.io/datastore"
)

var _ = New("damon-sync-carts",
	func(c *gin.Context) []interface{} {
		db := ds.New(c)
		org := organization.New(db)
		org.GetById("damon")
		c.Set("namespace", "damon")
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
