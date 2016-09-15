package referral

import (
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/currency"
)

type Referral struct {
	mixin.Model

	// User being referred
	UserId string `json:"userId"`

	// Associated order
	OrderId string `json:"orderId"`

	// Referred by
	ReferrerUserId string `json:"referrerUserId"`
	ReferrerId     string `json:"referrerId"`

	// Affiliate and fee
	AffiliateId string `json:"affiliateId"`

	Fee struct {
		Currency currency.Type  `json:"currency,omitempty"`
		Id       string         `json:"id,omitempty"`
		Amount   currency.Cents `json:"amount,omitempty"`
	} `json:"fee,omitempty"`
}
