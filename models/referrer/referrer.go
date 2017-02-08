package referrer

import (
	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/order"
	"hanzo.io/models/referral"
	"hanzo.io/models/transaction"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Referrer struct {
	mixin.Model

	Program Program `json:"program"`
	OrderId string  `json:"orderId"`
	UserId  string  `json:"userId"`
}

func (r *Referrer) ApplyBonus() (*transaction.Transaction, error) {
	trans := transaction.New(r.Db)
	count, err := referral.Query(r.Db).Filter("ReferrerId=", r.Id()).Count()
	if err != nil {
		return nil, err
	}

	r.Program.GetBonus(trans, count)
	trans.UserId = r.UserId
	trans.Type = transaction.Deposit

	trans.Notes = "Deposit from referral " + r.Id()
	trans.Tags = "referral"

	trans.SourceId = r.Id()
	trans.SourceKind = r.Kind()

	if err := trans.Put(); err != nil {
		return nil, err
	}

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

	// Save transaction to referral user's account to update their balance
	if _, err := r.ApplyBonus(); err != nil {
		return ref, err
	}

	// Try to save referrer
	err := r.Put()

	return ref, err
}
