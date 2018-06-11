package subscription

import (
	"strconv"
	"time"

	"github.com/stripe/stripe-go"

	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/models/mixin"
	"crowdstart.com/models/plan"
	"crowdstart.com/util/hashid"
	"crowdstart.com/util/json"
	"crowdstart.com/util/val"

	. "crowdstart.com/models"
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

type Subscription struct {
	mixin.Model

	Number int `json:"number,omitempty" datastore:"-"`

	// Payment source information
	Account Account `json:"account"`

	// Immutable buyer data from time of payment, may or may not be associated
	// with a user.
	Buyer Buyer `json:"buyer"`

	PlanId string `json:"planId"`
	UserId string `json:"userId"`

	FeePercent float64 `json:"application_fee_percent"`
	EndCancel  bool    `json:"cancel_at_period_end"`

	PeriodStart time.Time `json:"current_period_start"`
	PeriodEnd   time.Time `json:"current_period_end"`

	Start      time.Time `json:"start"`
	Ended      time.Time `json:"ended_at"`
	CanceledAt time.Time `json:"canceled_at"`

	TrialStart time.Time `json:"trial_start"`
	TrialEnd   time.Time `json:"trial_end"`

	Plan     plan.Plan `json:"plan"`
	Quantity int       `json:"quantity"`
	Status   string    `json:"status"`

	Metadata  Map    `json:"metadata" datastore:"-"`
	Metadata_ string `json:"-" datastore:"-"`

	// Stripe livemode
	Live bool `json:"live"`

	// Internal testing flag
	Test bool `json:"-"`
}

func New(db *datastore.Datastore) *Subscription {
	s := new(Subscription)
	s.Init()
	s.Model = mixin.Model{Db: db, Entity: s}
	return s
}

func (s *Subscription) Init() {
	s.Metadata = make(Map)
}

func (s Subscription) Kind() string {
	return "subscription"
}

func (s Subscription) ToCard() *stripe.CardParams {
	card := stripe.CardParams{}
	card.Name = s.Buyer.Name()
	card.Number = s.Account.Number
	card.CVC = s.Account.CVC
	card.Month = strconv.Itoa(s.Account.Month)
	card.Year = strconv.Itoa(s.Account.Year)
	card.Address1 = s.Buyer.Address.Line1
	card.Address2 = s.Buyer.Address.Line2
	card.City = s.Buyer.Address.City
	card.State = s.Buyer.Address.State
	card.Zip = s.Buyer.Address.PostalCode
	card.Country = s.Buyer.Address.Country
	return &card
}

func (s *Subscription) Load(c <-chan aeds.Property) (err error) {
	// Ensure we're initialized
	s.Init()

	// Load supported properties
	if err = IgnoreFieldMismatch(aeds.LoadStruct(s, c)); err != nil {
		return err
	}

	// Set order number
	s.Number = s.NumberFromId()

	// Deserialize from datastore
	if len(s.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(s.Metadata_), &s.Metadata)
	}

	return err
}

func (s *Subscription) Save(c chan<- aeds.Property) (err error) {
	// Serialize unsupported properties
	s.Metadata_ = string(json.EncodeBytes(&s.Metadata))

	if err != nil {
		return err
	}

	// Save properties
	return IgnoreFieldMismatch(aeds.SaveStruct(s, c))
}

func (s *Subscription) Validator() *val.Validator {
	return val.New(s)
}

func (s Subscription) NumberFromId() int {
	if s.Id_ == "" {
		return -1
	}
	return hashid.Decode(s.Id_)[1]
}

func Query(db *datastore.Datastore) *mixin.Query {
	return New(db).Query()
}
