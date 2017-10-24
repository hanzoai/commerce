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

type Ethereum struct {
	FinalAddress         string                `json:"finalAddress,omitempty"`
	FinalTransactionHash string                `json:"finalTransactionHash,omitempty"`
	FinalGasUsed         blockchains.BigNumber `json:"finalGasUsed,omitempty"`
	FinalAmount          blockchains.BigNumber `json:"finalAmount,omitempty"`
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

	Ethereum Ethereum `json:"ethereum"`

	// Stripe livemode
	Live bool `json:"live"`

	// Internal testing flag
	Test bool `json:"-"`
}
