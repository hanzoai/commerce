// Package paymentorg records, for every settled payment, the org whose books its
// receipt lives in — by the processor's payment id, in the system namespace.
//
// A processor's callback names a payment and no org: Square sends no X-Org-Id, and a
// sessionless request resolves to the default org. So a refund or a dispute of a
// customer's top-up, or of the payment for a plan's period, is traced to its org
// here, never read off the request.
package paymentorg

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/dispute"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/orm"
)

func init() {
	orm.Register[PaymentOrg]("payment-org", orm.WithStringKey[PaymentOrg]())
}

// PaymentOrg is where one payment's receipt lives: the org, the wallet it credited,
// and whether the charge was a sandbox one. A payment that paid an invoice names it
// in Invoice; Balance is the part of it, if any, that it credited to the wallet as
// well (an organization's starting balance, bought with its seats), and the part a
// refund or a dispute takes back from the wallet first. Report is a dispute of a
// payment that credited a wallet, as the processor last reported it.
type PaymentOrg struct {
	mixin.Model[PaymentOrg]

	Org     string `json:"org"`
	Subject string `json:"subject"`
	Invoice string `json:"invoice,omitempty"`
	Balance int64  `json:"balance,omitempty"`
	Test    bool   `json:"test"`

	// A card sale's payment is recorded the moment the charge settles (Open),
	// before anything else the sale writes: Sale says so, and that it is never a
	// wallet's top-up. Order is the storage id of the sale's retry guard, which
	// holds its receipt. An order keeps its starting balance's state here too.
	// Credited says the credit is decided, Credit is its amount — Balance less what
	// refunds had taken by then — and Deposit the credit's receipt once it landed.
	// Takes are what each refund and dispute took of the balance, credited or not:
	// a refund taken before the credit is decided is netted from it; a dispute's
	// take is credited and then taken from the wallet (Moved), so a dispute won
	// gives back exactly what its loss took, through the ledger that took it.
	Sale bool `json:"sale,omitempty"`
	// Via is the processor that took the payment ("square", "wire"), what its
	// credit's deposit names as its source.
	Via      string `json:"via,omitempty"`
	Order    string `json:"order,omitempty"`
	Credited bool   `json:"credited,omitempty"`
	Credit   int64  `json:"credit,omitempty"`
	Deposit  string `json:"deposit,omitempty"`
	Takes    []Take `json:"takes,omitempty"`

	dispute.Report
}

func (p *PaymentOrg) Load(ps []datastore.Property) error  { return datastore.LoadStruct(p, ps) }
func (p *PaymentOrg) Save() ([]datastore.Property, error) { return datastore.SaveStruct(p) }

func db() *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(context.Background(), "system"))
}

// Put records that payment's receipt lives in org, for subject, the invoice it
// paid ("" for a payment that credited subject's wallet) and the cents of it that
// credited the wallet beside an invoice. Idempotent: the payment id is the key,
// and a payment already recorded keeps what was taken of it and credited since.
func Put(payment, org, subject, invoice string, balance int64, test bool) error {
	payment = strings.TrimSpace(payment)
	if payment == "" || org == "" {
		return nil
	}
	if _, found, err := Change(payment, func(p *PaymentOrg) bool {
		p.Org, p.Subject, p.Invoice, p.Balance, p.Test = org, subject, invoice, balance, test
		return true
	}); err != nil || found {
		return err
	}
	d := db()
	p := new(PaymentOrg)
	p.Init(d)
	p.SetId(payment)
	p.Org, p.Subject, p.Invoice, p.Balance, p.Test = org, subject, invoice, balance, test
	return p.Put()
}

// Take is what one refund or dispute (Ref: "refund:<id>", "dispute:<id>") took of
// an order's starting balance. Moved says it was taken out of the wallet, the
// credit having been decided before it; otherwise the credit nets against it.
// Restored says a dispute won gave it back.
type Take struct {
	Ref      string `json:"ref"`
	Cents    int64  `json:"cents"`
	Moved    bool   `json:"moved,omitempty"`
	Restored bool   `json:"restored,omitempty"`
}

// Took is the take ref recorded, or nil.
func (p *PaymentOrg) Took(ref string) *Take {
	for i := range p.Takes {
		if p.Takes[i].Ref == ref {
			return &p.Takes[i]
		}
	}
	return nil
}

// Taken is what refunds and disputes hold of the starting balance now.
func (p *PaymentOrg) Taken() int64 {
	n := int64(0)
	for _, t := range p.Takes {
		if !t.Restored {
			n += t.Cents
		}
	}
	return n
}

// Refunded is what refunds took of the starting balance: refunds are final, so a
// credit decided after them nets them.
func (p *PaymentOrg) Refunded() int64 {
	n := int64(0)
	for _, t := range p.Takes {
		if strings.HasPrefix(t.Ref, "refund:") {
			n += t.Cents
		}
	}
	return n
}

