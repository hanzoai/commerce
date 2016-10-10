package referrer

import (
	"time"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/transaction"
	"crowdstart.com/models/types/client"
	"crowdstart.com/util/log"
	"crowdstart.com/util/timeutil"
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
}

type Referrent interface {
	Id() string
	Kind() string
}

func (r *Referrer) SaveReferral(typ referral.Type, rfn Referrent) (*referral.Referral, error) {
	log.Debug("Creating referral")
	// Create new referral
	rfl := referral.New(r.Db)
	rfl.Type = typ
	rfl.Referrer.Id = r.Id()
	rfl.Referrer.AffiliateId = r.AffiliateId
	rfl.Referrer.UserId = r.UserId

	// Save referrent's id
	switch rfn.Kind() {
	case "order":
		log.Debug("Saving referral for new order")
		rfl.OrderId = rfn.Id()
	case "user":
		log.Debug("Saving referral for new user")
		rfl.UserId = rfn.Id()
	}

	log.JSON("Saving referral", rfl)

	// Try to save referral
	if err := rfl.Create(); err != nil {
		return rfl, err
	}

	// If this is the first referral, update referrer
	if timeutil.IsZero(r.FirstReferredAt) {
		r.FirstReferredAt = time.Now()
		r.Update()
	}

	// Apply any program actions if they are configured
	if len(r.Program.Actions) > 0 {
		if err := r.Program.ApplyActions(r); err != nil {
			return rfl, err
		}
	}

	return rfl, nil
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
