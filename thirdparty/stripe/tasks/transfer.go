package tasks

import (
	"time"

	"appengine"

	"hanzo.io/thirdparty/stripe"
	"hanzo.io/util/delay"
	"hanzo.io/util/log"
)

// Synchronize payment using transfer
var TransferSync = delay.Func("stripe-transfer-sync", func(ctx appengine.Context, ns string, token string, str stripe.Transfer, start time.Time) {
	ctx = getNamespacedContext(ctx, ns)

	// Get payment using transfer
	tr, ok, err := getTransfer(ctx, &str)
	if err != nil {
		log.Error("Failed to query for transfer associated with Stripe transfer '%s': %v", str.ID, err, ctx)
		return
	}

	if !ok {
		log.Warn("No transfer associated with Stripe transfer '%s'", str.ID, ctx)
		return
	}

	if start.Before(tr.UpdatedAt) {
		log.Warn("Transfer '%s' previously updated, bailing out", tr.Id(), ctx)
		return
	}

	// Update transfer
	err = tr.RunInTransaction(func() error {
		log.Debug("Transfer before: %+v", tr, ctx)
		stripe.UpdateTransferFromStripe(tr, &str)
		log.Debug("Transfer after: %+v", tr, ctx)

		return tr.Put()
	})

	if err != nil {
		log.Error("Failed to update transfer '%s' from Stripe transfer %v: ", tr.Id(), str, err, ctx)
		return
	}
})
