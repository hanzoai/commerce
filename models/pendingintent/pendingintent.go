// Package pendingintent records, BEFORE a buyer is sent to their bank, the
// payment a card's authentication step (3-D Secure) is waiting on: whose it is,
// what it is for and how much. A Stripe PaymentIntent that needs authentication
// has moved no money; it moves some when the buyer finishes the step in their
// browser, which may be after they have closed the tab. This record is what lets
// the intent's webhook, or the reconciler, put that money where it belongs
// without the browser ever calling back.
//
// Keyed by the PaymentIntent id, in the system namespace, like paymentorg.
package pendingintent

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
	orm.Register[Intent]("pending-intent", orm.WithStringKey[Intent]())
}

// Purposes: what a pending intent pays for.
const (
	Topup  = "topup"  // credits subject's wallet
	Reload = "reload" // credits subject's wallet, charged off-session by the auto-reload
	Sale   = "sale"   // a plan's first period: finished by the sale itself, refunded if it never is
)

// Intent is one PaymentIntent waiting on its buyer.
type Intent struct {
	mixin.Model[Intent]

	Org      string `json:"org"`
	Subject  string `json:"subject"`
	Purpose  string `json:"purpose"`
	Cents    int64  `json:"cents"`
	Currency string `json:"currency"`
	Test     bool   `json:"test"`
	// Done says the intent was settled — credited, finished by its sale, or
	// refunded — and is never settled again.
	Done bool `json:"done,omitempty"`
}

func (i *Intent) Load(ps []datastore.Property) error  { return datastore.LoadStruct(i, ps) }
func (i *Intent) Save() ([]datastore.Property, error) { return datastore.SaveStruct(i) }

func db() *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(context.Background(), "system"))
}

// Put records intent id as waiting on subject in org, for purpose. Idempotent.
func Put(id, org, subject, purpose string, cents int64, currency string, test bool) error {
	id = strings.TrimSpace(id)
	if id == "" || org == "" || subject == "" {
		return nil
	}
	if _, found, err := Get(id); err != nil || found {
		return err
	}
	d := db()
	i := new(Intent)
	i.Init(d)
	i.SetId(id)
	i.Org, i.Subject, i.Purpose, i.Cents, i.Currency, i.Test = org, subject, purpose, cents, strings.ToLower(currency), test
	return i.Put()
}

// Get answers the intent recorded under id.
func Get(id string) (*Intent, bool, error) {
	d := db()
	i := new(Intent)
	i.Init(d)
	err := i.Get(d.NewKey(i.Kind(), strings.TrimSpace(id), 0, nil))
	switch {
	case err == nil:
		return i, true, nil
	case err == datastore.ErrNoSuchEntity:
		return nil, false, nil
	default:
		return nil, false, err
	}
}

// Settle marks id done, once: it answers true to the writer that did it.
func Settle(id string) (bool, error) {
	for range mixin.Swaps {
		i, found, err := Get(id)
		if err != nil || !found || i.Done {
			return false, err
		}
		i.Done = true
		ok, err := i.Swap()
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// Open lists the intents not yet settled that were recorded before cutoff.
func Open(cutoff time.Time) ([]*Intent, error) {
	rows := make([]*Intent, 0)
	if _, err := db().Query("pending-intent").Filter("Done=", false).GetAll(&rows); err != nil {
		return nil, err
	}
	var out []*Intent
	for _, r := range rows {
		if r.CreatedAt.Before(cutoff) {
			out = append(out, r)
		}
	}
	return out, nil
}
