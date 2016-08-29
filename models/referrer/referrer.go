package referrer

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/transaction"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Referrer struct {
	mixin.Model

	Code           string                    `json:"code"`
	Program        Program                   `json:"program"`
	OrderId        string                    `json:"orderId"`
	UserId         string                    `json:"userId"`
	AffiliateId    string                    `json:"affiliateId,omitempty"`
	ReferralIds    []string                  `json:"referralIds"`
	TransactionIds []string                  `json:"transactionsIds"`
	Transactions   []transaction.Transaction `json:"transactions,omitempty"`
}

func (r *Referrer) ApplyBonus() (*transaction.Transaction, error) {
	trans := transaction.New(r.Db)
	r.Program.GetBonus(trans, len(r.ReferralIds))
	trans.UserId = r.UserId
	trans.Type = transaction.Deposit
	if err := trans.Put(); err != nil {
		return nil, err
	}
	r.TransactionIds = append(r.TransactionIds, trans.Id())
	trans.Notes = "Deposite due to referral"
	trans.Tags = "referral"

	trans.SourceId = r.Id()
	trans.SourceKind = r.Kind()

	return trans, nil
}

func (r *Referrer) SaveReferral(orderId, userId string) (*referral.Referral, error) {
	ref := referral.New(r.Db)
	ref.ReferrerUserId = r.UserId
	ref.OrderId = orderId
	ref.UserId = userId
	ref.ReferrerId = r.Id()

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
