package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"

	ds "crowdstart.com/datastore"
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
