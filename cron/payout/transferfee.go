package payout

import (
	"appengine"

	"crowdstart.com/datastore"
	"crowdstart.com/models/fee"
	"crowdstart.com/models/multi"
	"crowdstart.com/models/transfer"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/delay"
	"crowdstart.com/util/log"
)

// Create transfer for single fee
var TransferFee = delay.Func("transfer-fee", func(ctx appengine.Context, stripeToken, namespace, key string) {
	var fe *fee.Fee
	var tr *transfer.Transfer

	// Switch to corrct namespace
	ctx, _ = appengine.Namespace(ctx, namespace)

	// Create transfer and update payment in transaction
	err := datastore.RunInTransaction(ctx, func(db *datastore.Datastore) error {
		// Fetch related fee
		fe = fee.New(db)
		if err := fe.Get(key); err != nil {
			log.Warn("Failed to get fee with key '%s': %v", key, err, ctx)
			return err
		}

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
		log.Warn("Failed to create transfer for fee '%s', transfer '%s': %v", fe.Id(), tr.Id(), err, ctx)
		return
	}

	// Initiate transfer on Stripe's side
	sc := stripe.New(ctx, stripeToken)
	if tr_, err := sc.Transfer(tr); err != nil {
		log.Warn("Failed to create Stripe transfer for fee '%s', transfer '%s': %v", fe.Id(), tr.Id(), err, ctx)

		// Update transfer to reflect failure status
		tr.Status = transfer.Error
		if tr_.FailMsg == "" {
			tr.FailureCode = string(tr_.FailCode)
			tr.FailureMessage = tr_.FailMsg
		} else {
			tr.FailureCode = "stripe-error"
			tr.FailureMessage = err.Error()
		}

		// Save transfer
		if err := tr.Update(); err != nil {
			log.Error("Failed to update status of failed transfer '%s': %v", tr.Id(), err, ctx)
		}
	}
})
