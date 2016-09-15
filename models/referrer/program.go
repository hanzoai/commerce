package referrer

import (
	"crowdstart.com/models/transaction"
	"crowdstart.com/models/types/currency"
)

type Type string

const (
	StoreCredit Type = "Credit" // Add credit to user's balance
	Refund           = "Refund" // Refund part of the payment on a order
)

type Event string

const (
	NewOrder Event = "order.new"
	NewUser        = "user.new"
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

	// Trigger is the number of referrals, 0 means it triggers on every referral
	Triggers []int    `json:"triggers"`
	Actions  []Action `json:"actions"`

	Event Event `json:"event"`
}

func (r *Program) GetBonus(trans *transaction.Transaction, referrals int) {
	for i, trig := range r.Triggers {
		if trig == referrals || trig == 0 {
			action := r.Actions[i]
			switch r.Actions[i].Type {
			case StoreCredit:
				trans.Amount = action.Amount
				trans.Currency = action.Currency
				return
			case Refund:
			}
		}
	}
}
