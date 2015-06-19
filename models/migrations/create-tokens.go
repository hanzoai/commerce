package migrations

import (
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/models/token"
	"crowdstart.com/models/user"

	ds "crowdstart.com/datastore"
)

var _ = New("create-tokens",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "bellabeat")
		return NoArgs
	},
	func(db *ds.Datastore, usr *user.User) {
		tok := token.New(usr.Db)
		tok.Email = usr.Email
		tok.UserId = usr.Id()
		tok.Expires = time.Now().Add(time.Hour * 168)
	},
)
