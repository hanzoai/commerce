package invoice

import (
	"strconv"

	"hanzo.io/models/mixin"
	"hanzo.io/models/types/client"
	"hanzo.io/models/types/currency"
	"hanzo.io/thirdparty/stripe"

	. "hanzo.io/models"
)

type Status string

const (
	Cancelled  Status = "cancelled"
	Credit            = "credit"
	Disputed          = "disputed"
	Failed            = "failed"
	Fraudulent        = "fraudulent"
	Paid              = "paid"
	Refunded          = "refunded"
	Unpaid            = "unpaid"
)

type Type string

const (
	Stripe Type = "stripe"
	Affirm      = "affirm"
	PayPal      = "paypal"
)

type Invoice struct {
	mixin.Model

	// Invoice source information
	Account Account `json:"account"`

	// Immutable buyer data from time of payment, may or may not be associated
	// with a user.
	Buyer Buyer `json:"buyer"`

	Type Type `json:"type"`

	// Order this is associated with
	OrderId string `json:"orderId,omitempty"`

	Currency currency.Type `json:"currency"`

	CampaignId string `json:"campaignId"`

	Amount         currency.Cents `json:"amount"`
	AmountRefunded currency.Cents `json:"amountRefunded"`
	Fee            currency.Cents `json:"fee"`

	AmountTransferred   currency.Cents `json:"-"`
	CurrencyTransferred currency.Type  `json:"-"`

	Description string `json:"description"`
	Status      Status `json:"status"`

	// Client's browser, associated info
	Client client.Client `json:"client"`

	// Whether this payment has been captured or not
	Captured bool `json:"captured"`

	// Stripe livemode
	Live bool `json:"live"`

	// Internal testing flag
	Test bool `json:"-"`

	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (i *Invoice) Defaults() {
	i.Status = Unpaid
	i.Metadata = make(Map)
}

func (p Invoice) ToCard() *stripe.CardParams {
	card := stripe.CardParams{}
	card.Name = p.Buyer.Name()
	card.Number = p.Account.Number
	card.CVC = p.Account.CVC
	card.Month = strconv.Itoa(p.Account.Month)
	card.Year = strconv.Itoa(p.Account.Year)
	card.Address1 = p.Buyer.Address.Line1
	card.Address2 = p.Buyer.Address.Line2
	card.City = p.Buyer.Address.City
	card.State = p.Buyer.Address.State
	card.Zip = p.Buyer.Address.PostalCode
	card.Country = p.Buyer.Address.Country
	return &card
}
