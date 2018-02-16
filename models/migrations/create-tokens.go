package migrations

import (
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/models/token"
	"hanzo.io/models/user"

	ds "hanzo.io/datastore"
)

var _ = New("create-tokens",
	func(c *context.Context) []interface{} {
		c.Set("namespace", "bellabeat")
		return NoArgs
	},
	func(db *ds.Datastore, usr *user.User) {
		tok := token.New(usr.Db)
		tok.Email = usr.Email
		tok.UserId = usr.Id()
		tok.Expires = time.Now().Add(time.Hour * 168)
		tok.Put()
	},
)
