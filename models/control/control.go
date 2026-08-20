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

	// Cap is the CEILING, in exact minor units, on everything this reserve may
	// ever withhold from the subject — the total, not the per-move share. It is
	// REQUIRED on a reserve and refused on the other effects.
	//
	// A rate with no ceiling is not a reserve, it is a standing seizure: the
	// share applies to every outbound move forever, so the withheld total is
	// bounded only by how much money the merchant tries to take out. The
	// ceiling is what makes the reserve a stated, finite, disclosable amount,
	// and it is enforced against the reserve LEDGER (models/reserve) so the
	// accounting is cumulative rather than per-move.
	Cap int64 `json:"cap,omitempty"`

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

// Page is how many controls one read answers with, and the most it can ever
// answer with — the same bound, for the same reason, as [screen.Page].
//
// A subject's LIVE set is naturally small (Place is idempotent per effect and
// rate, so a monitor running every cycle does not accumulate duplicates), which
// is why reading a page of it is not a truncation in practice. The bound is
// here for the case where it is not: a bound that only holds when the data is
// well-behaved is not a bound.
const Page = 200

func bound(limit int) int {
	if limit <= 0 || limit > Page {
		return Page
	}
	return limit
}

// LiveFor reads the controls in force for a subject at now, newest first. The
// datastore it is handed is ALREADY namespaced to one org — that is the tenant
// boundary, and this function neither takes an org nor could widen one.
func LiveFor(db *datastore.Datastore, subjectKind, subject string, now time.Time) ([]*Control, error) {
	out := []*Control{}
	for _, c := range For(db, subjectKind, subject, 0) {
		if c.Live(now) {
			out = append(out, c)
		}
	}
	return out, nil
}

// For reads a subject's controls, live or not, newest first, at most
// bound(limit) of them.
func For(db *datastore.Datastore, subjectKind, subject string, limit int) []*Control {
	root := db.NewKey("synckey", "", 1, nil)
	q := Query(db).Ancestor(root)
	if subjectKind != "" {
		q = q.Filter("SubjectKind=", subjectKind)
	}
	if subject != "" {
		q = q.Filter("Subject=", subject)
	}
	return collect(q, db, limit)
}

// All reads the org's controls, live or not, newest first, at most
// bound(limit) of them.
func All(db *datastore.Datastore, limit int) []*Control {
	return For(db, "", "", limit)
}

func collect(q datastore.Query, db *datastore.Datastore, limit int) []*Control {
	n := bound(limit)
	iter := q.Order("-CreatedAt").Limit(n).Run()

	out := []*Control{}
	for len(out) < n {
		c := New(db)
		if _, err := iter.Next(c); err != nil {
			break
		}
		out = append(out, c)
	}
	return out
}
