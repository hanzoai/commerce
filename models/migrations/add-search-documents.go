package migrations

import (
	"github.com/gin-gonic/gin"

	ds "hanzo.io/datastore"
	"hanzo.io/models/order"
	"hanzo.io/models/product"
	// "hanzo.io/models/user"
)

var _ = New("add-search-documents",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "stoned")

		return NoArgs
	},
	// func(db *ds.Datastore, u *user.User) {
	// 	u.PutDocument()
	// },
	func(db *ds.Datastore, o *order.Order) {
		o.PutDocument()
	},
	func(db *ds.Datastore, p *product.Product) {
		p.PutDocument()
	},
)
