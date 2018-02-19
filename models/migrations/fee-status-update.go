package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/fee"
	"hanzo.io/models/payment"
	"hanzo.io/thirdparty/stripe/tasks"
	"hanzo.io/log"

	ds "hanzo.io/datastore"
)

var _ = New("payment-fee-status-update",
	func(c *context.Context) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, pay *payment.Payment) {
		if fees, err := pay.GetFees(); err == nil {
			tasks.UpdateFeesFromPayment(fees, pay)
		}
	},
)

var _ = New("fee-status-update",
	func(c *context.Context) []interface{} {
		return NoArgs
	},
	func(db *ds.Datastore, fe *fee.Fee) {
		fees := []*fee.Fee{fe}
		pay := payment.New(db)
		if err := pay.GetById(fe.PaymentId); err == nil {
			log.Warn("Updating Fees", db.Context)
			tasks.UpdateFeesFromPayment(fees, pay)
		} else {
			log.Error("Payment '%s' not found.", fe.PaymentId, db.Context)
		}
	},
)
