package migrations

import (
	"strings"

	"github.com/gin-gonic/gin"

	"hanzo.io/models/token"
	"hanzo.io/models/user"

	ds "hanzo.io/datastore"
)

var _ = New("fix-emails",
	func(c *gin.Context) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, tok *token.Token) {
		tok.Email = strings.ToLower(strings.TrimSpace(tok.Email))
		tok.Put()
	},
	func(db *ds.Datastore, usr *user.User) {
		usr.Email = strings.ToLower(strings.TrimSpace(usr.Email))
		usr.Put()
	},
)
