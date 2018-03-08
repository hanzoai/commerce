package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/app"
	"hanzo.io/models/organization"

	ds "hanzo.io/datastore"
)

var _ = New("add-missing-defaultapp",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "")
		return NoArgs
	},
	func(db *ds.Datastore, org *organization.Organization) {
		nsCtx := org.Namespaced(org.Db.Context)
		nsDb := datastore.New(nsCtx)

		ap := app.New(nsDb)
		if err := ap.GetById(organization.DefaultAppName); err != nil {
			ap.Name = organization.DefaultAppName
			ap.MustCreate()
		}
	},
)
