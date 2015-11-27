package migrations

import (
	"github.com/gin-gonic/gin"

	ds "crowdstart.com/datastore"
	"crowdstart.com/models/order"
)

var _ = New("add-search-documents",
	func(c *gin.Context) []interface{} {
		return NoArgs
	},
	// func(db *ds.Datastore, u *user.User) {
	// 	u.PutDocument()
	// },
	func(db *ds.Datastore, o *order.Order) {
		o.PutDocument()
	},
)
