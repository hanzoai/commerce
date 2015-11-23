package plan

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/val"
)

type Interval string

const (
	Year  Interval = "year"
	Month          = "month"
)

// Based On Stripe Plan
// Stripe\Plan JSON: {
//   "id": "gold21323",
//   "object": "plan",
//   "amount": 2000,
//   "created": 1386247539,
//   "currency": "usd",
//   "interval": "month",
//   "interval_count": 1,
//   "livemode": false,
//   "metadata": {
//   },
//   "name": "New plan name",
//   "statement_descriptor": null,
//   "trial_period_days": null
// }

type Plan struct {
	mixin.Model

	// Unique human readable id
	Slug string `json:"slug"`
	// Internal id
	SKU string `json:"sku"`

	StripeId string `json:"stripeId"`

	// Human readable name
	Name        string `json:"name"`
	Description string `json:"description"`

	Amount          currency.Cents `json:"amount"`
	Currency        currency.Type  `json:"currency"`
	Interval        Interval       `json:"interval"`
	IntervalCount   int            `json:"intervalCount"`
	TrialPeriodDays int            `json:"trialPeriodDays"`
}

func New(db *datastore.Datastore) *Plan {
	p := new(Plan)
	p.Model = mixin.Model{Db: db, Entity: p}
	return p
}

func (p Plan) Kind() string {
	return "plan"
}

func (p Plan) Document() mixin.Document {
	return nil
}

func (p *Plan) Validator() *val.Validator {
	return val.New(p)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
