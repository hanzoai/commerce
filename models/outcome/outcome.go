// Copyright © 2026 Hanzo AI. MIT License.

// Package outcome is how a scored money event actually turned out.
//
// A screen is what we judged; an outcome is what the world judged. The pair is
// the only thing a model can learn from, which is why an outcome is written
// DURABLY to the org's own books first and forwarded to the scoring plane
// second. Wired the other way a bus hiccup loses the label and the loss is
// invisible.
package outcome

import (
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/orm"
)

func init() { orm.Register[Outcome]("risk-outcome") }

// Every model MUST satisfy mixin.Entity — see the note on control.Control.
var _ mixin.Entity = (*Outcome)(nil)

// Event values — the post-purchase facts the money plane can observe about
// itself. Each is something commerce KNOWS, not something it infers.
const (
	Dispute    = "dispute"    // a chargeback was opened
	Won        = "won"        // the dispute was defended
	Lost       = "lost"       // the dispute was conceded or lost
	Refund     = "refund"     // the merchant returned the money
	PayoutFail = "payoutfail" // money out failed at the destination
	Negative   = "negative"   // the account balance went below zero
	Abuse      = "abuse"      // metered consumption was judged abusive
)

var events = map[string]bool{
	Dispute: true, Won: true, Lost: true,
	Refund: true, PayoutFail: true, Negative: true, Abuse: true,
}

// Events reports whether e is an outcome this plane records. An unknown outcome
// is refused rather than stored under a label nothing will ever read.
func Events(e string) bool { return events[e] }

// Outcome is one observed fact about a money event.
type Outcome struct {
	mixin.Model[Outcome]

	// Event is what happened: dispute, won, lost, refund, payoutfail, negative
	// or abuse. It is NOT named Kind — a model field named Kind shadows the ORM's
	// Kind() method and the entity stops satisfying mixin.Entity, which surfaces
	// as a nil dereference on the first read rather than a build error.
	Event       string `json:"event"`
	SubjectKind string `json:"subjectKind"`
	Subject     string `json:"subject"`

	Amount   int64         `json:"amount,omitempty"`
	Currency currency.Type `json:"currency,omitempty"`

	// Screen and Decision anchor the outcome to the judgement it corrects. Both
	// may be empty: a dispute can land on a charge that was never screened, and
	// that is exactly the case a model most needs to see.
	Screen   string `json:"screen,omitempty"`
	Decision string `json:"decision,omitempty"`

	Reference string `json:"reference,omitempty"`
	Note      string `json:"note,omitempty"`
	Idem      string `json:"idem,omitempty"`

	// Reported records whether the scoring plane accepted the label, and Refusal
	// why it did not. An unreported outcome is still a durable record here, so a
	// scoring outage costs learning latency and never evidence.
	Reported   bool      `json:"reported"`
	ReportedAt time.Time `json:"reportedAt,omitempty"`
	Refusal    string    `json:"refusal,omitempty"`

	// By is who reported it, from the validated principal, never off the wire.
	By string `json:"by,omitempty"`
}

func (o *Outcome) Defaults() {
	o.Parent = o.Datastore().NewKey("synckey", "", 1, nil)
}

func New(db *datastore.Datastore) *Outcome {
	o := new(Outcome)
	o.Init(db)
	o.Defaults()
	return o
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("risk-outcome")
}

// ByIdem returns the outcome already written under key, if any.
func ByIdem(db *datastore.Datastore, key string) (*Outcome, bool) {
	if key == "" {
		return nil, false
	}
	root := db.NewKey("synckey", "", 1, nil)
	iter := Query(db).Ancestor(root).Filter("Idem=", key).Run()
	o := New(db)
	if _, err := iter.Next(o); err != nil {
		return nil, false
	}
	return o, true
}

// For reads outcomes for one subject, and with an empty subject every outcome
// in the org. The datastore is already namespaced to one tenant.
func For(db *datastore.Datastore, subjectKind, subject string) []*Outcome {
	root := db.NewKey("synckey", "", 1, nil)
	q := Query(db).Ancestor(root)
	if subjectKind != "" {
		q = q.Filter("SubjectKind=", subjectKind)
	}
	if subject != "" {
		q = q.Filter("Subject=", subject)
	}

	out := []*Outcome{}
	iter := q.Run()
	for {
		o := New(db)
		if _, err := iter.Next(o); err != nil {
			break
		}
		out = append(out, o)
	}
	return out
}
