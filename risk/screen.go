// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"errors"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/screen"
	"github.com/hanzoai/commerce/models/types/currency"
)

// ErrRefused is what a money move gets when the money plane will not let it
// happen. It is a distinct error so a caller can render a clean refusal instead
// of a gateway failure — a refused move is a decision, not a fault.
var ErrRefused = errors.New("risk: the move is refused")

// Screener screens money moves for ONE org. The datastore it holds is already
// namespaced to that org, which IS the tenant boundary: a Screener cannot read
// or write another tenant's rows because it holds no way to name one.
type Screener struct {
	// DB is the org-namespaced datastore. Required.
	DB *datastore.Datastore
	// Plane is the scoring plane. Nil uses the process-wide one from [Of].
	Plane Client
	// Now is the clock, a seam for tests. Nil is time.Now.
	Now func() time.Time
	// By is the validated principal on whose behalf the screen runs. It is set
	// by the caller from the request identity and never from a request body.
	By string
}

// Move is one money movement put to the money plane for judgement.
type Move struct {
	Stage     Stage
	Subject   Subject
	Amount    currency.Cents
	Currency  currency.Type
	Out       bool
	Signals   map[string]string
	Reference string
	Processor string
	// Idem makes a repeat of the same move return the first screen instead of
	// screening — and, far more importantly, instead of the caller acting twice
	// on two different answers to one question.
	Idem string
}

func (s *Screener) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Screener) plane() Client {
	if s.Plane != nil {
		return s.Plane
	}
	return Of()
}

// Screen judges one move and RECORDS the judgement before returning it. The
// record is written first for the same reason a ledger entry is: an answer the
// money plane acted on and did not keep is an answer that cannot be defended in
// a dispute.
//
// Order is deliberate. The CONTROLS are read first and from the org's own
// store, so a scoring outage can never lift a reserve. A move the controls
// already stop is not sent for scoring at all — spending the authorization
// budget on advice that cannot change the outcome is exactly the latency an
// attacker hammering a blocked merchant wants to buy.
func (s *Screener) Screen(ctx context.Context, m Move) (*screen.Screen, error) {
	if err := m.Subject.Valid(); err != nil {
		return nil, err
	}
	if m.Amount < 0 {
		return nil, errors.New("risk: amount is negative")
	}
	now := s.now()

	if prior, ok := screen.ByIdem(s.DB, m.Idem); ok {
		return prior, nil
	}

	live, err := control.LiveFor(s.DB, m.Subject.Kind, m.Subject.ID, now)
	if err != nil {
		return nil, err
	}
	restraint := Restrain(live, m.Amount, m.Out, now)

	rec := screen.New(s.DB)
	rec.Stage = string(m.Stage)
	rec.SubjectKind = m.Subject.Kind
	rec.Subject = m.Subject.ID
	rec.Amount = int64(m.Amount)
	rec.Currency = m.Currency
	rec.Out = m.Out
	rec.Reference = m.Reference
	rec.Processor = m.Processor
	rec.Idem = m.Idem
	rec.Held = int64(restraint.Held)
	rec.Allowed = int64(restraint.Allowed)
	rec.Reason = restraint.Reason
	rec.Detail = map[string]any{}
	if len(restraint.Controls) > 0 {
		rec.Detail["controls"] = restraint.Controls
	}
	if len(m.Signals) > 0 {
		rec.Detail["signals"] = m.Signals
	}

	action := Allow
	if restraint.Blocked {
		action = Block
	} else {
		ask := &Ask{Stage: m.Stage, Subject: m.Subject, Signals: m.Signals, Idem: m.Idem}
		if m.Amount > 0 || m.Currency != "" {
			ask.Amount = &Money{Cents: m.Amount, Currency: m.Currency, Out: m.Out}
		}
		d, err := s.plane().Decide(ctx, ask)
		switch {
		case errors.Is(err, ErrAbsent):
			rec.Refusal = RefusalAbsent
		case err != nil:
			rec.Refusal = RefusalUnreachable
		default:
			action = Strictest(action, d.Action)
			rec.Score = d.Score
			rec.Agency = d.Agency
			rec.Decision = d.ID
			rec.Shadow = d.Shadow
			if d.Refusal != "" {
				rec.Refusal = d.Refusal
			}
			if len(d.Hits) > 0 {
				rec.Detail["hits"] = d.Hits
			}
		}
	}

	// A shadow decision is advisory by construction: it is recorded exactly as
	// the plane returned it and it does not stop money. The controls still do —
	// they are the org's own standing instruction, not a model's opinion.
	if rec.Shadow && !restraint.Blocked {
		action = Allow
	}
	rec.Action = string(action)
	if !action.Moves() {
		rec.Allowed = 0
		rec.Held = int64(m.Amount)
	}

	if err := rec.Create(); err != nil {
		return nil, err
	}
	return rec, nil
}

// Refused reports whether a screen stops the move it judged.
func Refused(rec *screen.Screen) bool { return !Action(rec.Action).Moves() }
