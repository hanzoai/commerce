package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/user"

	ds "crowdstart.com/datastore"
)

var _ = New("fix-ludela-pt1",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "ludela")
		return NoArgs
	},
	func(db *ds.Datastore, usr *user.User) {
		if usr.FirstName == "☺" {
			usr.FirstName = ""
		}
		if usr.LastName == "☺" {
			usr.LastName = ""
		}
	},
)
