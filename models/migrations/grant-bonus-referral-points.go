package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/organization"
	"hanzo.io/models/transaction"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/user"

	ds "hanzo.io/datastore"
)

var _ = New("grant-bonus-referral-points",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "bellabeat")

		db := ds.New(c)
		org := organization.New(db)
		if _, err := org.Query().Filter("Name=", "bellabeat").First(); err != nil {
			panic(err)
		}
		return NoArgs
	},
	func(db *ds.Datastore, usr *user.User) {
		id := usr.Id()

		if ok, _ := transaction.Query(db).Filter("UserId=", id).First(); ok {
			trans := transaction.New(db)
			trans.UserId = id
			trans.Type = transaction.Deposit
			trans.Currency = currency.USD
			trans.Amount = currency.Cents(3000)
			trans.Put()
		}
	},
)
