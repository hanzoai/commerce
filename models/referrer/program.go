package referrer

import (
	"crowdstart.com/models/referral"
	"crowdstart.com/models/transaction"
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

type Program struct {
	Name string `json:"name"`

	// Trigger is the number of referrals, 0 means it triggers on every referral
	ReferralTriggers []int    `json:"triggers"` // Deprecate soon, keep until that point in time
	Trigger          Trigger  `json:"trigger"`
	Actions          []Action `json:"actions"`

	Event referral.Type `json:"event"`
}

func (p *Program) TestTrigger(r *Referrer) error {
	switch p.Trigger.Type {
	case CreditGreaterThan:
		return nil
	case ReferralsGreaterThan:
		return nil
	}

	return nil
}

func (p *Program) ApplyActions(r *Referrer) error {
	for i, _ := range p.ReferralTriggers {
		action := p.Actions[i]
		switch action.Type {
		case StoreCredit:
			return saveStoreCredit(r, action.Amount, action.Currency)
		case Refund:
		}
	}

	// No actions triggered for this referral
	return nil
}

// Credit user with store credit by saving transaction
func saveStoreCredit(r *Referrer, amount currency.Cents, cur currency.Type) error {
	trans := transaction.New(r.Db)
	trans.Type = transaction.Deposit
	trans.Amount = amount
	trans.Currency = cur
	trans.SourceId = r.Id()
	trans.SourceKind = r.Kind()
	trans.UserId = r.UserId
	trans.Notes = "Deposit due to referral"
	trans.Tags = "referral"
	return trans.Create()
}
