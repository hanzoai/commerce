package migrations

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/user"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("fix-ludela-pt2",
	func(c *zip.Ctx) []interface{} {
		c.Locals("namespace", "ludela")
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		key := pay.Key().Parent().Parent()
		usr := user.New(db)
		usr.Get(key)

		if usr.FirstName == "" {
			usr.FirstName = pay.Buyer.FirstName
		}

		if usr.LastName == "" {
			usr.LastName = pay.Buyer.LastName
		}

		usr.MustPut()
	},
)
