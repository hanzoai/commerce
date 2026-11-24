// Package checkoutsession records a checkout an agent opens for a buyer: what is being
// bought, at what USD price, and how it ended. It is the thread a payment is bound to:
// the session id, the buyer's account and the idempotency key a completion arrived with
// together name one charge, so completing twice is one purchase.
//
// Keyed by the session id, in the system namespace, like pendingintent.
package checkoutsession

import (
	"context"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/orm"
)

func init() {
	orm.Register[Session]("checkout-session", orm.WithStringKey[Session]())
}

// Statuses of a session.
const (
	Open     = "open"     // may still be changed and completed
	Complete = "complete" // paid; Order names what it bought
	Pending  = "pending"  // payment started and waiting on the buyer's bank or a deposit
	Canceled = "canceled"
	Failed   = "failed" // a completion was refused; the session stays usable
)

// Line kinds.
const (
	Credit = "credit" // USD credit for the wallet
	Plan   = "plan"   // a subscription plan's first period
)

// Session is one checkout.
type Session struct {
	mixin.Model[Session]

	Org     string `json:"org"`
	Subject string `json:"subject"`
	Test    bool   `json:"test"`
	Status  string `json:"status"`

	// The one line: Line credit with UsdCents, or plan with Plan and Interval.
	Line     string `json:"line"`
	UsdCents int64  `json:"usdCents"`
	Plan     string `json:"plan,omitempty"`
	Level    int    `json:"level,omitempty"`
	Quantity int    `json:"quantity,omitempty"`
	Interval string `json:"interval,omitempty"`
	Coupon   string `json:"coupon,omitempty"`
	// TotalCents is what completing charges, in USD cents, fixed from the catalog when the
	// session was opened or last updated.
	TotalCents int64 `json:"totalCents"`

	// How it was paid and the key that completed it.
	Payment string `json:"payment,omitempty"` // "shared_payment_token" | "stablecoin"
	Key     string `json:"key,omitempty"`
	// Intent is the processor payment a stablecoin deposit tracks.
	Intent string `json:"intent,omitempty"`
	// Order is what a completed session bought: the receipt of the credit, the
	// subscription, the charge reference.
	PaymentID      string    `json:"paymentId,omitempty"`
	SubscriptionID string    `json:"subscriptionId,omitempty"`
	Reason         string    `json:"reason,omitempty"`
	CompletedAt    time.Time `json:"completedAt,omitempty"`
}

func (s *Session) Load(ps []datastore.Property) error  { return datastore.LoadStruct(s, ps) }
func (s *Session) Save() ([]datastore.Property, error) { return datastore.SaveStruct(s) }

func db() *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(context.Background(), "system"))
}

// Put stores the session under its id.
func Put(s *Session) error {
	d := db()
	s.Init(d)
	return s.Put()
}

// Get answers the session recorded under id.
func Get(id string) (*Session, bool, error) {
	d := db()
	s := new(Session)
	s.Init(d)
	err := s.Get(d.NewKey(s.Kind(), strings.TrimSpace(id), 0, nil))
	switch {
	case err == nil:
		return s, true, nil
	case err == datastore.ErrNoSuchEntity:
		return nil, false, nil
	default:
		return nil, false, err
	}
}

// Change applies f to the session under id and stores it, retrying when another writer got
// there first. f answers false to leave the session as it is.
func Change(id string, f func(*Session) bool) (*Session, bool, error) {
	for range mixin.Swaps {
		s, found, err := Get(id)
		if err != nil || !found {
			return nil, false, err
		}
		if !f(s) {
			return s, false, nil
		}
		ok, err := s.Swap()
		if err != nil {
			return nil, false, err
		}
		if ok {
			return s, true, nil
		}
	}
	return nil, false, nil
}
