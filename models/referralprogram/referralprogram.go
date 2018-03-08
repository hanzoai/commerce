package referralprogram

import (
	"hanzo.io/models/mixin"
	"hanzo.io/models/referral"
	"hanzo.io/models/types/currency"
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
	CreditGreaterThanOrEquals    TriggerType = "CreditGreaterThanOrEquals"
	ReferralsGreaterThanOrEquals TriggerType = "ReferralsGreaterThanOrEquals"
	Always                       TriggerType = "Always"
)

type CreditGreaterThanOrEqualsTrigger struct {
	CreditGreaterThanOrEquals currency.Cents `json:"creditGreaterThanOrEquals,omitempty"`
	Currency                  currency.Type  `json:"currency,omitempty"`
}

type ReferralsGreaterThanOrEqualsTrigger struct {
	ReferralsGreaterThanOrEquals int `json:"referralsGreaterThanOrEquals,omitempty"`
}

// Union of possible triggers
type Trigger struct {
	Event referral.Event `json:"event"`

	Type TriggerType `json:"type"`
	CreditGreaterThanOrEqualsTrigger
	ReferralsGreaterThanOrEqualsTrigger
}

type ReferralProgram struct {
	mixin.Model

	Name string `json:"name"`

	// Trigger is the number of referrals, 0 means it triggers on every referral
	Triggers []int    `json:"triggers"` // Deprecate soon, keep until that point in time
	Actions  []Action `json:"actions"`
}
