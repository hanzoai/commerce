// Copyright © 2026 Hanzo AI. MIT License.

// Package screen is the record of one risk evaluation on the money plane.
//
// A screen is written BEFORE the answer is acted on and kept whether the answer
// was a judgement or a refusal, because a scoring plane that could not answer
// must not look like a plane that answered "clean". It is also the countable
// unit the tiers price — one screen per transaction, account or customer — so
// the record and the meter are the same row and cannot disagree.
package screen

import (
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/orm"

	. "github.com/hanzoai/commerce/types"
)

func init() { orm.Register[Screen]("risk-screen") }

// Every model MUST satisfy mixin.Entity — see the note on control.Control.
var _ mixin.Entity = (*Screen)(nil)

// Screen is one evaluation: what was asked, what came back, and what the money
// plane did about it.
type Screen struct {
	mixin.Model[Screen]

	Stage       string `json:"stage"`
	SubjectKind string `json:"subjectKind"`
	Subject     string `json:"subject"`

	// Amount is exact minor units of Currency. Out records the direction, so a
	// $10 charge and a $10 payout are never the same row.
	Amount   int64         `json:"amount"`
	Currency currency.Type `json:"currency,omitempty"`
	Out      bool          `json:"out,omitempty"`

	// Action is what the money plane did, AFTER the controls were applied — not
	// merely what the scoring plane suggested. Decision names the /v1/risk
	// decision this record is anchored to, so a dispute can be defended with the
	// exact judgement that admitted the charge.
	Action   string  `json:"action"`
	Score    float64 `json:"score,omitempty"`
	Agency   string  `json:"agency,omitempty"`
	Decision string  `json:"decision,omitempty"`

	// Refusal states why the scoring plane could not judge. A screen carrying a
	// refusal was decided by the controls alone.
	Refusal string `json:"refusal,omitempty"`
	Shadow  bool   `json:"shadow,omitempty"`

	// Held is the exact minor units a reserve withheld from this move, and
	// Allowed what remained. Both are zero on an inbound move.
	Held    int64 `json:"held,omitempty"`
	Allowed int64 `json:"allowed,omitempty"`

	// Posted records that Held has been written to the reserve ledger. It is
	// what makes the ledger idempotent per move: a retried payout replays THIS
	// row, so the second attempt posts nothing and the merchant's money is
	// withheld once.
	Posted bool `json:"posted,omitempty"`

	// Reference is the money object this judged — a payment intent, a payout, a
	// dispute — so the record joins back to the books.
	Reference string `json:"reference,omitempty"`
	Processor string `json:"processor,omitempty"`
	Reason    string `json:"reason,omitempty"`

	// Idem is the caller's idempotency key. A repeat under the same key returns
	// this row instead of screening — and, more importantly, instead of moving
	// money twice.
	Idem string `json:"idem,omitempty"`

	// Digest fingerprints the move this row answered: the stage, the subject,
	// the exact amount and direction, and the facts sent. A key is the name of
	// ONE question, so a repeat under the same key that asks a DIFFERENT
	// question is refused rather than answered with the cheap first answer — the
	// swap that would otherwise let a caller screen one cent and spend the
	// verdict on ten thousand dollars.
	Digest string `json:"digest,omitempty"`

	// Reasserted counts the times the controls in force were re-applied to this
	// answer after it was first given. A replay never returns a verdict more
	// permissive than the org's standing controls, so a block placed between two
	// attempts tightens the row rather than being bypassed by it.
	Reasserted int64 `json:"reasserted,omitempty"`

	// Detail carries the evidence a decision has to survive on: the signals sent,
	// the rules that hit, and the ids of the controls that bore on the move.
	Detail  Map    `json:"detail,omitempty" datastore:"-"`
	Detail_ string `json:"-" datastore:",noindex"`

	// Outcome is how the world judged it later — set by the outcome feed, empty
	// until then. It is the label the org's own model learns from.
	Outcome   string    `json:"outcome,omitempty"`
	OutcomeAt time.Time `json:"outcomeAt,omitempty"`
}

