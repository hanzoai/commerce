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
	// Every caller string on this move is bounded HERE, before anything is
	// stored, so both doors into the plane get the same ceiling — see [Text].
	if err := Bound(string(m.Stage), m.Subject.Kind, m.Subject.ID,
		string(m.Currency), m.Reference, m.Processor, m.Idem); err != nil {
		return nil, err
	}
	var dropped int
	m.Signals, dropped = Facts(m.Signals)
	now := s.now()

	live, err := control.LiveFor(s.DB, m.Subject.Kind, m.Subject.ID, now)
	if err != nil {
		return nil, err
	}
	restraint := Restrain(live, m.Amount, m.Currency, m.Out, now)
	restraint = Cap(restraint, currency.Cents(reserve.Headroom(s.DB, restraint.Reserve)))

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
	if restraint.Reserve != nil {
		rec.Reserve = restraint.Reserve.Ref()
	}
	rec.Detail = map[string]any{}
	if len(restraint.Controls) > 0 {
		rec.Detail["controls"] = restraint.Controls
	}
	if len(m.Signals) > 0 {
		rec.Detail["signals"] = m.Signals
	}
	// Every bound this judgement's inputs hit, counted on the judgement. A
	// dropped signal or a truncated rule list is a fact the score was NOT
	// computed from; recording nothing would leave an operator reading a flat
	// score with no way to learn its inputs never arrived.
	lost := map[string]any{}
	if dropped > 0 {
		lost["signals"] = dropped
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
			h, lostHits := hits(d.Hits)
			if len(h) > 0 {
				rec.Detail["hits"] = h
			}
			if lostHits > 0 {
				lost["hits"] = lostHits
			}
		}
	}
	if len(lost) > 0 {
		rec.Detail["dropped"] = lost
	}

	// A shadow decision is advisory by construction: it is recorded exactly as
	// the plane returned it and it does not stop money. The controls still do —
	// they are the org's own standing instruction, not a model's opinion.
	if rec.Shadow && !restraint.Blocked {
		action = Allow
	}
	rec.Action = string(action)
	if !action.Moves() {
		// A refused move withholds everything BY NOT HAPPENING, and that is not a
		// reserve taking money — it is money that never left. Naming the reserve
		// here would let the disbursement boundary withhold against a payout the
		// plane blocked: a merchant's ceiling burned on moves that never occurred,
		// and a ledger saying money was taken out of nothing.
		rec.Allowed = 0
		rec.Held = int64(m.Amount)
		rec.Reserve = ""
	}

	if err := rec.Create(); err != nil {
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
// It moves no money, and neither does the judgement it belongs to: what a
// repeat answers becomes real when the disbursement withholds it ([Withhold]),
// against the judgement's own id, which is why a tightened repeat and a first
// answer are accounted for by exactly the same act.
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
	reserved := prior.Reserve
	if r.Reserve != nil {
		reserved = r.Reserve.Ref()
	}
	if r.Blocked || !action.Moves() {
		reserved = "" // refused: nothing was taken, so nothing took it
	}

	if string(action) == prior.Action && allowed == prior.Allowed &&
		held == prior.Held && reserved == prior.Reserve {
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
	prior.Reserve = reserved
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

// Withhold takes the reserve's share of a judged move AT THE MOMENT THE MONEY
// LEAVES, and reports what may actually go out and what was actually held.
//
// THIS IS THE ACCOUNTING BOUNDARY, and it is deliberately not [Screen]. A
// judgement answers a question; it must not spend a ceiling, because a ceiling
// any authenticated caller can consume with a money-free screen is worse than
// no ceiling — it switches the reserve off. So the account moves here, once,
// for the amount the money actually moved, called by the disbursement that is
// about to write the payout row.
//
// It is IDEMPOTENT ON THE JUDGEMENT: a retried disbursement under one screen
// re-reads its own hold and takes nothing more, because the screen is
// idempotent and the two must agree about how many times one move happened.
//
// It returns the AUTHORITATIVE split. The judgement's Held is a forecast
// clamped against a headroom read outside any transaction; this clamp happens
// inside one, so the reserve may withhold LESS than the judgement said — never
// more — and then more money leaves, which the caller discloses.
func (s *Screener) Withhold(rec *screen.Screen) (allowed, held currency.Cents, err error) {
	if rec == nil {
		return 0, 0, errors.New("risk: nothing to disburse")
	}
	if !Action(rec.Action).Moves() {
		// A refused move moves nothing, so nothing may be withheld from it. The
		// gate refuses before this point; this is the invariant, not the gate.
		return 0, 0, nil
	}
	if rec.Held <= 0 || rec.Reserve == "" {
		// NOTHING NAMED, NOTHING WITHHELD, so the whole amount goes out — and
		// deliberately not rec.Allowed. A judgement carrying Held with no reserve
		// to take it is not a reserve, it is a shortfall on a merchant's payout
		// that no ledger entry names, no ceiling counts and no lift returns. Held
		// and Allowed always sum to Amount, so paying Amount here is exact.
		return currency.Cents(rec.Amount), 0, nil
	}
	c, err := s.reserve(rec)
	if err != nil {
		return 0, 0, err
	}
	if c == nil {
		// The declaration is gone or no longer a live reserve: nothing may be
		// withheld under it, so the whole move goes out.
		return currency.Cents(rec.Amount), 0, nil
	}
	took, err := reserve.Take(s.DB, c, rec.Held, reserve.Cause{
		SubjectKind: rec.SubjectKind,
		Subject:     rec.Subject,
		Currency:    rec.Currency,
		Screen:      rec.Id(),
		Reference:   rec.Reference,
	})
	if err != nil {
		return 0, 0, err
	}
	return currency.Cents(rec.Amount - took), currency.Cents(took), nil
}

// Restore gives back what [Withhold] took, because the move it was taken from
// did not happen after all.
//
// A disbursement that failed AFTER its share was withheld would otherwise leave
// the merchant short by that share forever, under a ceiling that believes it is
// that much fuller. It is idempotent on the judgement, so calling it on a move
// that withheld nothing is not an error.
func (s *Screener) Restore(rec *screen.Screen) error {
	if rec == nil || rec.Reserve == "" {
		return nil
	}
	c, err := s.reserve(rec)
	if err != nil || c == nil {
		return err
	}
	_, err = reserve.Return(s.DB, c, rec.Id())
	return err
}

// reserve reads back the declaration a judgement named, BY ID, because a
// control that came from a query cannot be written through in this ORM and the
// account's transaction needs a row it can name.
//
// A declaration that is gone, is not a reserve, or is NO LONGER IN FORCE
// returns nil and no error: it holds nothing further. Withholding under a
// declaration that has been lifted is precisely the money-under-something-that-
// no-longer-exists that lifting a reserve is supposed to end, and the direction
// it errs in — the whole payout goes out — is the one that does not strand a
// merchant's money.
func (s *Screener) reserve(rec *screen.Screen) (*control.Control, error) {
	c := control.New(s.DB)
	if err := c.GetById(rec.Reserve); err != nil {
		if errors.Is(err, datastore.ErrNoSuchEntity) {
			return nil, nil
		}
		return nil, err
	}
	if c.Effect != control.Reserve || !c.Live(s.now()) {
		return nil, nil
	}
	return c, nil
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
