package migrations

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/models/user"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("fix-ludela-pt1",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "ludela")
		return NoArgs
	},
	func(db *ds.Datastore, usr *user.User) {
		if usr.Email == "" {
			usr.Delete()
			return
		}

		if usr.FirstName == "\u263A" {
			usr.FirstName = ""
		}

		if usr.LastName == "\u263A" {
			usr.LastName = ""
		}

		if usr.FirstName == "☺" {
			usr.FirstName = ""
		}

		if usr.LastName == "☺" {
			usr.LastName = ""
		}

		usr.MustPut()
	},
)
