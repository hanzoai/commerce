package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/cart"
	"crowdstart.com/models/organization"
	"crowdstart.com/thirdparty/mailchimp"

	ds "crowdstart.com/datastore"
)

var _ = New("sync-carts",
	func(c *gin.Context) []interface{} {
		db := ds.New(c)
		org := organization.New(db)
		org.GetById("ludela")
		c.Set("namespace", "ludela")
		return []interface{}{org.DefaultStore, org.Mailchimp.APIKey}
	},
	func(db *ds.Datastore, car *cart.Cart, defaultStore, apiKey string) {
		// Don't add carts which have converted into orders
		if car.OrderId != "" {
			return
		}

		// Update Mailchimp cart
		if car.UserId != "" || car.Email != "" {
			client := mailchimp.New(db.Context, apiKey)
			client.UpdateOrCreateCart(defaultStore, car)
		}
	},
)
