package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/user"
	"crowdstart.com/util/log"

	ds "crowdstart.com/datastore"
)

var _ = New("dedupe-users",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "bellabeat")
		return NoArgs
	},
	func(db *ds.Datastore, usr *user.User) {
		ctx := db.Context

		// Try to find newest instance of a user with this email
		usr2 := user.New(db)
		if _, err := usr2.Query().Filter("Email=", usr.Email).Order("-CreatedAt").First(); err != nil {
			log.Error("Failed to query for newest user: %v", err, ctx)
			return
		}

		// Same user, just return
		if usr2.Id() == usr.Id() {
			log.Warn("Same user", ctx)
			return
		}

		usr2.Delete()
	},
)
