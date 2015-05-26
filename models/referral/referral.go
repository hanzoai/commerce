package referral

import (
	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/transaction"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/val"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Type string

const (
	StoreCredit Type = "Credit" // Add credit to user's balance
	Refund           = "Refund" // Refund part of the payment on a order
)

type Credit struct {
	Currency currency.Type
	Amount   currency.Cents
}

type Percent struct {
	Percent float64
}

type Action struct {
	Type Type
	Credit
	Percent
}

type Referral struct {
	mixin.Model

	// Trigger is the number of referrals, 0 means it triggers on every referral
	Triggers []int    `json:"triggers"`
	Actions  []Action `json:"actions"`
}

func New(db *datastore.Datastore) *Referral {
	r := new(Referral)
	r.Model = mixin.Model{Db: db, Entity: r}
	return r
}

func (r Referral) Init() {
	r.Triggers = make([]int, 0)
}

func (r Referral) Kind() string {
	return "referral"
}

func (r *Referral) Load(c <-chan aeds.Property) (err error) {
	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(r, c)); err != nil {
		return err
	}

	return err
}

func (r *Referral) Save(c chan<- aeds.Property) (err error) {
	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(r, c))
}

func (r *Referral) Validator() *val.Validator {
	return nil
}

func (r *Referral) GetBonus(referrals int) *transaction.Transaction {
	for i, trig := range r.Triggers {
		if trig == referrals || trig == 0 {
			action := r.Actions[i]
			switch r.Actions[i].Type {
			case StoreCredit:
				trans := transaction.New(r.Db)
				trans.Amount = action.Amount
				trans.Currency = action.Currency
				return trans
			case Refund:
			}
		}
	}
	return nil
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
