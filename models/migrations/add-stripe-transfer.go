package migrations

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/thirdparty/stripe"

	ds "github.com/hanzoai/commerce/datastore"
)

var _ = New("add-stripe-transfer",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "cycliq")

		db := ds.New(c)
		org := organization.New(db)
		if _, err := org.Query().Filter("Name=", "cycliq").Get(); err != nil {
			panic(err)
		}
		return []interface{}{org.Stripe.AccessToken}
	},
	func(db *ds.Datastore, pay *payment.Payment, accessToken string) {
		sc := stripe.New(db.Context, accessToken)
		charge, err := sc.GetCharge(pay.Account.ChargeId)
		if err != nil {
			log.Error(err)
			return
		}

		pay.Account.BalanceTransactionId = charge.BalanceTransaction.ID
		pay.AmountTransferred = currency.Cents(charge.Amount)
		pay.CurrencyTransferred = currency.Type(charge.Currency)
		pay.Put()
	},
)
