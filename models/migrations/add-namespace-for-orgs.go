package migrations

import (
	"github.com/gin-gonic/gin"

	"appengine"

	"crowdstart.io/models/constants"
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
		nsCtx, err := appengine.Namespace(db.Context, constants.NamespaceNamespace)
		if err != nil {
			log.Error("Could not update namespace %v", err, db.Context)
		}

		nsDb := ds.New(nsCtx)
		ns := namespace.New(nsDb)

		ns.Name = org.Name
		ns.GetOrCreate("Name=", ns.Name)
		ns.IntId = org.Key().IntID()
		ns.MustPut()
	},
)
