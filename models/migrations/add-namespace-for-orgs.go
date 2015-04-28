package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models/namespace"
	"crowdstart.io/models/organization"
	"crowdstart.io/util/log"

	ds "crowdstart.io/datastore"
)

var _ = New("add-namespace-for-orgs",
	func(c *gin.Context) {
		c.Set("namespace", "")
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
