package plan

import (
	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/json"
	"crowdstart.com/util/val"

	. "crowdstart.com/models"
)

type Interval string

const (
	Yearly  Interval = "year"
	Monthly          = "month"
)

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

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

	Price           currency.Cents `json:"price"`
	Currency        currency.Type  `json:"currency"`
	Interval        Interval       `json:"interval"`
	IntervalCount   int            `json:"intervalCount"`
	TrialPeriodDays int            `json:"trialPeriodDays"`

	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:"-"`
}

func New(db *datastore.Datastore) *Plan {
	p := new(Plan)
	p.Model = mixin.Model{Db: db, Entity: p}
	return p
}

func (p *Plan) Init() {
	p.Metadata = make(Map)
}

func (p Plan) Kind() string {
	return "plan"
}

func (p *Plan) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	p.Init()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(p, c)); err != nil {
		return err
	}

	// Deserialize from datastore
	if len(p.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(p.Metadata_), &p.Metadata)
	}

	return err
}

func (p *Plan) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	p.Metadata_ = string(json.EncodeBytes(&p.Metadata))

	if err != nil {
		return err
	}

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(p, c))
}

func (p *Plan) Validator() *val.Validator {
	return val.New(p)
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
