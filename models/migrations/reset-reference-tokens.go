package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/organization"
	"hanzo.io/models/token2"
	"hanzo.io/models/user"
	"hanzo.io/util/bit"
	"hanzo.io/util/log"
	"hanzo.io/util/permission"

	ds "hanzo.io/datastore"
)

var _ = New("reset-reference-tokens",
	func(c *gin.Context) []interface{} {
		db := ds.New(c)
		org := organization.New(db)
		if _, err := org.Query().Filter("Name=", "verus").First(); err != nil {
			panic(err)
		}

		c.Set("namespace", "")
		return []interface{}{org.Owners}
	},
	func(db *ds.Datastore, org *organization.Organization, verusOwners []string) {
		resetToken := func(userId string) {
			usr := user.New(db)
			if err := usr.GetById(userId); err != nil {
				log.Warn("Could not find owner.", db.Context)
				return
			}

			claims := token.Claims{}
			claims.Permissions = bit.Field(permission.Admin | permission.Live)
			claims.Scopes = []string{"owner"}

			_, err := org.ResetReferenceToken(usr, claims)
			if err != nil {
				log.Error("Could not reset refrence token %v", err)
			}
		}

		for _, ownerId := range org.Owners {
			resetToken(ownerId)
		}

		for _, ownerId := range verusOwners {
			resetToken(ownerId)
		}
	},
)
