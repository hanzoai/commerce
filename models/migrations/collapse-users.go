package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/order"
	"crowdstart.com/models/user"
	"crowdstart.com/util/log"

	ds "crowdstart.com/datastore"
)

var _ = New("collapse-users",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "bellabeat")
		return NoArgs
	},
	func(db *ds.Datastore, ord *order.Order) {
		id := ord.UserId

		// Look up user for this order
		usr := user.New(db)
		if err := usr.GetById(id); err != nil {
			log.Warning("Failed to query for user: %v", id)
			return
		}

		// Try to find newest instance of a user with this email
		usr2 := user.New(db)
		if err := usr2.Query().Order("-CreatedAt").Filter("Email=", usr.Email).First(); err != nil {
			log.Warning("Failed to query for newest user: %v", usr)
			return
		}

		// Update order with correct user id
		ord.UserId = usr2.Id()

		// Save order
		ord.Put()
	},
)
