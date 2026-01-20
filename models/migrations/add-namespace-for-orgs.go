package migrations

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/models/namespace"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/log"

	ds "github.com/hanzoai/commerce/datastore"
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
