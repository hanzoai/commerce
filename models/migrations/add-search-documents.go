package migrations

import (
	"github.com/zap-proto/zip"

	ds "github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/models/user"
)

var _ = New("add-search-documents",
	func(c *zip.Ctx) []interface{} {
		c.Locals("namespace", "cryptounderground")
		return NoArgs
	},
	func(db *ds.Datastore, u *user.User) {
		u.PutDocument()
	},
	func(db *ds.Datastore, o *order.Order) {
		o.PutDocument()
	},
	func(db *ds.Datastore, p *product.Product) {
		p.PutDocument()
	},
)
