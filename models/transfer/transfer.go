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

type Status string

const (
	// Stripe status
	Canceled  Status = "canceled"
	Failed           = "failed"
	InTransit        = "in-transit"
	Paid             = "paid"
	Pending          = "pending"

	// Failed to submit to stripe
	Error = "error"
)

type StripeAccount struct {
	Id   string `json:"transferId,omimtempty"`
	Type string `json:"type,omitempty"`

	ApplicationFee int64 `json:"applicationFee,omitempty"` // FIXME: Apparently not returned by stripe-go?

	BalanceTransaction int64     `json:"balanceTransaction,omitempty"`
	Created            time.Time `json:"created,omitempty"`
	Date               time.Time `json:"date,omitempty"`
	Description        string    `json:"description,omitempty"`
	Destination        string    `json:"destination,omitempty"`
	DestinationType    string    `json:"destinationType,omitempty"`

	FailureCode    string `json:"failureCode,omitempty"`
	FailureMessage string `json:"failureMessage,omitempty"`

	Reversed bool `json:"reversed,omitempty"`

	SourceTransaction string `json:"sourceTransaction,omitempty"`
	SourceType        string `json:"sourceType,omitempty"`

	StatementDescriptor string `json:"statementDescriptor,omitempty"`
}

type Account struct {
	StripeAccount
}

type Transfer struct {
	mixin.Model
	Account

	AffiliateId string `json:"affiliateId"`

	Currency       currency.Type  `json:"currency"`
	Amount         currency.Cents `json:"amount"`
	AmountReversed currency.Cents `json:"amountReversed,omitempty"`

	Type   Type   `json:"type"`
	Status Status `json:"status"`
	Live   bool   `json:"live,omitempty"`

	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}
