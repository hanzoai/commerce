// Copyright © 2026 Hanzo AI. MIT License.

// Package control is a standing restraint the money plane enforces on one
// subject inside one org.
//
// A control is a DECLARATION that money must move differently — it is not a
// judgement and it is not a balance. Hanzo Risk declares; commerce enforces at
// the money boundary. It lives HERE, in the money plane's own store, for one
// reason: enforcement must not depend on a network hop. A scoring outage may
// cost a score; it may never lift a reserve.
//
// NOTHING HERE IS A RUNNING TOTAL. A reserve's ceiling is measured against what
// the reserve is actually holding, and that number lives in models/reserve,
// behind the one door that moves money. It was a field on this row once, and a
// balance on a declaration is a balance every op that can touch the declaration
// can move — which made the ceiling a consumable a merchant could spend with a
// money-free screen, disarming its own reserve. A declaration declares.
//
// Three kinds, orthogonal by what they stop:
//
//	reserve  a share of every outbound move is withheld  (partial, out)
//	hold     no money leaves                             (total, out)
//	block    no money moves, in or out                   (total, both)
package control

import (
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/orm"
)

func init() { orm.Register[Control]("risk-control") }

// Every model MUST satisfy mixin.Entity. A struct field named like an embedded
// Model[T] method (Kind/Key/Id/Save/…) silently shadows it and breaks the
// interface — this assertion turns that into a build error instead of a runtime
// nil dereference on the first read.
var _ mixin.Entity = (*Control)(nil)

// Effect values — what the control does to money.
const (
	Reserve = "reserve"
	Hold    = "hold"
	Block   = "block"
)

// Effects reports whether e is a control this plane enforces. An unknown effect
// is refused at the boundary rather than stored and silently never applied.
func Effects(e string) bool { return e == Reserve || e == Hold || e == Block }

// FullRate is one hundred percent in basis points. A reserve rate is basis
// points, not a fraction: money is integer arithmetic end to end, and a float
// rate would drift the withheld amount by a cent per move at scale.
const FullRate = 10000

// Control is one standing restraint.
type Control struct {
	mixin.Model[Control]

	// Effect is what this control does: reserve, hold or block. It is NOT named
	// Kind: a model field named Kind shadows the ORM's Kind() method, and the
	// entity stops satisfying mixin.Entity — a nil dereference at the first read
	// rather than a build error. The compile-time guard below turns any future
	// recurrence into one.
	Effect      string `json:"effect"`
	SubjectKind string `json:"subjectKind"`
	Subject     string `json:"subject"`

	// Rate is basis points withheld from each outbound move, for Effect=reserve
	// only. 0 on the other effects, which withhold everything by stopping the move.
	Rate int64 `json:"rate,omitempty"`

	// Cap is the CEILING on what this reserve may withhold in total: exact minor
	// units of Currency, cumulative across every move it touches. Zero means the
	// reserve declares no ceiling.
	//
	// A rate without a ceiling bounds nothing. It is a share of a number the
	// CALLER chooses, so "25% of every payout" withholds a quarter of whatever is
	// asked for, forever, with no total it converges on and no account that says
	// how much is being held. A ceiling is what makes a reserve a quantity a
	// merchant can reconcile and a platform can release.
	Cap int64 `json:"cap,omitempty"`

	// Currency scopes a reserve to the money it is denominated in. A ceiling is
	// an amount, and an amount without a currency is a number: 10000 of held EUR
	// does not satisfy a cap declared in USD. Empty means the reserve bears on
	// every currency, which is only allowed when it declares no ceiling.
	Currency currency.Type `json:"currency,omitempty"`

	// Until is when the control lapses. Zero means it stands until released,
	// which is what a fraud restraint should do — an expiry a caller forgot to
	// set must not silently open the gate.
	Until time.Time `json:"until,omitempty"`

	Reason string `json:"reason,omitempty"`

	// By is who placed it, taken from the validated principal and never from the
	// request body. A control the caller could attribute to someone else is not
	// an audit record.
	By string `json:"by,omitempty"`

	Released   bool      `json:"released,omitempty"`
	ReleasedAt time.Time `json:"releasedAt,omitempty"`
	ReleasedBy string    `json:"releasedBy,omitempty"`
}

func (c *Control) Defaults() {
	c.Parent = c.Datastore().NewKey("synckey", "", 1, nil)
}

