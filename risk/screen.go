// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/reserve"
	"github.com/hanzoai/commerce/models/screen"
	"github.com/hanzoai/commerce/models/types/currency"
)

// ErrRefused is what a money move gets when the money plane will not let it
// happen. It is a distinct error so a caller can render a clean refusal instead
// of a gateway failure — a refused move is a decision, not a fault.
var ErrRefused = errors.New("risk: the move is refused")

// ErrIdem is what a repeat gets when it reuses an idempotency key for a
// DIFFERENT move. The key names one question; answering a second question with
// the first one's answer is how a $1 screen authorizes a $1,000,000 payout.
var ErrIdem = errors.New("risk: this idempotency key was used for a different move")

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
	Reference string
	Processor string

	// Signals are CALLER-supplied facts. They pass through [Facts] on the way
	// in, so what a caller can send and what this plane will store are one
	// bounded, allowlisted set.
	Signals map[string]string

	// Standing is facts the money plane COUNTED for itself — a merchant's own
	// dispute rate, its refused share, its volumes. It is deliberately a
	// separate field from Signals and reachable from no typed op: a caller that
	// could state its own dispute rate would be scored on a number it chose.
	Standing map[string]string

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
// ORDER IS THE WHOLE SAFETY PROPERTY, and it is exactly this:
//
//	controls  →  idempotency  →  scoring  →  record
//
// The CONTROLS are read first, from the org's own store, so a scoring outage
// can never lift a reserve, and a move the controls already stop is not sent
// for scoring at all — spending the authorization budget on advice that cannot
// change the outcome is exactly the latency an attacker hammering a blocked
// merchant wants to buy.
//
// IDEMPOTENCY COMES AFTER THE CONTROLS, never before. Answering a repeat from
// the stored row before applying them makes the key a way to LIFT A LIVE
// CONTROL: screen once while the subject is clean, keep the key, and replay it
// after the block lands — the block is in the store, in force, and the money
// moves anyway on the strength of an answer given before it existed. A repeat
// gets the SAME ANSWER TO THE SAME QUESTION, and the question is what the
// caller asked; the controls are the world, and the world is re-read every
// time. So a repeat is re-asserted against the live controls and may only ever
// come back STRICTER — never looser, never the stale permission.
func (s *Screener) Screen(ctx context.Context, m Move) (*screen.Screen, error) {
	if err := m.Subject.Valid(); err != nil {
		return nil, err
	}
	if m.Amount < 0 {
		return nil, errors.New("risk: amount is negative")
	}
	m.Signals = Facts(m.Signals)
	now := s.now()

	live, err := control.LiveFor(s.DB, m.Subject.Kind, m.Subject.ID, now)
	if err != nil {
		return nil, err
	}
	restraint := Restrain(live, m.Amount, m.Currency, m.Out, now)
	restraint = Cap(restraint, Headroom(restraint.Reserve))

	if prior, ok := screen.ByIdem(s.DB, m.Idem); ok {
		return s.repeat(prior, m, restraint, now)
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
	rec.Fingerprint = fingerprint(m)
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
		ask := &Ask{Stage: m.Stage, Subject: m.Subject, Signals: m.facts(), Idem: m.Idem}
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

	// The money a reserve withheld is ACCOUNTED FOR, against the declaration
	// that took it. A withheld cent that nothing records is not a reserve, it is
	// a shortfall on a merchant's payout with no name and no way back.
	if err := s.withhold(rec, restraint); err != nil {
		return nil, err
	}
	return rec, nil
}

// repeat answers a move whose idempotency key was already used.
//
// It gives the SAME ANSWER TO THE SAME QUESTION and refuses a different one:
// the fingerprint is the question, and a key reused for another move gets
// [ErrIdem] rather than the first move's Allowed. The answer it gives is then
// re-asserted against the controls in force NOW, so a repeat can only ever come
// back stricter than the row says.
func (s *Screener) repeat(prior *screen.Screen, m Move, r Restraint, now time.Time) (*screen.Screen, error) {
	if prior.Fingerprint != fingerprint(m) {
		return nil, ErrIdem
	}
	if !reassert(prior, r) {
		return prior, nil
	}
	// The world tightened, so the RECORD has to. prior came from a query and is
	// not writable — see screen.Writable — so the same composition is applied to
	// a row read back by id, which is the one that lands.
	rec, err := screen.Writable(s.DB, prior.Id())
	if err != nil {
		return nil, err
	}
	reassert(rec, r)
	rec.Reasserts++
	rec.ReassertedAt = now
	if err := rec.Update(); err != nil {
		return nil, err
	}
	return rec, nil
}

// reassert tightens a recorded answer to the controls in force now, and reports
// whether it moved. It is pure over the two values and never loosens: the
// action composes by [Strictest] and the allowed amount can only fall.
//
// It does NOT post to the reserve ledger. A repeat moves no money — the money
// moved on the first answer — so a hold recorded twice would be a hold that
// happened once.
func reassert(prior *screen.Screen, r Restraint) bool {
	action := Strictest(Action(prior.Action), refusal(r))
	allowed := prior.Allowed
	if !r.Blocked && int64(r.Allowed) < allowed {
		allowed = int64(r.Allowed)
	}
	if r.Blocked || !action.Moves() {
		allowed = 0
	}
	held := prior.Amount - allowed

	if string(action) == prior.Action && allowed == prior.Allowed && held == prior.Held {
		return false
	}
	// Keep what this row FIRST answered. A record that quietly rewrote itself is
	// not evidence, and the first answer is the one an earlier payment was made
	// on.
	if prior.Detail == nil {
		prior.Detail = map[string]any{}
	}
	if _, seen := prior.Detail["asFirstAnswered"]; !seen {
		prior.Detail["asFirstAnswered"] = map[string]any{
			"action":  prior.Action,
			"allowed": prior.Allowed,
			"held":    prior.Held,
		}
	}
	if len(r.Controls) > 0 {
		prior.Detail["controls"] = r.Controls
	}
	if r.Reason != "" {
		prior.Reason = r.Reason
	}
	prior.Action = string(action)
	prior.Allowed = allowed
	prior.Held = held
	return true
}

// refusal is what the controls alone say, as an action. Restrain reports a hold
// and a block the same way — nothing moves — so this plane names that Block and
// composes it with whatever the scoring plane said.
func refusal(r Restraint) Action {
	if r.Blocked {
		return Block
	}
	return Allow
}

// withhold accounts for money a reserve took: a durable ledger entry naming the
// judgement and the declaration, and the running total on the control the
// ceiling is measured against.
//
// It fails the SCREEN when it cannot record the hold. A reserve whose account
// did not take the entry is a reserve that is not bounded by its own ceiling,
// and the money plane must not hand back an Allowed it cannot account for.
func (s *Screener) withhold(rec *screen.Screen, r Restraint) error {
	if r.Reserve == nil || r.Held <= 0 || rec.Held <= 0 {
		return nil
	}
	if _, err := reserve.Hold(s.DB, rec.SubjectKind, rec.Subject, rec.Currency,
		int64(r.Held), r.Reserve.Ref(), rec.Id(), rec.Reference); err != nil {
		return err
	}
	_, err := control.Withhold(s.DB, r.Reserve.Ref(), int64(r.Held))
	return err
}

// facts is what actually travels to the scoring plane: the caller's allowlisted
// signals, with the money plane's own counted standing over the top. Counted
// facts win, because a fact we counted is not open to a caller's opinion.
func (m Move) facts() map[string]string {
	if len(m.Standing) == 0 {
		return m.Signals
	}
	out := make(map[string]string, len(m.Signals)+len(m.Standing))
	for k, v := range m.Signals {
		out[k] = v
	}
	for k, v := range m.Standing {
		out[k] = v
	}
	return out
}

// fingerprint is the exact money question a screen answered, as a stable digest:
// what stage, about whom, for how much, in what currency, which way, against
// which money object and through which processor.
//
// It is what makes an idempotency key mean "the same request" rather than
// merely "the same string". Signals are NOT in it: they are what we know about
// the request, not what the request is, and a retry that carries a fresher user
// agent is still the same payment.
func fingerprint(m Move) string {
	h := sha256.New()
	for _, part := range []string{
		string(m.Stage), m.Subject.Kind, m.Subject.ID,
		strconv.FormatInt(int64(m.Amount), 10), string(m.Currency),
		strconv.FormatBool(m.Out), m.Reference, m.Processor,
	} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// Refused reports whether a screen stops the move it judged.
func Refused(rec *screen.Screen) bool { return !Action(rec.Action).Moves() }
