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
func payout(ctx appengine.Context, stripeToken, orgName, feeKey string) {
	var tr *transfer.Transfer

	// Create transfer and update payment in transaction
	err := datastore.RunInTransaction(ctx, func(db *datastore.Datastore) error {
		// Fetch related fee
		fe := fee.New(db)
		fe.MustGet(feeKey)

		// Create associated transfer
		tr = transfer.New(db)

		// Allocate transfer ID and Update fee
		fe.TransferId = tr.Id()
		fe.Status = fee.Paid

		// Save reference to transfer's key so we can update it later

		// Save models
		models := []interface{}{tr, fe}
		return multi.Update(models)
	}, nil)

	// Bail out if error happened creating transactions, any changes in
	// transaction will have been rolled back.
	if err != nil {
		return
	}

	// Initiate transfer on Stripe's side
	sc := stripe.New(ctx, stripeToken)
	if _, err := sc.Transfer(tr); err != nil {
		return err
	}

	// Update transfer to reflect failure status
	if err != nil {
		tr.Status = transfer.Canceled
		tr.MustUpdate()
	}
}

// Create associated tasks with unique queues
var PayoutPlatform = delay.Func("payout-platform", payout)
var PayoutAffiliate = delay.Func("payout-affiliate", payout)
