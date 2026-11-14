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
	"sort"
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
	// Invoice is a past-due invoice paid on-session after an off-session renewal
	// needed its bank: finished by the next press of "pay", refunded if never.
	Invoice = "invoice"
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
	// PaidCents and PaidCurrency are what the buyer was asked to pay when it is not
	// the credit: a local method (Pix, iDEAL, ...) takes the buyer's own currency, and
	// the quote made at the start fixed how many of those buy Cents of Currency. The
	// intent settles only for exactly this, and credits Cents.
	// PolicyVersion is the routing policy version the payment was opened under; a later
	// change to the policy moves nothing already opened.
	PolicyVersion int    `json:"policyVersion,omitempty"`
	PaidCents     int64  `json:"paidCents,omitempty"`
	PaidCurrency  string `json:"paidCurrency,omitempty"`
	// Ref names what the intent pays beyond the buyer: the invoice, for Invoice.
	Ref string `json:"ref,omitempty"`
	// Tries is how many sweeps failed to settle it, Next the earliest the next one
	// may try (exponential back-off), Last why the last one failed.
	Tries int       `json:"tries,omitempty"`
	Next  time.Time `json:"next,omitempty"`
	Last  string    `json:"last,omitempty"`
	// Terms the quote was made on, bound to this attempt when it was opened: the rate
	// (local per dollar, major units), where the rate came from
	// ("processor" or "market"), the processor's locked quote id, and the rate before
	// the processor's own conversion fee.
	Rate       float64 `json:"rate,omitempty"`
	RateSource string  `json:"rateSource,omitempty"`
	QuoteID    string  `json:"quoteId,omitempty"`
	BaseRate   float64 `json:"baseRate,omitempty"`
	// FeeRate and Premium are the processor's conversion fee and lock premium as
	// fractions of the amount: costs, recorded with the attempt, never in the price.
	FeeRate float64 `json:"feeRate,omitempty"`
	Premium float64 `json:"premium,omitempty"`
	// ExchangeRate is Stripe's effective rate (USD per unit of the paid currency),
	// ReferenceRate and ReferenceProvider its market reference, FXFee who bears the
	// conversion cost ("pass" or "absorb"), LockExpiresAt when the locked quote lapses.
	ExchangeRate      float64 `json:"exchangeRate,omitempty"`
	ReferenceRate     float64 `json:"referenceRate,omitempty"`
	ReferenceProvider string  `json:"referenceProvider,omitempty"`
	FXFee             string  `json:"fxCost,omitempty"`
	LockExpiresAt     int64   `json:"lockExpiresAt,omitempty"`
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
	return PutRef(id, org, subject, purpose, "", cents, currency, test)
}

// PutRef is Put for an intent that also names what it pays (an invoice).
func PutRef(id, org, subject, purpose, ref string, cents int64, currency string, test bool) error {
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
	i.Org, i.Subject, i.Purpose, i.Ref, i.Cents, i.Currency, i.Test = org, subject, purpose, ref, cents, strings.ToLower(currency), test
	return i.Put()
}

// PutQuoted is Put for a local-method intent: Cents of Currency is the credit, and
// the buyer pays paidCents of paidCurrency for it.
func PutQuoted(id, org, subject, purpose string, cents int64, cur string, paidCents int64, paidCur string, version int, test bool) error {
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
	i.Org, i.Subject, i.Purpose, i.Cents, i.Currency, i.Test = org, subject, purpose, cents, strings.ToLower(cur), test
	i.PaidCents, i.PaidCurrency, i.PolicyVersion = paidCents, strings.ToLower(paidCur), version
	return i.Put()
}

// Terms are the rate terms of a quoted intent.
type Terms struct {
	Rate       float64
	RateSource string
	QuoteID    string
	BaseRate   float64
	FeeRate    float64
	Premium    float64

	ExchangeRate      float64
	ReferenceRate     float64
	ReferenceProvider string
	FXFee             string
	LockExpiresAt     int64
}

