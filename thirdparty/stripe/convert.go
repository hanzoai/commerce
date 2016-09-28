package stripe

import (
	"time"

	"crowdstart.com/models/transfer"
	"crowdstart.com/models/types/currency"
)

// Update transfer from Stripe
func UpdateTransferFromStripe(tr *transfer.Transfer, str *Transfer) {
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
