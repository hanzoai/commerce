package tasks

import (
	"time"

	"appengine"
	"appengine/delay"

	"crowdstart.com/models/transfer"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/thirdparty/stripe"
	"crowdstart.com/util/log"
)

// Update payment from transfer
func UpdateTransfer(tr *transfer.Transfer, str *stripe.Transfer) {
	tr.Amount = currency.Cents(str.Amount)
	tr.AmountReversed = currency.Cents(str.AmountReversed)
	tr.Currency = currency.Type(str.Currency)
	tr.Live = str.Live

	tr.Account.ApplicationFee = str.Tx.Fee
	tr.Account.BalanceTransaction = str.Tx.Amount
	tr.Account.Date = time.Unix(str.Date, 0)
	tr.Account.Created = time.Unix(str.Date, 0)
	tr.Account.Description = str.Desc
	tr.Account.Destination = str.Dest.ID
	tr.Account.DestinationType = string(str.Dest.Type)
	tr.Account.FailureCode = string(str.FailCode)
	tr.Account.FailureMessage = str.FailMsg
	tr.Account.Reversed = str.Reversed
	tr.Account.SourceTransaction = str.SourceTx.ID
	tr.Account.SourceType = string(str.SourceType)
	tr.Account.StatementDescriptor = str.Statement
	tr.Account.Type = string(str.Type)

	switch str.Status {
	case "paid":
		tr.Status = transfer.Paid
	case "pending":
		tr.Status = transfer.Pending
	case "in_transit":
		tr.Status = transfer.InTransit
	case "cancelled":
		tr.Status = transfer.Canceled
	case "failed":
		tr.Status = transfer.Failed
	}
}

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
		UpdateTransfer(tr, &str)
		log.Debug("Transfer after: %+v", tr, ctx)

		return tr.Put()
	})

	if err != nil {
		log.Error("Failed to update transfer '%s' from Stripe transfer %v: ", tr.Id(), str, err, ctx)
		return
	}
})
