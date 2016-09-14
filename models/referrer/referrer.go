package referrer

import (
	"time"

	"crowdstart.com/datastore"
	"crowdstart.com/models/affiliate"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/transaction"
	"crowdstart.com/util/timeutil"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type Referrer struct {
	mixin.Model

	Code            string    `json:"code"`
	Program         Program   `json:"program"`
	OrderId         string    `json:"orderId"`
	UserId          string    `json:"userId"`
	AffiliateId     string    `json:"affiliateId,omitempty"`
	FirstReferredAt time.Time `json:"firstReferredAt"`
}

func (r *Referrer) ApplyBonus() (*transaction.Transaction, error) {
	trans := transaction.New(r.Db)
	trans.UserId = r.UserId
	trans.Type = transaction.Deposit
	trans.Notes = "Deposite due to referral"
	trans.Tags = "referral"
	trans.SourceId = r.Id()
	trans.SourceKind = r.Kind()
	r.Program.GetBonus(trans)

	if err := trans.Put(); err != nil {
		return nil, err
	}

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

	if timeutil.IsZero(r.FirstReferredAt) {
		r.FirstReferredAt = time.Now()

		// Update affiliate if referral from affiliate
		if r.AffiliateId != "" {
			aff := affiliate.New(r.Db)
			if err := aff.Get(r.AffiliateId); err != nil {
				aff.Schedule.StartAt = r.FirstReferredAt
				aff.Update()
			}
		}

	}

	// Apply bonus if this is a referral for user
	if r.UserId != "" {
		if _, err := r.ApplyBonus(); err != nil {
			return ref, err
		}
	}

	return ref, nil
}
