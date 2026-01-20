package migrations

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/user"

	ds "github.com/hanzoai/commerce/datastore"
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
