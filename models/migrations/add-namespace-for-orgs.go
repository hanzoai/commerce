package migrations

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/namespace"
	"github.com/hanzoai/commerce/models/organization"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("add-namespace-for-orgs",
	func(c *zip.Ctx) []interface{} {
		c.Locals("namespace", "")
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
