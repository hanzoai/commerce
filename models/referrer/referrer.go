package referrer

import (
	"time"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/transaction"
	"crowdstart.com/models/types/client"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

// Is a link that can refer customers to buy products
type Referrer struct {
	mixin.Model

	Code    string  `json:"code"`
	Program Program `json:"program"`
	OrderId string  `json:"orderId"`
	UserId  string  `json:"userId"`

	AffiliateId     string    `json:"affiliateId,omitempty"`
	FirstReferredAt time.Time `json:"firstReferredAt"`

	Client      client.Client `json:"-"`
	Blacklisted bool          `json:"blacklisted,omitempty"`
	Duplicate   bool          `json:"duplicate,omitempty"`

	RewardedReferrals int `json:"-"`
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