func (s *Screen) Defaults() {
	s.Parent = s.Datastore().NewKey("synckey", "", 1, nil)
}

func (s *Screen) Load(ps []datastore.Property) (err error) {
	if err = datastore.LoadStruct(s, ps); err != nil {
		return err
	}
	if len(s.Detail_) > 0 {
		err = json.DecodeBytes([]byte(s.Detail_), &s.Detail)
	}
	return err
}

func (s *Screen) Save() (ps []datastore.Property, err error) {
	s.Detail_ = string(json.EncodeBytes(&s.Detail))
	return datastore.SaveStruct(s)
}

func New(db *datastore.Datastore) *Screen {
	s := new(Screen)
	s.Init(db)
	s.Defaults()
	return s
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("risk-screen")
}

// Page is how many rows one read answers with, and the most it can ever
// answer with.
//
// THERE IS NO UNBOUNDED READ. A limit of zero is not "every row" — it is this
// page. A single request that materialises a busy merchant's whole history is
// how one tenant takes the store down for every tenant sharing the process, and
// a bound that a caller can opt out of by passing 0 is not a bound. Reads that
// want more take another page.
const Page = 200

// bound clamps a caller's limit into 1..Page. It is the ONE place the page size
// is decided, so no read can grow its own.
func bound(limit int) int {
	if limit <= 0 || limit > Page {
		return Page
	}
	return limit
}

// ByIdem returns the screen already written under key, if any. The datastore it
// is handed is namespaced to one org, so a key is unique within a tenant and
// two tenants using the same key never collide.
func ByIdem(db *datastore.Datastore, key string) (*Screen, bool) {
	if key == "" {
		return nil, false
	}
	root := db.NewKey("synckey", "", 1, nil)
	iter := Query(db).Ancestor(root).Filter("Idem=", key).Limit(1).Run()
	s := New(db)
	if _, err := iter.Next(s); err != nil {
		return nil, false
	}
	return s, true
}

// For reads screens NEWEST FIRST, optionally narrowed to one subject, at most
// [bound](limit) of them. The ordering is what makes the bound honest: the most
// recent page of a merchant's judgements is a window on its behaviour now,
// where the oldest page is a window on the day it signed up.
func For(db *datastore.Datastore, subjectKind, subject string, limit int) []*Screen {
	return collect(narrow(db, subjectKind, subject), db, limit)
}

// ByReference reads the screens that judged one money object — a payment
// intent, a payout, an order. It asks the store for that reference instead of
// walking every row looking for it: a dispute on a busy merchant must not read
// the merchant's whole history to find the one judgement that admitted the
// charge.
func ByReference(db *datastore.Datastore, reference string, limit int) []*Screen {
	if reference == "" {
		return []*Screen{}
	}
	root := db.NewKey("synckey", "", 1, nil)
	return collect(Query(db).Ancestor(root).Filter("Reference=", reference), db, limit)
}

func narrow(db *datastore.Datastore, subjectKind, subject string) datastore.Query {
	root := db.NewKey("synckey", "", 1, nil)
	q := Query(db).Ancestor(root)
	if subjectKind != "" {
		q = q.Filter("SubjectKind=", subjectKind)
	}
	if subject != "" {
		q = q.Filter("Subject=", subject)
	}
	return q
}

// collect runs a bounded, newest-first read. The bound is pushed into the
// QUERY, not applied after the rows arrive: a limit enforced in Go has already
// paid for every row it throws away.
func collect(q datastore.Query, db *datastore.Datastore, limit int) []*Screen {
	n := bound(limit)
	iter := q.Order("-CreatedAt").Limit(n).Run()

	out := []*Screen{}
	for len(out) < n {
		s := New(db)
		if _, err := iter.Next(s); err != nil {
			break
		}
		out = append(out, s)
	}
	return out
}
