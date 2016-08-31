package transfer

import (
	"time"

	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/currency"

	. "crowdstart.com/models"
)

type Type string

const (
	Stripe Type = "stripe"
)

type StripeData struct {
	// note: all times should be in UTC. sadly, the stdlib does not
	// include a datatype that enforces this
	DestinationPaymentId string    `json:"destinationPaymentId,omitempty"`
	BalanceTransactionId string    `json:"balanceTransactionId,omitempty"`
	SourceTransactionId  string    `json:"sourceTransactionId,omitempty"`
	StatementDescriptor  string    `json:"statementDescriptor,omitempty"`
	Destination          string    `json:"destination,omitempty"`
	DeliveryDate         time.Time `json:"deliveryDate,omitempty"`
	Description          string    `json:"description,omitempty"`
	Live                 bool      `json:"live,omitempty"`        // see stripe's "livemode" field
	PaymentType          string    `json:"paymentType,omitempty"` // see stripe's "type" field
	FailureCode          string    `json:"failureCode,omitempty"`
	FailureMessage       string    `json:"failureMessage,omitempty"`
	ApplicationFee       string    `json:"applicationFee,omitempty"`
}

// data TransferData = Stripe StripeData | ...
type TransferData struct {
	StripeData
}

type Status string

const (
	Initializing Status = "initializing"
	Pending             = "pending"
	Paid                = "paid"
	InTransit           = "inTransit"
	Canceled            = "canceled"
	Failed              = "failed"
)

type Transfer struct {
	mixin.Model
	TransferData // see 'Type'

	Amount         currency.Cents `json:"amount"`
	AmountRefunded currency.Cents `json:"amountRefunded,omitempty"`

	Type Type `json:"type"`

	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:"-"`
}

// XXXih: the typical lifecycle of a Transfer is as follows:
// 1. a Transfer is created and stored to datastore; this produces a unique ID
// 2. the aforementioned unique ID is then used as an "idempotency tag" in all
//    associated requests to our payment processor
// 3. ...
