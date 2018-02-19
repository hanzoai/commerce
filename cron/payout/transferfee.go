package payout

import (
	"fmt"

	"google.golang.org/appengine"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/models/affiliate"
	"hanzo.io/models/fee"
	"hanzo.io/models/multi"
	"hanzo.io/models/partner"
	"hanzo.io/models/transfer"
	"hanzo.io/thirdparty/stripe"
	"hanzo.io/util/delay"
	"hanzo.io/log"
)

func transferFromFee(db *datastore.Datastore, fe *fee.Fee) *transfer.Transfer {
	tr := transfer.New(db)
	tr.Amount = fe.Amount
	tr.Currency = fe.Currency
	tr.FeeId = fe.Id()

	// Setup transfer
	switch fe.Type {
	case fee.Affiliate:
		aff := affiliate.New(db)
		aff.MustGetById(fe.AffiliateId)
		tr.Description = fmt.Sprintf("Affiliate transfer '%s'", tr.Id())
		tr.Destination = aff.Stripe.UserId
	case fee.Partner:
		par := partner.New(db)
		par.MustGetById(fe.PartnerId)
		tr.Description = fmt.Sprintf("Partner transfer '%s'", tr.Id())
		tr.Destination = par.Stripe.UserId
	case fee.Platform:
		tr.Description = fmt.Sprintf("Platform fee transfer '%s', fee '%s'", tr.Id(), fe.Id())
		tr.Destination = config.Stripe.BankAccount
	default:
		panic(fmt.Errorf("Invalid fee type: '%s'\n", fe.Type, fe))
	}
	return tr
}

// Create transfer for single fee
var TransferFee = delay.Func("transfer-fee", func(ctx context.Context, stripeToken, namespace, id string) {
	var fe *fee.Fee
	var tr *transfer.Transfer

	// Switch to corrct namespace
	ctx, err := appengine.Namespace(ctx, namespace)
	if err != nil {
		log.Error("Failed to switch to namespace '%s': %v", namespace, err, ctx)
		return
	}

	// Create transfer and update payment in transaction
	err = datastore.RunInTransaction(ctx, func(db *datastore.Datastore) error {
		// Fetch related fee
		fe = fee.New(db)
		if err := fe.GetById(id); err != nil {
			log.Warn("Failed to get fee with id '%s': %v", id, err, ctx)
			return err
		}

		// Deal with invalid states
		if fe.Status == fee.Disputed {
			return fmt.Errorf("Fee '%s' is being disputed", fe.Id())
		}

		if fe.Status == fee.Transferred {
			return fmt.Errorf("Fee '%s' is already transferred", fe.Id())
		}

		// Create associated transfer
		tr := transferFromFee(db, fe)

		// Allocate transfer ID and Update fee
		fe.Status = fee.Transferred
		fe.TransferId = tr.Id()

		// Save models
		models := []interface{}{tr, fe}
		return multi.Update(models)
	})

	// Bail out if error happened creating transactions, any changes in
	// transaction will have been rolled back.
	if err != nil {
		log.Warn("Failed to create transfer for fee '%s', transfer '%s': %v", fe.Id(), tr.Id(), err, ctx)
		return
	}

	// Initiate transfer on Stripe's side
	sc := stripe.New(ctx, stripeToken)
	res, err := sc.Payout(tr)

	// Save transfer ID
	tr.Account.Id = res.ID

	if err != nil {
		log.Warn("Failed to create Stripe transfer for fee '%s', transfer '%s': %v", fe.Id(), tr.Id(), err, ctx)

		// Update transfer to reflect failure status
		tr.Status = transfer.Error
		if res.FailMessage == "" {
			tr.FailureCode = string(res.FailCode)
			tr.FailureMessage = res.FailMessage
		} else {
			tr.FailureCode = "stripe-error"
			tr.FailureMessage = err.Error()
		}

	}

	// Save transfer
	if err := tr.Update(); err != nil {
		log.Error("Failed to update status of failed transfer '%s': %v\n%v", tr.Id(), err, tr, ctx)
	}
})
