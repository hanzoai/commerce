package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/models/types/currency"
	"hanzo.io/thirdparty/stripe"
	"hanzo.io/log"

	ds "hanzo.io/datastore"
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

		pay.Account.BalanceTransactionId = charge.Tx.ID
		pay.AmountTransferred = currency.Cents(charge.Tx.Amount)
		pay.CurrencyTransferred = currency.Type(charge.Tx.Currency)
		pay.Put()
	},
)
