package tasks

import (
	"appengine"

	"crowdstart.com/datastore"
	"crowdstart.com/models/fee"
	"crowdstart.com/models/multi"
	"crowdstart.com/models/transfer"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/delay"
)

// Payout a single fee
func payout(ctx appengine.Context, stripeToken, feeId string) {
	// Create stripe client
	sc := stripe.New(ctx, stripeToken)

	datastore.RunInTransaction(ctx, func(db *datastore.Datastore) error {
		// Fetch related fee
		fe := fee.New(db)
		fe.MustGet(feeId)

		// Create associated transfer
		tr := transfer.New(db)

		// Allocate transfer ID and Update fee
		fe.TransferId = tr.Id()
		fe.Status = fee.Paid

		// Create Stripe transfer
		if _, err := sc.Transfer(tr); err != nil {
			return err
		}

		// Save models
		models := []interface{}{tr, fe}
		return multi.Update(models)
	}, nil)
}

// Create associated tasks with unique queues
var PayoutPlatform = delay.Func("payout-platform", payout)
var PayoutAffiliate = delay.Func("payout-affiliate", payout)
