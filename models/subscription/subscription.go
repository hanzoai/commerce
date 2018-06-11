package subscription

import (
	"time"
	"errors"

	aeds "google.golang.org/appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/mixin"
	"hanzo.io/models/payment"
	"hanzo.io/models/plan"
	"hanzo.io/util/hashid"
	"hanzo.io/util/json"
	"hanzo.io/util/val"

	. "hanzo.io/models"
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

var IgnoreFieldMismatch = datastore.IgnoreFieldMismatch

type BillingType string

const (
	Charge	BillingType = "charge_automatically"
	Invoice BillingType = "send_invoice"
)

type Status string

const (
	Trialing	Status = "trialing"
	Active		Status = "active"
	PastDue		Status = "past_due"
	Canceled	Status = "canceled"
	Unpaid		Status = "unpaid"
)

type Subscription struct {
	mixin.Model

	Number int `json:"number,omitempty" datastore:"-"`

	// Immutable buyer data from time of payment, may or may not be associated
	// with a user.
	Buyer Buyer `json:"buyer"`

	Type BillingType `json:"billing_type"`

	PlanId string `json:"planId"`
	UserId string `json:"userId"`

	FeePercent float64 `json:"application_fee_percent"`
	EndCancel  bool    `json:"cancel_at_period_end"`

	PeriodStart time.Time `json:"current_period_start"`
	PeriodEnd   time.Time `json:"current_period_end"`

	Start      time.Time `json:"start"`
	Ended      time.Time `json:"ended_at"`
	Canceled bool `json:"canceled"`
	CanceledAt time.Time `json:"canceled_at"`

	TrialStart time.Time `json:"trial_start"`
	TrialEnd   time.Time `json:"trial_end"`

	Plan     plan.Plan `json:"plan"`
	Quantity int       `json:"quantity"`
	Status   Status	   `json:"status"`

	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:"-"`

	// Stripe livemode
	Live bool `json:"live"`

	// Internal testing flag
	Test bool `json:"-"`

	StripeAccount payment.Account `json:"stripeAccount,omitEmpty"`
	StripeSubscriptionId string `json:"stripeSubscriptionId,omitEmpty"`
}

func (s *Subscription) Load(ps []aeds.Property) (err error) {
	// Ensure we're initialized

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(s, ps)); err != nil {
		return err
	}

	// Set order number
	num, err := s.NumberFromId()
	if err != nil {
		return err
	}
	s.Number = num

	// Deserialize from datastore
	if len(s.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(s.Metadata_), &s.Metadata)
	}

	return err
}

func (s *Subscription) Save() (ps []aeds.Property, err error) {
	// Serialize unsupported properties
	s.Metadata_ = string(json.EncodeBytes(&s.Metadata))

	if err != nil {
		return nil, err
	}

	// Save properties
	return datastore.SaveStruct(s)
}

func (s *Subscription) Validator() *val.Validator {
	return val.New()
}

func (s Subscription) NumberFromId() (i int, err error) {
	if s.Id_ == "" {
		return -1, errors.New("Subscription.NumberFromID(): Blank ID passed.")
	}

	ret, err := hashid.Decode(s.Id_)

	return ret[1], err
}

