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

// ByIdem returns the screen already written under key, if any. The datastore it
// is handed is namespaced to one org, so a key is unique within a tenant and
// two tenants using the same key never collide.
func ByIdem(db *datastore.Datastore, key string) (*Screen, bool) {
	if key == "" {
		return nil, false
	}
	root := db.NewKey("synckey", "", 1, nil)
	iter := Query(db).Ancestor(root).Filter("Idem=", key).Run()
	s := New(db)
	if _, err := iter.Next(s); err != nil {
		return nil, false
	}
	return s, true
}

// For reads screens, newest first, optionally narrowed to one subject. limit 0
// means the caller stated no bound and gets the page default.
func For(db *datastore.Datastore, subjectKind, subject string, limit int) []*Screen {
	root := db.NewKey("synckey", "", 1, nil)
	q := Query(db).Ancestor(root)
	if subjectKind != "" {
		q = q.Filter("SubjectKind=", subjectKind)
	}
	if subject != "" {
		q = q.Filter("Subject=", subject)
	}

	out := []*Screen{}
	iter := q.Run()
	for {
		s := New(db)
		if _, err := iter.Next(s); err != nil {
			break
		}
		out = append(out, s)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}
