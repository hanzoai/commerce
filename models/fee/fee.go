package fee

import (
	"hanzo.io/models/blockchains"
	"hanzo.io/models/mixin"
	"hanzo.io/models/types/commission"
	"hanzo.io/models/types/currency"
)

type Type string

const (
	Platform  Type = "platform"
	Stripe    Type = "stripe"
	Affiliate Type = "affiliate"
	Partner   Type = "partner"
)

type Status string

const (
	Pending     Status = "pending"
	Disputed    Status = "disputed"
	Transferred Status = "transferred"
	Payable     Status = "payable"
	Refunded    Status = "refunded"
)

type EthereumFee struct {
	Address           string                `json:"address,omitempty"`
	SignedTransaction string                `json:"signedTransaction,omitempty"`
	TransferCost      blockchains.BigNumber `json:"transferCost,omitempty`
}

type Fee struct {
	mixin.Model

	Name string `json:"name"`

	Type        Type   `json:"type"`
	AffiliateId string `json:"affiliateId,omitempty"`
	PartnerId   string `json:"partnerId,omitempty"`

	PaymentId  string `json:"paymentId"`
	TransferId string `json:"transferId"`

	Commission commission.Commission `json:"commission,omitempty"`

	Currency       currency.Type  `json:"currency"`
	Amount         currency.Cents `json:"amount"`
	AmountRefunded currency.Cents `json:"amountRefunded,omitempty"`

	Status Status `json:"status"`

	EthereumFee `json:"ethereumFee"`

	// Stripe livemode
	Live bool `json:"live"`

	// Internal testing flag
	Test bool `json:"-"`
}
