package fee

import (
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/commission"
	"crowdstart.com/models/types/currency"
)

type Type string

const (
	Platform  Type = "platform"
	Stripe         = "stripe"
	Affiliate      = "affiliate"
	Partner        = "partner"
)

type Status string

const (
	Pending  Status = "pending"
	Paid            = "paid"
	Refunded        = "refunded"
)

type Fee struct {
	mixin.Model

	Name string `json:"name"`

	Type        Type   `json:"type"`
	AffiliateId string `json:"affiliateId,omitempty"`
	PartnerId   string `json:"partnerId,omitempty"`

	PaymentId string `json:"paymentId"`
	TransferId string `json:"transferId"`

	Commission commission.Commission `json:"commission,omitempty"`

	Currency       currency.Type  `json:"currency"`
	Amount         currency.Cents `json:"amount"`
	AmountRefunded currency.Cents `json:"amountRefunded,omitempty"`

	Status Status `json:"status"`

	// Stripe livemode
	Live bool `json:"live"`

	// Internal testing flag
	Test bool `json:"-"`
}
