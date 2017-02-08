package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/namespace"
	"hanzo.io/models/organization"
	"hanzo.io/util/log"

	ds "hanzo.io/datastore"
)

var _ = New("add-namespace-for-orgs",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "")
		return NoArgs
	},
	func(db *ds.Datastore, org *organization.Organization) {
		ns := namespace.New(db)
		ns.Name = org.Name
		ns.IntId = org.Key().IntID()
		err := ns.Put()
		if err != nil {
			log.Warn("Failed to put namespace: %v", err)
		}
	},
)
