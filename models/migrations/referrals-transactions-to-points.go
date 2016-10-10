package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/organization"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/transaction"
	"crowdstart.com/models/types/currency"

	ds "crowdstart.com/datastore"
)

var _ = New("referrals-transactions-to-points",
	func(c *gin.Context) []interface{} {
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
