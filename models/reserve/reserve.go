// Copyright © 2026 Hanzo AI. MIT License.

// Package reserve is the LEDGER of money a reserve actually withheld.
//
// A reserve rate says what share of a move to hold back. It does not say how
// much is being held, when it was taken, which judgement took it, or what a
// release returns — and a haircut nobody accounts for is not a reserve, it is
// an arbitrary shortfall on a merchant's payout. This package is the account:
// one durable, append-only entry per movement of reserved money, in the org's
// own store.
//
// Two things are kept apart on purpose. The RUNNING TOTAL a ceiling is measured
// against lives on the control that declared it (control.Control.Held), because
// that is the one number the money path must read to decide the next move and
// it must be one read, not a sum over a table. The ENTRIES here are the
// evidence: what was taken, from which judgement, under which control. The
// total is what enforcement needs; the entries are what a merchant reconciles
// and an appeal cites.
//
// Entries are IDENTIFIED BY WHAT THEY RECORD, never minted fresh: a hold is
// named by the screen that took it and a release by the control that returned
// it, so a retried request writes the same row rather than a second one. A
// ledger that double-posts under retry is worse than no ledger.
package reserve

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/orm"
)

func init() { orm.Register[Entry]("risk-reserve", orm.WithStringKey[Entry]()) }

// Every model MUST satisfy mixin.Entity — see the note on control.Control.
var _ mixin.Entity = (*Entry)(nil)

// Max is the most entries any read of this kind materialises. The ledger grows
// with a merchant's payouts, so it is exactly the table a single unbounded read
// would eventually die on.
const Max = 500

// Entry is one movement of reserved money. Exactly one of Held and Released is
// non-zero: an entry records a hold or a return, never a net, because a net
// cannot be audited back to the move that caused it.
type Entry struct {
	mixin.Model[Entry]

	SubjectKind string        `json:"subjectKind"`
	Subject     string        `json:"subject"`
	Currency    currency.Type `json:"currency"`

	// Held and Released are exact minor units of Currency.
	Held     int64 `json:"held,omitempty"`
	Released int64 `json:"released,omitempty"`

	// Control is the reserve that took or returned it, and Screen the judgement
	// that decided the move. Together they join a shortfall on a payout back to
	// the declaration that caused it.
	Control string `json:"control"`
	Screen  string `json:"screen,omitempty"`

	// Reference is the money object the hold came out of — the payout, the
	// transfer — so a merchant reconciling a short payment finds this row by the
	// id on its own statement.
	Reference string `json:"reference,omitempty"`
}

func (e *Entry) Load(ps []datastore.Property) error { return datastore.LoadStruct(e, ps) }

func (e *Entry) Save() ([]datastore.Property, error) { return datastore.SaveStruct(e) }

// New returns an entry bound to db. It sets no ancestor: an entry is named by
// what it records (a deterministic string id), and a string-keyed row is stored
// under that id alone — the tenant boundary is db's namespace, which every read
// and write here inherits and none can widen.
func New(db *datastore.Datastore) *Entry {
	e := new(Entry)
	e.Init(db)
	return e
}

// Query is every read on this kind. Like screen.Query it filters by NO
// ANCESTOR — an ancestor-filtered read loses a row the moment anything updates
// it, and the tenant boundary is the datastore's namespace either way. The full
// reasoning lives on screen.Query; it is one property of one ORM and it is
// written down once.
func Query(db *datastore.Datastore) datastore.Query { return db.Query("risk-reserve") }

// ErrAmount refuses an entry that records no money. A zero movement is not a
// ledger row, it is noise in the account a merchant has to read.
var ErrAmount = errors.New("reserve: an entry records a positive amount")

// id derives an entry's STORAGE id from what it records, so the same movement
// posted twice lands on one row through the backend's ON CONFLICT upsert rather
// than doubling the ledger. Named by cause: a hold by the screen that took it, a
// release by the control that returned it.
func id(prefix, cause string) string {
	sum := sha256.Sum256([]byte(prefix + "\x00" + cause))
	return prefix + "_" + hex.EncodeToString(sum[:16])
}

// Hold records money withheld from one move by one reserve.
//
// It is idempotent on the SCREEN: a retried move re-posts the same row instead
// of holding twice, which matters because the screen itself is idempotent and
// the two must agree about how many times one move happened.
func Hold(db *datastore.Datastore, subjectKind, subject string, cur currency.Type, amount int64, control, screen, reference string) (*Entry, error) {
	if amount <= 0 {
		return nil, ErrAmount
	}
	e := New(db)
	e.SetId(id("hold", screen))
	e.SubjectKind = subjectKind
	e.Subject = subject
	e.Currency = cur
	e.Held = amount
	e.Control = control
	e.Screen = screen
	e.Reference = reference
	if err := e.Put(); err != nil {
		return nil, err
	}
	return e, nil
}

// Release records reserved money returned when a control is lifted.
//
// It is idempotent on the CONTROL: a control releases once, and a retried
// release re-posts the same row rather than returning the money twice on paper.
func Release(db *datastore.Datastore, subjectKind, subject string, cur currency.Type, amount int64, control string) (*Entry, error) {
	if amount <= 0 {
		return nil, ErrAmount
	}
	e := New(db)
	e.SetId(id("release", control))
	e.SubjectKind = subjectKind
	e.Subject = subject
	e.Currency = cur
	e.Released = amount
	e.Control = control
	if err := e.Put(); err != nil {
		return nil, err
	}
	return e, nil
}

// For reads a subject's reserve ledger, newest first, up to limit — and never
// more than [Max]. The datastore is already namespaced to one org, which is the
// tenant boundary.
func For(db *datastore.Datastore, subjectKind, subject string, limit int) []*Entry {
	if limit <= 0 || limit > Max {
		limit = Max
	}
	q := Query(db)
	if subjectKind != "" {
		q = q.Filter("SubjectKind=", subjectKind)
	}
	if subject != "" {
		q = q.Filter("Subject=", subject)
	}

	out := []*Entry{}
	iter := q.Order("-CreatedAt").Limit(limit).Run()
	for {
		e := New(db)
		if _, err := iter.Next(e); err != nil {
			break
		}
		out = append(out, e)
	}
	return out
}
