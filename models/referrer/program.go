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

func (p *Program) ApplyActions(r *Referrer) error {
	for i, _ := range p.Triggers {
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
