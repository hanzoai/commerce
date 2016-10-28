package referrer

import (
	"fmt"
	"time"

	"crowdstart.com/datastore"
	"crowdstart.com/models/affiliate"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/transaction"
	"crowdstart.com/models/types/client"
	"crowdstart.com/util/delay"
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

	AffiliateId     string              `json:"affiliateId,omitempty"`
	Affiliate       affiliate.Affiliate `json:"affiliate,omitempty" datastore:"-"`
	FirstReferredAt time.Time           `json:"firstReferredAt"`

	Client      client.Client `json:"-"`
	Blacklisted bool          `json:"blacklisted,omitempty"`
	Duplicate   bool          `json:"duplicate,omitempty"`

	RewardedReferrals int `json:"-"`
}

type Referrent interface {
	Id() string
	Kind() string
}

type TimeInterval struct {
	Start time.Time // inclusive
	End   time.Time // exclusive
}

func IntervalFromInstant(t time.Time, width time.Duration) TimeInterval {
	start := t.Truncate(intervalWidth)
	end := start.Add(intervalWidth)
	return TimeInterval{start, end}
}

func NameFromInterval(d time.Duration, i TimeInterval, referrerId string) string {
	layout := time.RFC3339
	start := i.Start.Format(layout)
	end := i.End.Format(layout)
	// taskqueue supports delays specified in terms of microseconds:
	// https://github.com/golang/appengine/blob/75a29a66d4850a15c19eb6d70a31f5c453572be0/internal/taskqueue/taskqueue_service.pb.go#L456
	// https://github.com/golang/appengine/blob/75a29a66d4850a15c19eb6d70a31f5c453572be0/taskqueue/taskqueue.go#L176
	return fmt.Sprintf("processReferrals-%s-%d-from-%s-to-%s", referrerId, int64(d.Nanoseconds() / 1000), start, end)
}

const processingLatency = 5 * time.Minute
const intervalWidth = 10 * time.Minute

func (r *Referrer) SaveReferral(typ referral.Type, rfn Referrent, t time.Time, org *organization.Organization) (*referral.Referral, error) {
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

	batchProcessReferrals := delay.FuncByKey("batch-process-referrals")
	interval := IntervalFromInstant(t, intervalWidth)
	name := NameFromInterval(processingLatency, interval, r.Id())
	batchProcessReferrals.Once(r.Context(), name, processingLatency, interval, r.Id(), org.Id())
	return rfl, nil
}

func (r *Referrer) LoadAffiliate() error {
	if r.AffiliateId == "" {
		return nil
	}

	aff := affiliate.New(r.Db)

	if err := aff.GetById(r.AffiliateId); err != nil {
		return err
	}

	r.Affiliate = *aff

	return nil
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
