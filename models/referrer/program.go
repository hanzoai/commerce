package referrer

import (
	"crowdstart.com/models/transaction"
	"crowdstart.com/models/types/currency"
)

type Type string

const (
	StoreCredit Type = "Credit" // Add credit to user's balance
	Refund           = "Refund" // Refund part of the payment on a order
	EmailUser        = "EmailUser"
)

type Credit struct {
	Currency currency.Type  `json:"currency,omitempty"`
	Amount   currency.Cents `json:"amount,omitempty"`
}

type Percent struct {
	Percent float64 `json:"percent,omitempty"`
}

// Union of possible actions
type Action struct {
	Type Type `json:"type"`
	Credit
	Percent
}

type Program struct {
	Name string `json:"name"`

	// Invariant: Triggers and Actions must have the same length.
	// Each trigger entry specifies the minimum number of referrals
	// necessary to fulfill the associated action.
	// A trigger of 0 indicates that the associated action is always
	// eligible for fulfillment.
	Triggers []int    `json:"triggers"`
	Actions  []Action `json:"actions"`
}

func (p *Program) ApplyActions(r *Referrer, alreadyRewarded int, newReferrals int) []ActionError {
	var ret []ActionError
	
	if (newReferrals <= alreadyRewarded) {
		return ret
	}

	for i, trigger := range p.Triggers {
		if trigger == 0 || (newReferrals >= trigger && trigger > alreadyRewarded) {
			var err error
			a := p.Actions[i]
			switch a.Type {
			case StoreCredit:
				err = saveStoreCredit(r, a.Amount, a.Currency)
			case EmailUser:
			case Refund:
			}
			if err != nil {
				ret = append(ret, ActionError{a, err})
			}
		}
	}

	return ret
}

type ActionError struct {
	Action Action
	Error error
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
