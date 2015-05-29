package referrer

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/order"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/transaction"
	"crowdstart.com/util/val"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Referrer struct {
	mixin.Model

	Program        Program                   `json:"program"`
	OrderId        string                    `json:"orderId"`
	UserId         string                    `json:"userId"`
	ReferralIds    []string                  `json:"referralIds"`
	TransactionIds []string                  `json:"transactionsIds"`
	Transactions   []transaction.Transaction `json:"transactions,omitempty"`
}

func New(db *datastore.Datastore) *Referrer {
	r := new(Referrer)
	r.Init()
	r.Model = mixin.Model{Db: db, Entity: r}
	return r
}

func (r Referrer) Init() {
	r.ReferralIds = make([]string, 0)
	r.TransactionIds = make([]string, 0)
}

func (r Referrer) Kind() string {
	return "referrer"
}

func (r Referrer) Document() mixin.Document {
	return nil
}

func (r *Referrer) Validator() *val.Validator {
	return nil
}

func (r *Referrer) ApplyBonus() (*transaction.Transaction, error) {
	trans := transaction.New(r.Db)
	r.Program.GetBonus(trans, len(r.ReferralIds))
	trans.UserId = r.UserId
	if err := trans.Put(); err != nil {
		return nil, err
	}
	r.TransactionIds = append(r.TransactionIds, trans.Id())

	return trans, nil
}

func (r *Referrer) SaveReferral(ord *order.Order) (*referral.Referral, error) {
	ref := referral.New(ord.Db)
	ref.UserId = ord.UserId
	ref.ReferrerUserId = r.UserId
	ref.OrderId = ord.Id()
	ref.ReferrerId = ord.ReferrerId

	// Try to save referral
	if err := ref.Put(); err != nil {
		return ref, err
	}

	// Save referral id on referrer
	r.ReferralIds = append(r.ReferralIds, ref.Id())

	// Save transaction to referral user's account to update their balance
	if _, err := r.ApplyBonus(); err != nil {
		return ref, err
	}

	// Try to save referrer
	err := r.Put()

	return ref, err
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
