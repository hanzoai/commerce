package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/user"

	ds "hanzo.io/datastore"
)

var _ = New("fix-zero-dates",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "kanoa")

		db := datastore.New(c)
		org := organization.New(db)
		org.GetById("kanoa")

		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		if pay.CreatedAt.IsZero() {
			pay.CreatedAt = pay.UpdatedAt
			pay.MustUpdate()
		}
	},
	func(db *ds.Datastore, usr *user.User) {
		if usr.CreatedAt.IsZero() {
			usr.CreatedAt = usr.UpdatedAt
			usr.MustUpdate()
		}
	},
	func(db *ds.Datastore, ord *order.Order) {
		if ord.CreatedAt.IsZero() {
			ord.CreatedAt = ord.UpdatedAt
			ord.MustUpdate()
		}
	},
)
