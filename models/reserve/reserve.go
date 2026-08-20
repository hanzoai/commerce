// Copyright © 2026 Hanzo AI. MIT License.

// Package reserve is the ledger of money a reserve control withheld.
//
// A reserve without a ledger is a haircut: a share comes off one payout and
// nothing anywhere says the money is held, how much has accumulated, whose it
// is, or when it comes back. That is not a control — it is an unrecorded
// deduction, and it is indefensible the first time a merchant asks.
//
// The accounting is the ordinary double-sided one. An ENTRY is an append-only
// movement in exact minor units, signed: positive is money withheld, negative
// is money released. A BALANCE is the running total for one (subject,
// currency), stored under a DETERMINISTIC key so reading it is one Get and
// never a scan of the ledger. Entries are the evidence; the balance is the
// answer.
//
// The balance is what the CEILING is enforced against (control.Cap), which is
// why the accounting has to be cumulative: a per-move ceiling bounds one
// payout, and the thing that needs bounding is the total ever taken.
//
// TENANCY. Every read and write here takes a datastore that is ALREADY
// namespaced to one org. No function in this package takes an org, so none can
// widen one, and the deterministic key is derived from the subject only —
// inside a namespace it names one row, and across namespaces the same subject
// is two different rows in two different stores.
package reserve

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/orm"
)

func init() {
	orm.Register[Entry]("risk-reserve-entry")
	orm.Register[Balance]("risk-reserve-balance", orm.WithStringKey[Balance]())
}

// Every model MUST satisfy mixin.Entity — see the note on control.Control.
var (
	_ mixin.Entity = (*Entry)(nil)
	_ mixin.Entity = (*Balance)(nil)
)

// Page is how many ledger rows one read answers with, and the most it can ever
// answer with. There is no unbounded read of a ledger.
const Page = 200

func bound(limit int) int {
	if limit <= 0 || limit > Page {
		return Page
	}
	return limit
}

// ErrShort refuses a release larger than the balance it releases from. A ledger
// that can go negative is a ledger that has invented money.
var ErrShort = errors.New("reserve: the release is larger than the balance held")

// Entry is one movement of withheld money: positive withheld, negative
// released. It is APPEND-ONLY — a correction is another entry, never an edit,
// because the sequence is the evidence.
type Entry struct {
	mixin.Model[Entry]

	SubjectKind string        `json:"subjectKind"`
	Subject     string        `json:"subject"`
	Currency    currency.Type `json:"currency,omitempty"`

	// Amount is exact minor units, signed: + withheld, − released.
	Amount int64 `json:"amount"`

	// Held is the balance AFTER this entry, so a row of the ledger is readable
	// on its own without summing everything before it.
	Held int64 `json:"held"`

	// Screen is the judgement that withheld the money and Control the standing
	// restraint that required it — the two things an appeal has to cite.
	Screen  string `json:"screen,omitempty"`
	Control string `json:"control,omitempty"`

	Reason string `json:"reason,omitempty"`

	// By is who caused it, from the validated principal, never off the wire.
	By string `json:"by,omitempty"`
}

func (e *Entry) Defaults() {
	e.Parent = e.Datastore().NewKey("synckey", "", 1, nil)
}

func (e *Entry) Load(ps []datastore.Property) error  { return datastore.LoadStruct(e, ps) }
func (e *Entry) Save() ([]datastore.Property, error) { return datastore.SaveStruct(e) }

// Balance is how much of one subject's money is withheld right now, in one
// currency. Its storage id is DETERMINISTIC, so the running total is one keyed
// read and two writers of the same subject collapse onto one row via the
// backend's upsert rather than forking the ledger.
type Balance struct {
	mixin.Model[Balance]

	SubjectKind string        `json:"subjectKind"`
	Subject     string        `json:"subject"`
	Currency    currency.Type `json:"currency,omitempty"`

	// Held is exact minor units currently withheld. It is never negative.
	Held int64 `json:"held"`

	// Entries counts the movements behind Held, so a reader can tell a balance
	// that was never touched from one that netted back to zero.
	Entries int64 `json:"entries,omitempty"`

	At time.Time `json:"at,omitempty"`
}

func (b *Balance) Load(ps []datastore.Property) error  { return datastore.LoadStruct(b, ps) }
func (b *Balance) Save() ([]datastore.Property, error) { return datastore.SaveStruct(b) }

func NewEntry(db *datastore.Datastore) *Entry {
	e := new(Entry)
	e.Init(db)
	e.Defaults()
	return e
}

func NewBalance(db *datastore.Datastore) *Balance {
	b := new(Balance)
	b.Init(db)
	return b
}

func EntryQuery(db *datastore.Datastore) datastore.Query { return db.Query("risk-reserve-entry") }

