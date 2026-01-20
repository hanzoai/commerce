package fee

import (
	"github.com/hanzoai/commerce/models/blockchains"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/commission"
	"github.com/hanzoai/commerce/models/types/currency"
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

type Bitcoin struct {
	FinalTransactionTxId string         `json:"finalTransactionTxId,omitempty"`
	FinalAddress         string         `json:"finalAddress,omitempty"`
	FinalAmount          currency.Cents `json:"finalAmount,omitempty"`
	FinalVOut            int64          `json:"vout,omitempty"`
}

type Ethereum struct {
	FinalTransactionHash string                `json:"finalTransactionHash,omitempty"`
	FinalTransactionCost blockchains.BigNumber `json:"finalTransactionCost,omitempty"`
	FinalAddress         string                `json:"finalAddress,omitempty"`
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
	Bitcoin  Bitcoin  `json:"bitcoin"`

	// Stripe livemode
	Live bool `json:"live"`

	// Internal testing flag
	Test bool `json:"-"`
}
