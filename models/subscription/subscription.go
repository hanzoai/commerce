package subscription

import (
	"time"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/plan"
	"crowdstart.com/util/val"
)

// Based On Stripe Subscription
// Stripe\Subscription JSON: {
//   "id": "sub_7OTicGsP51uH9F",
//   "object": "subscription",
//   "application_fee_percent": null,
//   "cancel_at_period_end": false,
//   "canceled_at": null,
//   "current_period_end": 1450725048,
//   "current_period_start": 1448133048,
//   "customer": "cus_7OSfdiUiYYf0tS",
//   "discount": null,
//   "ended_at": null,
//   "metadata": {
//   },
//   "plan": {
//		...
//   },
//   "quantity": 1,
//   "start": 1448133048,
//   "status": "active",
//   "tax_percent": null,
//   "trial_end": null,
//   "trial_start": null
// }

type Subscription struct {
	mixin.Model

	PlanId string `json:"planId"`
	UserId string `json:"userId"`

	StripeId         string `json:"stripeId"`
	StripeCustomerId string `json:"customer"`

	ApplicationFeePercent float64 `json:"application_fee_percent"`
	CancelAtPeriodEnd     bool    `json:"cancel_at_period_end"`

	CurrentPeriodStart time.Time `json:"current_period_start"`
	CurrentPeriodEnd   time.Time `json:"current_period_end"`

	Start      time.Time `json:"start"`
	EndedAt    time.Time `json:"ended_at"`
	CanceledAt time.Time `json:"canceled_at"`

	TrialStart time.Time `json:"trial_start"`
	TrialEnd   time.Time `json:"trial_end"`

	Plan     plan.Plan `json:"plan"`
	Quantity int       `json:"quantity"`
	Status   string    `json:"status"`
}

func New(db *datastore.Datastore) *Subscription {
	p := new(Subscription)
	p.Model = mixin.Model{Db: db, Entity: p}
	return p
}

func (p Subscription) Kind() string {
	return "subscription"
}

func (p Subscription) Document() mixin.Document {
	return nil
}

func (p *Subscription) Validator() *val.Validator {
	return val.New(p)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
