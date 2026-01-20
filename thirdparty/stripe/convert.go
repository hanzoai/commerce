package stripe

import (
	"time"

	"github.com/hanzoai/commerce/models/transfer"
	"github.com/hanzoai/commerce/models/types/currency"
)

// Update transfer from Stripe
func UpdateTransferFromStripe(tr *transfer.Transfer, str *Transfer) {
	tr.Amount = currency.Cents(str.Amount)
	tr.AmountReversed = currency.Cents(str.AmountReversed)
	tr.Currency = currency.Type(str.Currency)

	tr.Live = str.Livemode

	tr.Account.ApplicationFee = str.DestinationPayment.ApplicationFeeAmount
	tr.Account.BalanceTransaction = str.Amount
	//tr.Account.Date = time.Unix(str.Date, 0)
	//tr.Account.Created = time.Unix(str.Date, 0)
	//tr.Account.Description = str.Description
	tr.Account.Destination = str.Destination.ID
	tr.Account.DestinationType = string(str.Destination.Type)
	//tr.Account.FailureCode = string(str.FailCode)
	//tr.Account.FailureMessage = str.FailMsg
	tr.Account.Reversed = str.Reversed
	tr.Account.SourceTransaction = str.SourceTransaction.ID
	tr.Account.SourceType = string(str.SourceType)
	tr.Account.StatementDescriptor = str.Description
	tr.Account.Type = string(str.DestinationPayment.Status)

	switch str.DestinationPayment.Status {
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

func UpdatePayoutFromStripe(tr *transfer.Transfer, str *Payout) {
	tr.Amount = currency.Cents(str.Amount)
	//tr.AmountReversed = currency.Cents(str.AmountReversed)
	tr.Currency = currency.Type(str.Currency)
	tr.Live = str.Livemode
	//tr.Account.ApplicationFee = str.Tx.Fee
	//tr.Account.BalanceTransaction = str.Tx.Amount
	tr.Account.Date = time.Unix(str.ArrivalDate, 0)
	tr.Account.Created = time.Unix(str.Created, 0)
	tr.Account.Description = str.StatementDescriptor
	tr.Account.Destination = str.Destination.ID
	tr.Account.DestinationType = string(str.Destination.Type)
	tr.Account.FailureCode = string(str.FailureCode)
	tr.Account.FailureMessage = str.FailureMessage
	//tr.Account.Reversed = str.Reversed
	//tr.Account.SourceTransaction = str.SourceTx.ID
	tr.Account.SourceType = string(str.SourceType)
	tr.Account.StatementDescriptor = str.StatementDescriptor
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
