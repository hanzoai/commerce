package referral

import "crowdstart.com/models/mixin"

type Referral struct {
	mixin.Model

	// User being referred
	UserId    string `json:"userId"`
	FirstName string `json:"firstName"`

	// Associated order
	OrderId string `json:"orderId"`

	// Referred by
	ReferrerUserId string `json:"referrerUserId"`
	ReferrerId     string `json:"referrerId"`
}
