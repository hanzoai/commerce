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

// Is a link that can refer customers to buy products
type Referrer struct {
	mixin.Model

	Code            string    `json:"code"`
	Program         Program   `json:"program"`
	OrderId         string    `json:"orderId"`
	UserId          string    `json:"userId"`
	AffiliateId     string    `json:"affiliateId,omitempty"`
	FirstReferredAt time.Time `json:"firstReferredAt"`
}

func (r *Referrer) SaveReferral(orderId, userId string) (*referral.Referral, error) {
	ref := referral.New(r.Db)
	ref.ReferrerUserId = r.UserId
	ref.OrderId = orderId
	ref.UserId = userId
	ref.ReferrerId = r.Id()

	// Try to save referral
	if err := ref.Create(); err != nil {
		return ref, err
	}

	// If this is the first referral, update referrer and affiliate
	if timeutil.IsZero(r.FirstReferredAt) {
		r.FirstReferredAt = time.Now()
		r.Update()

		if r.AffiliateId != "" {
			aff := affiliate.New(r.Db)
			if err := aff.Get(r.AffiliateId); err != nil {
				aff.Schedule.StartAt = r.FirstReferredAt
				aff.Update()
			}
		}
	}

	// Apply any program actions if they are configured
	if len(r.Program.Actions) > 0 {
		if err := r.Program.ApplyActions(r); err != nil {
			return ref, err
		}
	}

	return ref, nil
}

func (r *Referrer) Referrals() ([]*referral.Referral, error) {
	referrals := make([]*referral.Referral, 0)
	_, err := referral.Query(r.Db).Filter("ReferrerId=", r.Id()).GetAll(referrals)
	return referrals, err
}

func (r *Referrer) Transactions() ([]*transaction.Transaction, error) {
	transactions := make([]*transaction.Transaction, 0)
	_, err := transaction.Query(r.Db).Filter("ReferrerId=", r.Id()).GetAll(transactions)
	return transactions, err
}
