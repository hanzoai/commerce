// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"
	"strings"
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

// ErrReused refuses an idempotency key that names a different move than the one
// it first named.
//
// A key is the name of ONE question. Answering a second, different question
// with the first question's answer is how a caller screens a one-cent move and
// spends the verdict on ten thousand dollars — so the second question is
// refused outright rather than answered, and no money moves on it at all.
var ErrReused = errors.New("risk: this idempotency key already names a different move")

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

// digest fingerprints the move: everything that makes it THIS move and nothing
// that makes it this ATTEMPT. The idempotency key is deliberately not part of
// it — the digest is what the key is checked against.
func (m Move) digest() string {
	h := sha256.New()
	write := func(parts ...string) {
		for _, p := range parts {
			h.Write([]byte(p))
			h.Write([]byte{0})
		}
	}
	write(string(m.Stage), m.Subject.Kind, m.Subject.ID,
		strconv.FormatInt(int64(m.Amount), 10), strings.ToLower(string(m.Currency)),
		strconv.FormatBool(m.Out), m.Reference, m.Processor)

	keys := make([]string, 0, len(m.Signals))
	for k := range m.Signals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		write(k, m.Signals[k])
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// Screen judges one move and RECORDS the judgement before returning it. The
// record is written first for the same reason a ledger entry is: an answer the
// money plane acted on and did not keep is an answer that cannot be defended in
// a dispute.
//
// ORDER IS THE WHOLE DESIGN, and it is this:
//
//	controls  →  idempotency  →  scoring  →  record
//
// The CONTROLS are read first, every time, from the org's own store — so a
// scoring outage can never lift a reserve, AND NEITHER CAN A REPLAY. An
// idempotency key makes the SCORE idempotent (one question, one verdict, one
// charge) and it is applied AFTER the controls precisely so that it can never
// hand back a verdict the controls have since overtaken. A move the controls
// already stop is not sent for scoring at all: spending the authorization
// budget on advice that cannot change the outcome is exactly the latency an
// attacker hammering a blocked merchant wants to buy.
func (s *Screener) Screen(ctx context.Context, m Move) (*screen.Screen, error) {
	if err := m.Subject.Valid(); err != nil {
		return nil, err
	}
	if m.Amount < 0 {
		return nil, errors.New("risk: amount is negative")
	}
	stated, err := Facts(m.Signals)
	if err != nil {
		return nil, err
	}
	m.Signals = stated
	now := s.now()

	// 1. THE CONTROLS. Durable rows in this org's own store, read with no
	//    network in the path and nothing cached in front of them.
	live, err := control.LiveFor(s.DB, m.Subject.Kind, m.Subject.ID, now)
	if err != nil {
		return nil, err
	}
	held, err := s.settle(live, m.Subject, m.Currency, now)
	if err != nil {
		return nil, err
	}
	restraint := Restrain(live, m.Amount, m.Out, now, held)

	// 2. IDEMPOTENCY, against the controls just read. A repeat gets the first
	//    answer re-asserted — never a laxer one, and never another move's.
	if prior, ok := screen.ByIdem(s.DB, m.Idem); ok {
		if prior.Digest != "" && prior.Digest != m.digest() {
			return nil, ErrReused
		}
		return s.reassert(prior, restraint)
	}

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
	rec.Digest = m.digest()
	rec.Held = int64(restraint.Held)
	rec.Allowed = int64(restraint.Allowed)
	rec.Reason = restraint.Reason
	rec.Detail = map[string]any{}
	if len(restraint.Controls) > 0 {
		rec.Detail["controls"] = restraint.Controls
	}
	if restraint.Reserve != "" {
		rec.Detail["reserve"] = restraint.Reserve
	}
	if len(m.Signals) > 0 {
		rec.Detail["signals"] = m.Signals
	}

	// 3. SCORING, only when the controls left something to decide.
	action := restraint.Action()
	if !restraint.Blocked {
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

	// 4. THE RECORD.
	if err := rec.Create(); err != nil {
		return nil, err
	}
	return rec, nil
}

// reassert answers a repeat with the first answer, tightened by the controls in
// force NOW.
//
// Composition is [Strictest], which makes the repeat safe from both sides. A
// block placed since the first answer TIGHTENS the repeat, so a cached verdict
// can never release money the org has since stopped. A block LIFTED since the
// first answer does not loosen it, because the answer to one question does not
// change under the caller — that is what an idempotency key promises.
//
// The row is rewritten only when the effective answer actually moved, so a
// retry storm against an unchanged posture is pure reads. What is rewritten is
// the ENFORCEMENT — the action and the split. The provenance of the original
// judgement (its score, its decision id, its refusal) is left exactly as it
// was: that is the evidence, and it did not happen twice.
func (s *Screener) reassert(prior *screen.Screen, r Restraint) (*screen.Screen, error) {
	action := Strictest(Action(prior.Action), r.Action())
	held := currency.Cents(prior.Held)
	if r.Held > held {
		held = r.Held
	}
	allowed := currency.Cents(prior.Amount) - held
	if !action.Moves() || allowed < 0 {
		held, allowed = currency.Cents(prior.Amount), 0
	}

	if string(action) == prior.Action && int64(held) == prior.Held && int64(allowed) == prior.Allowed {
		return prior, nil
	}

	prior.Action = string(action)
	prior.Held = int64(held)
	prior.Allowed = int64(allowed)
	prior.Reasserted++
	if r.Reason != "" {
		prior.Reason = r.Reason
	}
	if prior.Detail == nil {
		prior.Detail = map[string]any{}
	}
	if len(r.Controls) > 0 {
		prior.Detail["controls"] = r.Controls
	}
	prior.Detail["reasserted"] = prior.Reasserted
	if err := prior.Update(); err != nil {
		return nil, err
	}
	return prior, nil
}

// Refused reports whether a screen stops the move it judged.
func Refused(rec *screen.Screen) bool { return !Action(rec.Action).Moves() }
