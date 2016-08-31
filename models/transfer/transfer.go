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

type StripeAccount struct {
	// note: all times should be in UTC. sadly, the stdlib does not
	// include a datatype that enforces this
	DestinationPaymentId string    `json:"destinationPaymentId,omitempty"`
	BalanceTransactionId string    `json:"balanceTransactionId,omitempty"`
	SourceTransactionId  string    `json:"sourceTransactionId,omitempty"`
	StatementDescriptor  string    `json:"statementDescriptor,omitempty"`
	Destination          string    `json:"destination,omitempty"`
	DeliveryDate         time.Time `json:"deliveryDate,omitempty"`
	Description          string    `json:"description,omitempty"`
	Live                 bool      `json:"live,omitempty"`        // see Stripe's "livemode" field
	PaymentType          string    `json:"paymentType,omitempty"` // see Stripe's "type" field
	FailureCode          string    `json:"failureCode,omitempty"`
	FailureMessage       string    `json:"failureMessage,omitempty"`
	ApplicationFee       string    `json:"applicationFee,omitempty"`
}

// data Account = Stripe StripeAccount | ...
type Account struct {
	StripeAccount
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
	Account // see 'Type'

	Amount         currency.Cents `json:"amount"`
	AmountRefunded currency.Cents `json:"amountRefunded,omitempty"`

	Type   Type   `json:"type"`
	Status Status `json:"status"`

	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}
