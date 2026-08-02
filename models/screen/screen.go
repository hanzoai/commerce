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

	// Reference is the money object this judged — a payment intent, a payout, a
	// dispute — so the record joins back to the books.
	Reference string `json:"reference,omitempty"`
	Processor string `json:"processor,omitempty"`
	Reason    string `json:"reason,omitempty"`

	// Idem is the caller's idempotency key. A repeat under the same key returns
	// this row instead of screening — and, more importantly, instead of moving
	// money twice.
	Idem string `json:"idem,omitempty"`

	// Fingerprint is the exact request this key answered: the stage, the
	// subject, the amount, the currency, the direction and the money object.
	//
	// A key alone is not an idempotent request. Without the fingerprint, reusing
	// one key across two different moves returns the FIRST move's answer for the
	// second — and since the answer carries Allowed, which the payout boundary
	// pays out, that is a key that decides how much money leaves. A repeat whose
	// fingerprint differs is a caller's mistake and is refused as one.
	Fingerprint string `json:"-"`

	// Reasserts counts the times a repeat under this key was re-judged against
	// the controls in force at that moment and came back STRICTER, and
	// ReassertedAt is when it last happened.
	//
	// A recorded answer is evidence of a judgement; it is not a licence to
	// re-run the judgement's outcome later, when the org may have blocked the
	// subject in between. So a repeat re-asserts the live controls and this row
	// records that it did — with the answer it first gave preserved in Detail,
	// because a record that quietly rewrote itself is not evidence.
	Reasserts    int       `json:"reasserts,omitempty"`
	ReassertedAt time.Time `json:"reassertedAt,omitempty"`

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

// Query is every read on this kind, and it filters by NO ANCESTOR — the note
// the whole risk plane's reads are shaped around.
//
// Rows are created inside a synckey group, but a row read by id and written
// back comes out of that group, and in this ORM reading by id is the ONLY way
// to write at all (see [Writable]). So an ancestor-filtered read stops seeing a
// row the moment anything updates it: a control would vanish from the list the
// first time its running total moved, and a judgement would vanish the first
// time it was re-asserted. Rows silently disappearing from a money plane's
// evidence is a worse failure than any grouping buys back.
//
// The TENANT boundary is the datastore's namespace, which every query here
// inherits and none can widen. The ancestor was never the boundary.
func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("risk-screen")
}

// Max is the most screens any read of this kind materialises, and the bound a
// caller gets when it names none.
//
// THERE IS NO UNBOUNDED READ HERE AND NO WAY TO ASK FOR ONE. A screen is
// written on every judged move, so this is the fastest-growing table on the
// money plane: one request that materialises a busy merchant's whole history is
// enough to exhaust the process — and the process is shared, so the org that
// dies of it is not the org that asked. A limit of zero used to mean "all";
// it now means "the bound", and the difference is a whole class of outage.
const Max = 200

// bound clamps a caller's limit into 1..Max.
func bound(limit int) int {
	if limit <= 0 || limit > Max {
		return Max
	}
	return limit
}

// ByIdem returns the screen already written under key, if any. The datastore it
// is handed is namespaced to one org, so a key is unique within a tenant and
// two tenants using the same key never collide.
//
// It reads the OLDEST match, not an arbitrary one. Two first-ever screens under
// one key can race (this backend has no compare-and-swap reachable from the
// model layer — see models/idempotencykey), and if that happens every later
// repeat must still converge on the same answer rather than alternating between
// two. The money boundary closes the race itself: POST /v1/billing/payouts
// takes an idempotencykey guard, whose storage id IS deterministic, before it
// screens at all.
func ByIdem(db *datastore.Datastore, key string) (*Screen, bool) {
	if key == "" {
		return nil, false
	}
	iter := Query(db).Filter("Idem=", key).Order("CreatedAt").Limit(1).Run()
	s := New(db)
	if _, err := iter.Next(s); err != nil {
		return nil, false
	}
	return s, true
}

// Writable re-reads the screen named by id so it can be CHANGED.
//
// A row handed back by a query iterator — which is what [ByIdem], [For] and
// [ByReference] return — CANNOT BE WRITTEN THROUGH IN THIS ORM: Update and Put
// on one land nowhere and RETURN NO ERROR. A judgement re-asserted on a query
// row would look re-asserted in the response and be unchanged in the store,
// which on this plane means the tightening survives exactly as long as the
// process does. Every write that starts from a query goes through here.
func Writable(db *datastore.Datastore, id string) (*Screen, error) {
	s := New(db)
	if err := s.GetById(id); err != nil {
		return nil, err
	}
	return s, nil
}

// For reads screens, newest first, optionally narrowed to one subject, up to
// limit — and never more than [Max] however large a limit is asked for. The
// bound is applied by the STORE, not by breaking out of the loop: a query that
// selects a million rows has already cost the million rows.
func For(db *datastore.Datastore, subjectKind, subject string, limit int) []*Screen {
	q := Query(db)
	if subjectKind != "" {
		q = q.Filter("SubjectKind=", subjectKind)
	}
	if subject != "" {
		q = q.Filter("Subject=", subject)
	}
	return page(db, q, limit)
}

// ByReference reads the screens that judged one money object — a payment
// intent, a payout — newest first, up to limit and never more than [Max].
//
// It exists because assembling a dispute defence needs the judgement that
// admitted ONE charge, and the way to get that is to ask the store for it. The
// alternative this replaces was to read every screen the org ever wrote and
// compare references in Go, which is the same answer at the cost of the whole
// table.
func ByReference(db *datastore.Datastore, reference string, limit int) []*Screen {
	if reference == "" {
		return []*Screen{}
	}
	return page(db, Query(db).Filter("Reference=", reference), limit)
}

// page runs a bounded query, newest first. It is the ONE read shape in this
// package, so no caller can accidentally introduce an unbounded one.
func page(db *datastore.Datastore, q datastore.Query, limit int) []*Screen {
	out := []*Screen{}
	iter := q.Order("-CreatedAt").Limit(bound(limit)).Run()
	for {
		s := New(db)
		if _, err := iter.Next(s); err != nil {
			break
		}
		out = append(out, s)
	}
	return out
}
