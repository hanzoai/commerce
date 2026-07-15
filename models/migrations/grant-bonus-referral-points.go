package migrations

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/user"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("grant-bonus-referral-points",
	func(c *zip.Ctx) []interface{} {
		c.Locals("namespace", "bellabeat")

		db := ds.New(c.Context())
		org := organization.New(db)
		if _, err := org.Query().Filter("Name=", "bellabeat").Get(); err != nil {
			panic(err)
		}
		return NoArgs
	},
	func(db *ds.Datastore, usr *user.User) {
		id := usr.Id()

		trans := transaction.New(db)
		if ok, _ := trans.Query().Filter("DestinationId=", id).Get(); ok {
			trans := transaction.New(db)
			trans.DestinationId = id
			trans.Type = transaction.Deposit
			trans.Currency = currency.USD
			trans.Amount = currency.Cents(3000)
			trans.Put()
		}
	},
)
