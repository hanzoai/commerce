package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/order"
	"crowdstart.com/models/user"

	ds "crowdstart.com/datastore"
)

var _ = New("grant-bonus-referral-points",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "bellabeat")
		return NoArgs
	},
	func(db *ds.Datastore, ord *order.Order) {
		id := ord.UserId

		usr := user.New(db)
		usr.GetById(id)
		email := usr.Email

		usr2 := user.New(db)

		usr.Query().Order("-CreatedAt").Filter("Email=", email).First(usr2)
		ord.UserId = usr2.Id()

		ord.Put()
	},
)