// Bind records the terms the intent's quote was made on. They are written once, when the
// attempt is opened, and never changed: the rate disclosed is the rate charged.
func Bind(id string, t Terms) error {
	i, found, err := Get(id)
	if err != nil || !found {
		return err
	}
	if i.RateSource != "" {
		return nil
	}
	i.Rate, i.RateSource, i.QuoteID, i.BaseRate, i.FeeRate, i.Premium = t.Rate, t.RateSource, t.QuoteID, t.BaseRate, t.FeeRate, t.Premium
	i.ExchangeRate, i.ReferenceRate, i.ReferenceProvider, i.FXFee, i.LockExpiresAt = t.ExchangeRate, t.ReferenceRate, t.ReferenceProvider, t.FXFee, t.LockExpiresAt
	return i.Put()
}

// Charged is what the processor is asked to take and must settle: the quoted amount
// in the buyer's currency for a local method, else the credit itself.
func (i *Intent) Charged() (int64, string) {
	if i.PaidCurrency != "" {
		return i.PaidCents, i.PaidCurrency
	}
	return i.Cents, i.Currency
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

// Open lists the intents not yet settled that were recorded before cutoff and whose
// back-off has passed, oldest first.
func Open(cutoff time.Time) ([]*Intent, error) {
	rows := make([]*Intent, 0)
	if _, err := db().Query("pending-intent").Filter("Done=", false).GetAll(&rows); err != nil {
		return nil, err
	}
	now := time.Now()
	var out []*Intent
	for _, r := range rows {
		if r.CreatedAt.Before(cutoff) && !r.Next.After(now) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

// Failed records that a sweep could not settle id, and backs the next try off:
// 5 minutes doubling to 6 hours. It answers how many sweeps have failed.
func Failed(id, why string) (int, error) {
	for range mixin.Swaps {
		i, found, err := Get(id)
		if err != nil || !found {
			return 0, err
		}
		i.Tries++
		wait := 5 * time.Minute << min(i.Tries-1, 6)
		i.Next, i.Last = time.Now().Add(min(wait, 6*time.Hour)), why
		ok, err := i.Swap()
		if err != nil {
			return 0, err
		}
		if ok {
			return i.Tries, nil
		}
	}
	return 0, nil
}

// Wait defers id's next look until `until`, without counting a failure: a row that
// is only waiting on its buyer.
func Wait(id string, until time.Time) error {
	for range mixin.Swaps {
		i, found, err := Get(id)
		if err != nil || !found {
			return err
		}
		i.Next = until
		ok, err := i.Swap()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return nil
}

// OpenFor is the unsettled intent that pays ref for subject, or nil.
func OpenFor(subject, ref string) (*Intent, error) {
	rows := make([]*Intent, 0)
	if _, err := db().Query("pending-intent").Filter("Subject=", subject).Filter("Done=", false).GetAll(&rows); err != nil {
		return nil, err
	}
	var best *Intent
	for _, r := range rows {
		if r.Ref == ref && (best == nil || r.CreatedAt.After(best.CreatedAt)) {
			best = r
		}
	}
	return best, nil
}

// CountOpen is how many intents subject has waiting.
func CountOpen(subject string) (int, error) {
	rows := make([]*Intent, 0)
	if _, err := db().Query("pending-intent").Filter("Subject=", subject).Filter("Done=", false).GetAll(&rows); err != nil {
		return 0, err
	}
	return len(rows), nil
}

// Since lists the subject's intents for purpose recorded at or after since, newest
// first, settled or not.
func Since(subject, purpose string, since time.Time) ([]*Intent, error) {
	rows := make([]*Intent, 0)
	if _, err := db().Query("pending-intent").Filter("Subject=", subject).GetAll(&rows); err != nil {
		return nil, err
	}
	var out []*Intent
	for _, r := range rows {
		if r.Purpose == purpose && !r.CreatedAt.Before(since) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
