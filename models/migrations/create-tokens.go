package migrations

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/models/token"
	"github.com/hanzoai/commerce/models/user"

	ds "github.com/hanzoai/commerce/datastore"
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
		tok.Put()
	},
)
