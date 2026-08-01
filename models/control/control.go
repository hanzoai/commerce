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

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("risk-control")
}

// Live reads every control in force for a subject at now, newest first. The
// datastore it is handed is ALREADY namespaced to one org — that is the tenant
// boundary, and this function neither takes an org nor could widen one.
func LiveFor(db *datastore.Datastore, subjectKind, subject string, now time.Time) ([]*Control, error) {
	root := db.NewKey("synckey", "", 1, nil)
	iter := Query(db).Ancestor(root).
		Filter("SubjectKind=", subjectKind).
		Filter("Subject=", subject).
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

// All reads every control in the org, live or not, newest first.
func All(db *datastore.Datastore) []*Control {
	root := db.NewKey("synckey", "", 1, nil)
	iter := Query(db).Ancestor(root).Run()

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
