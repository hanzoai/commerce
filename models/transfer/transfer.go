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
	Id                  string    `json:"transferId,omimtempty"`
	ApplicationFee      int64     `json:"applicationFee,omitempty"` // FIXME: Apparently not returned by stripe-go?
	BalanceTransaction  int64     `json:"balanceTransaction,omitempty"`
	Created             string    `json:"created,omitempty"`
	DeliveryDate        time.Time `json:"deliveryDate,omitempty"`
	Description         string    `json:"description,omitempty"`
	DestinationId       string    `json:"destinationId,omitempty"`
	DestinationType     string    `json:"destinationType,omitempty"`
	FailureCode         string    `json:"failureCode,omitempty"`
	FailureMessage      string    `json:"failureMessage,omitempty"`
	Live                bool      `json:"live,omitempty"`
	Reversed            bool      `json:"reversed,omitempty"`
	SourceId            string    `json:"sourcId,omitempty"`
	SourceType          string    `json:"sourceType,omitempty"`
	StatementDescriptor string    `json:"statementDescriptor,omitempty"`
}

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
	Account

	Currency       currency.Type  `json:"currency"`
	Amount         currency.Cents `json:"amount"`
	AmountReversed currency.Cents `json:"amountReversed,omitempty"`

	Type   Type   `json:"type"`
	Status Status `json:"status"`

	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

// XXXih: the typical lifecycle of a Transfer is as follows:
// 1. a Transfer is created and stored to datastore; this produces a unique ID
// 2. the aforementioned unique ID is then used as an "idempotency tag" in all
//    associated requests to our payment processor
// 3. ...
