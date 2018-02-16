package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/organization"
	"hanzo.io/models/referrer"
	"hanzo.io/models/transaction"
	"hanzo.io/models/types/currency"

	ds "hanzo.io/datastore"
)

var _ = New("referrals-transactions-to-points",
	func(c *context.Context) []interface{} {
		c.Set("namespace", "bellabeat")

		db := ds.New(c)
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