// Decide decides the credit of an order's starting balance, once: the balance
// less what refunds took, every dispute's standing take moved to the wallet (the
// credit includes it and the wallet gives it up), and answers whether it decided
// now.
func (p *PaymentOrg) Decide() bool {
	if p.Credited {
		return false
	}
	p.Credited, p.Credit = true, max(p.Balance-p.Refunded(), 0)
	for i := range p.Takes {
		if !p.Takes[i].Restored && !strings.HasPrefix(p.Takes[i].Ref, "refund:") {
			p.Takes[i].Moved = true
		}
	}
	return true
}

// Left is the starting balance no refund or dispute holds.
func (p *PaymentOrg) Left() int64 {
	return max(p.Balance-p.Taken(), 0)
}

// Open records a card sale's payment before the sale writes anything else: its
// org, its wallet, the guard holding its receipt ("" when it had none) and the
// starting balance it bought, if any. It writes only when nothing recorded the
// payment, so a sale run again after a crash never resets what was taken or
// credited since.
func Open(payment, org, subject, order, via string, balance int64, test bool) error {
	payment = strings.TrimSpace(payment)
	if payment == "" || org == "" {
		return fmt.Errorf("paymentorg: a payment record names no payment or org")
	}
	for lost := range mixin.Swaps {
		mixin.Pause(lost)
		if _, found, err := Get(payment); err != nil || found {
			return err
		}
		d := db()
		p := new(PaymentOrg)
		p.Init(d)
		p.SetId(payment)
		p.Org, p.Subject, p.Order, p.Via, p.Balance, p.Test, p.Sale = org, subject, order, via, balance, test, true
		if ok, err := p.Swap(); err != nil || ok {
			return err
		}
	}
	return fmt.Errorf("payment %s changed under %d reads in a row", payment, mixin.Swaps)
}

// Orders lists the payments of subject's orders in org — the payments that bought
// a starting balance under a sale's guard — by the processor's payment id.
func Orders(org, subject string) ([]string, error) {
	rows := make([]*PaymentOrg, 0)
	keys, err := db().Query("payment-org").Filter("Org=", org).Filter("Subject=", subject).GetAll(&rows)
	if err != nil {
		return nil, err
	}
	var out []string
	for i, r := range rows {
		if i < len(keys) && r.Order != "" && r.Balance > 0 {
			out = append(out, keys[i].StringID())
		}
	}
	return out, nil
}

// Owing lists the payments of every order whose starting balance has not reached
// its wallet: a credit not decided yet, or decided and not landed.
func Owing() ([]*PaymentOrg, []string, error) {
	rows := make([]*PaymentOrg, 0)
	keys, err := db().Query("payment-org").Filter("Balance>", 0).GetAll(&rows)
	if err != nil {
		return nil, nil, err
	}
	var owed []*PaymentOrg
	var ids []string
	for i, r := range rows {
		if i < len(keys) && r.Order != "" && r.Deposit == "" && (!r.Credited || r.Credit > 0) {
			owed, ids = append(owed, r), append(ids, keys[i].StringID())
		}
	}
	return owed, ids, nil
}

// Change applies change to payment's record and swaps it in, reading it again
// when another writer changed it first (mixin.Model.Swap), so two changes of one
// payment — a credit and a refund, two refunds — are applied one after the
// other. A change that answers false writes nothing. A payment nothing recorded
// answers found=false.
func Change(payment string, change func(*PaymentOrg) bool) (*PaymentOrg, bool, error) {
	for lost := range mixin.Swaps {
		mixin.Pause(lost)
		p, found, err := Get(payment)
		if err != nil || !found {
			return nil, found, err
		}
		if !change(p) {
			return p, true, nil
		}
		ok, err := p.Swap()
		if err != nil {
			return nil, true, err
		}
		if ok {
			return p, true, nil
		}
	}
	return nil, true, fmt.Errorf("payment %s changed under %d reads in a row", payment, mixin.Swaps)
}

// Get answers where payment's receipt lives, and false when nothing recorded it.
func Get(payment string) (*PaymentOrg, bool, error) {
	d := db()
	p := new(PaymentOrg)
	p.Init(d)
	err := p.Get(d.NewKey(p.Kind(), strings.TrimSpace(payment), 0, nil))
	switch {
	case err == nil:
		return p, true, nil
	case err == datastore.ErrNoSuchEntity:
		return nil, false, nil
	default:
		return nil, false, err
	}
}

// RecordDispute records a reported state of a dispute of payment on its record and
// reports whether it is the latest known (dispute.Report.SetDispute). A payment
// nothing recorded answers false.
//
// The record is swapped in (mixin.Model.Swap): a report another writer beat to it
// reads it again, so no report is lost to another, on this replica or another.
func RecordDispute(payment, state string, version int64, at time.Time) (bool, error) {
	for lost := range mixin.Swaps {
		mixin.Pause(lost)
		p, found, err := Get(payment)
		if err != nil || !found {
			return false, err
		}
		if !p.SetDispute(state, version, at) {
			return false, nil
		}
		ok, err := p.Swap()
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, fmt.Errorf("payment %s changed under %d reads in a row", payment, mixin.Swaps)
}
