package referral

import (
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/currency"
)

type Type string

const (
	NewOrder Type = "new-order"
	NewUser       = "new-user"
)

type Referral struct {
	mixin.Model

	Type Type `json:"type"`

	// User created by referral
	UserId string `json:"userId"`

	// Order created by referral
	OrderId string `json:"orderId"`

	// Referred by
	Referrer struct {
		Id          string `json:"id"`
		UserId      string `json:"userId"`
		AffiliateId string `json:"affiliateId"`
	}

	Fee struct {
		Currency currency.Type  `json:"currency,omitempty"`
		Id       string         `json:"id,omitempty"`
		Amount   currency.Cents `json:"amount,omitempty"`
	} `json:"fee,omitempty"`
}
