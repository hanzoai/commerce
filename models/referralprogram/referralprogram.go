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
	// Refund        ActionType = "Refund" // Refund part of the payment on a order
	SendUserEmail ActionType = "SendUserEmail"
)

type SendTransactionalUserEmailAction struct {
	EmailTemplate string `json:"template"`
}

type CreditAction struct {
	Currency currency.Type  `json:"currency,omitempty"`
	Amount   currency.Cents `json:"amount,omitempty"`
}

// type PercentAction struct {
// 	Percent float64 `json:"percent,omitempty"`
// }

// Union of possible actions
type Action struct {
	Type ActionType `json:"type"`
	Name string     `json:"name"`
	Once bool       `json:"once"`

	CreditAction
	// PercentAction
	SendTransactionalUserEmailAction

	Trigger Trigger `json:"trigger"`
}

const (
	CreditGreaterThan    TriggerType = "CreditGreaterThan"
	ReferralsGreaterThan TriggerType = "ReferralsGreaterThan"
	Always               TriggerType = "Always"
)

type CreditGreaterThanTrigger struct {
	CreditGreaterThan currency.Cents `json:"creditGreaterThan,omitempty"`
	Currency          currency.Type  `json:"currency,omitempty"`
}

type ReferralsGreaterThanTrigger struct {
	ReferralsGreaterThan int `json:"referralsGreaterThan,omitempty"`
}

// Union of possible triggers
type Trigger struct {
	Event referral.Event `json:"event"`

	Type TriggerType `json:"type"`
	CreditGreaterThanTrigger
	ReferralsGreaterThanTrigger
}

type ReferralProgram struct {
	mixin.Model

	Name string `json:"name"`

	// Trigger is the number of referrals, 0 means it triggers on every referral
	Triggers []int    `json:"triggers"` // Deprecate soon, keep until that point in time
	Actions  []Action `json:"actions"`
}