func BalanceQuery(db *datastore.Datastore) datastore.Query { return db.Query("risk-reserve-balance") }

// id is the deterministic storage id of one subject's balance in one currency.
// It is derived from the subject alone: the datastore is already the tenant, so
// naming the org here would be naming it twice.
func id(subjectKind, subject string, cur currency.Type) string {
	sum := sha256.Sum256([]byte(subjectKind + "\x00" + subject + "\x00" + strings.ToLower(string(cur))))
	return "rsv_" + hex.EncodeToString(sum[:16])
}

// Held is how much of the subject's money is withheld right now — one keyed
// read, no scan. An unknown subject holds nothing.
func Held(db *datastore.Datastore, subjectKind, subject string, cur currency.Type) currency.Cents {
	b, err := get(db, subjectKind, subject, cur)
	if err != nil {
		return 0
	}
	return currency.Cents(b.Held)
}

// get reads the balance row, or an initialized zero one when there is none.
// Read by the EXACT storage key rather than through GetById: a deterministic
// non-hashid id decodes to a kind-less key there, which the Postgres backend
// never finds — the failure mode that makes a guard look brand new on every
// retry (see models/idempotencykey).
func get(db *datastore.Datastore, subjectKind, subject string, cur currency.Type) (*Balance, error) {
	b := NewBalance(db)
	key := db.NewKey(b.Kind(), id(subjectKind, subject, cur), 0, nil)
	if err := b.Get(key); err != nil {
		if errors.Is(err, datastore.ErrNoSuchEntity) {
			b = NewBalance(db)
			b.SubjectKind = subjectKind
			b.Subject = subject
			b.Currency = cur
			return b, nil
		}
		return nil, err
	}
	return b, nil
}

// Post moves the ledger by amount (positive withholds, negative releases) and
// returns the entry it wrote and the balance after it.
//
// A zero amount writes NOTHING and is not an error: a screen that withheld
// nothing has nothing to record, and a ledger padded with zero rows is a ledger
// nobody reads. A release larger than the balance is refused — the ledger
// cannot invent money it never held.
//
// The balance is written BEFORE the entry. A crash between the two leaves a
// balance with no evidence row, which over-states what is held and is the safe
// direction to be wrong in; the other order would release money whose evidence
// says it is still held.
func Post(db *datastore.Datastore, e *Entry) (*Entry, *Balance, error) {
	if e.Amount == 0 {
		b, err := get(db, e.SubjectKind, e.Subject, e.Currency)
		return nil, b, err
	}

	b, err := get(db, e.SubjectKind, e.Subject, e.Currency)
	if err != nil {
		return nil, nil, err
	}
	held := b.Held + e.Amount
	if held < 0 {
		return nil, nil, ErrShort
	}

	b.SetId(id(e.SubjectKind, e.Subject, e.Currency))
	b.SubjectKind = e.SubjectKind
	b.Subject = e.Subject
	b.Currency = e.Currency
	b.Held = held
	b.Entries++
	b.At = time.Now()
	if err := b.Put(); err != nil {
		return nil, nil, err
	}

	e.Held = held
	if err := e.Create(); err != nil {
		return nil, b, err
	}
	return e, b, nil
}

// Balances reads the org's reserve balances, largest held first, at most
// bound(limit) of them. Ordering by the amount withheld is what a reader wants:
// the balances that matter are the big ones.
func Balances(db *datastore.Datastore, subjectKind, subject string, limit int) []*Balance {
	q := BalanceQuery(db)
	if subjectKind != "" {
		q = q.Filter("SubjectKind=", subjectKind)
	}
	if subject != "" {
		q = q.Filter("Subject=", subject)
	}

	n := bound(limit)
	iter := q.Order("-Held").Limit(n).Run()

	out := []*Balance{}
	for len(out) < n {
		b := NewBalance(db)
		if _, err := iter.Next(b); err != nil {
			break
		}
		out = append(out, b)
	}
	return out
}

// Entries reads the ledger newest first, optionally narrowed to one subject, at
// most bound(limit) rows.
func Entries(db *datastore.Datastore, subjectKind, subject string, limit int) []*Entry {
	root := db.NewKey("synckey", "", 1, nil)
	q := EntryQuery(db).Ancestor(root)
	if subjectKind != "" {
		q = q.Filter("SubjectKind=", subjectKind)
	}
	if subject != "" {
		q = q.Filter("Subject=", subject)
	}

	n := bound(limit)
	iter := q.Order("-CreatedAt").Limit(n).Run()

	out := []*Entry{}
	for len(out) < n {
		e := NewEntry(db)
		if _, err := iter.Next(e); err != nil {
			break
		}
		out = append(out, e)
	}
	return out
}
