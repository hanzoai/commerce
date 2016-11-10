package migrations

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/fee"
	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/stripe/tasks"

	ds "crowdstart.com/datastore"
)

var _ = New("payment-fee-status-update",
	func(c *gin.Context) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		if fees, err := pay.GetFees(); err == nil {
			tasks.UpdateFeesFromPayment(fees, pay)
		}
	},
)

var _ = New("fee-status-update",
	func(c *gin.Context) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, fe *fee.Fee) {
		fees := []*fee.Fee{fe}
		pay := payment.New(db)
		if err := pay.GetById(fe.PaymentId); err != nil {
			tasks.UpdateFeesFromPayment(fees, pay)
		}
	},
)
