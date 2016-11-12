package referralprogram

import (
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/types/currency"
)

type ActionType string
type TriggerType string

const (
	StoreCredit ActionType = "Credit" // Add credit to user's balance
	Refund                 = "Refund" // Refund part of the payment on a order
)

const (
	CreditGreaterThan    TriggerType = "CreditGreaterThan"
	ReferralsGreaterThan             = "ReferralsGreaterThan"
)

type CreditAction struct {
	Currency currency.Type  `json:"currency,omitempty"`
	Amount   currency.Cents `json:"amount,omitempty"`
}

type PercentAction struct {
	Percent float64 `json:"percent,omitempty"`
}

// Union of possible actions
type Action struct {
	Type ActionType `json:"type"`
	Name string     `json:"name"`
	Once bool       `json:"once"`

	CreditAction
	PercentAction
}

type CreditGreaterThanTrigger struct {
	CreditGreaterThan int           `json:"creditGreaterThan,omitempty"`
	Currency          currency.Type `json:"currency,omitempty"`
}

type ReferralsGreaterThanTrigger struct {
	ReferralsGreaterThan int `json:"referralsGreaterThan,omitempty"`
}

// Union of possible triggers
type Trigger struct {
	Type TriggerType `json:"type"`
	CreditGreaterThanTrigger
	ReferralsGreaterThanTrigger
}

type ReferralProgram struct {
	mixin.Model

	Name string `json:"name"`

	// Trigger is the number of referrals, 0 means it triggers on every referral
	ReferralTriggers []int    `json:"triggers"` // Deprecate soon, keep until that point in time
	Trigger          Trigger  `json:"trigger"`
	Actions          []Action `json:"actions"`

	Event referral.Type `json:"event"`
}
