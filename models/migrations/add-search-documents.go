package migrations

import (
	"github.com/gin-gonic/gin"

	ds "crowdstart.com/datastore"
	"crowdstart.com/models/user"
)

var _ = New("add-search-documents",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "")

		return NoArgs
	},
	func(db *ds.Datastore, u *user.User) {
		u.PutDocument()
	},
)