// Ref is the control's recorded id, WITHOUT minting one.
//
// Id() allocates a key when there is none, which reaches for a datastore — so a
// control that was never stored panics there. The restraint algebra is a pure
// function over values and must be able to name a control without touching a
// store, so it asks for the ref and gets an empty string when there is nothing
// to name.
func (c *Control) Ref() string { return c.Id_ }

// Live reports whether the control bears on a move happening at now. A released
// or lapsed control is kept — it is the evidence that a restraint was once in
// force — but it restrains nothing.
func (c *Control) Live(now time.Time) bool {
	if c.Released {
		return false
	}
	return c.Until.IsZero() || c.Until.After(now)
}

// Bears reports whether this control bears on a move denominated in cur. Only a
// reserve narrows by currency — a hold and a block stop money whatever it is
// denominated in, because they are about the subject and not about an amount.
func (c *Control) Bears(cur currency.Type) bool {
	if c.Effect != Reserve {
		return true
	}
	return c.Currency == "" || c.Currency == cur
}

// Bounded reports whether this reserve declares a ceiling.
func (c *Control) Bounded() bool { return c.Effect == Reserve && c.Cap > 0 }

// Lift releases the control named by id and STORES it, returning the stored
// row. Releasing one already released is a no-op that does not rewrite who
// lifted it first.
//
// It reads BY ID, and that is not defensive tidiness — A ROW HANDED BACK BY A
// QUERY ITERATOR CANNOT BE WRITTEN THROUGH IN THIS ORM. Update and Put on one
// land nowhere AND RETURN NO ERROR (see the iterator-write test in
// control_test.go). Every read on the money path ([LiveFor], [All]) is a query,
// so a release applied to what a query returned would silently never happen.
// Reading it back by id is what makes the write real.
func Lift(db *datastore.Datastore, id, by string, now time.Time) (*Control, error) {
	c := New(db)
	if err := c.GetById(id); err != nil {
		return nil, err
	}
	if c.Released {
		return c, nil
	}
	c.Release(by, now)
	if err := c.Update(); err != nil {
		return nil, err
	}
	return c, nil
}

// Release marks the control lifted by who. Releasing twice is a no-op, so a
// retried release is not an error and does not rewrite the first release's
// author or time.
func (c *Control) Release(by string, now time.Time) {
	if c.Released {
		return
	}
	c.Released = true
	c.ReleasedAt = now
	c.ReleasedBy = by
}

func New(db *datastore.Datastore) *Control {
	c := new(Control)
	c.Init(db)
	c.Defaults()
	return c
}

// Query is every read on this kind. Like screen.Query it filters by NO
// ANCESTOR — an ancestor-filtered read loses a row the moment anything updates
// it, and the tenant boundary is the datastore's namespace either way. The full
// reasoning lives on screen.Query; it is one property of one ORM and it is
// written down once.
func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("risk-control")
}

// Max is the most controls any read of this kind materialises.
//
// There is no unbounded read here, and no caller may ask for one. A control is
// a standing declaration a human places on a subject, so a tenant with more
// than this many is already pathological — but "already pathological" is
// exactly the state an attacker arranges, and a single request that
// materialises a whole table is how one org takes the process down for every
// org sharing it.
const Max = 500

// bound clamps a caller's limit into 1..Max. A limit of zero or less means the
// caller stated no bound, and gets the bound rather than none.
func bound(limit int) int {
	if limit <= 0 || limit > Max {
		return Max
	}
	return limit
}

// LiveFor reads the controls in force for a subject at now, newest first, up to
// [Max]. The datastore it is handed is ALREADY namespaced to one org — that is
// the tenant boundary, and this function neither takes an org nor could widen
// one.
func LiveFor(db *datastore.Datastore, subjectKind, subject string, now time.Time) ([]*Control, error) {
	iter := Query(db).
		Filter("SubjectKind=", subjectKind).
		Filter("Subject=", subject).
		Order("-CreatedAt").
		Limit(Max).
		Run()

	out := []*Control{}
	for {
		c := New(db)
		if _, err := iter.Next(c); err != nil {
			break
		}
		if c.Live(now) {
			out = append(out, c)
		}
	}
	return out, nil
}

// All reads the org's controls, live or not, newest first, up to limit — and
// never more than [Max] however large a limit is asked for.
func All(db *datastore.Datastore, limit int) []*Control {
	iter := Query(db).Order("-CreatedAt").Limit(bound(limit)).Run()

	out := []*Control{}
	for {
		c := New(db)
		if _, err := iter.Next(c); err != nil {
			break
		}
		out = append(out, c)
	}
	return out
}
