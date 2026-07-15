package migrations

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/referrer"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("referrals-transactions-to-points",
	func(c *zip.Ctx) []interface{} {
		c.Locals("namespace", "bellabeat")

		db := ds.New(c.Context())
		org := organization.New(db)
		if _, err := org.Query().Filter("Name=", "bellabeat").Get(); err != nil {
			panic(err)
		}
		return NoArgs
	},
	func(db *ds.Datastore, ref *referrer.Referrer) {
		for i, _ := range ref.Program.Actions {
			ref.Program.Actions[i].Currency = currency.PNT
		}
		ref.Put()
	},
	func(db *ds.Datastore, trans *transaction.Transaction) {
		trans.Currency = currency.PNT
		trans.Put()
	},
)
