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
}

func (p *Program) GetBonus(trans *transaction.Transaction) {
	for i, _ := range p.Triggers {
		action := p.Actions[i]
		switch action.Type {
		case StoreCredit:
			trans.Amount = action.Amount
			trans.Currency = action.Currency
			return
		case Refund:
		}
	}
}
