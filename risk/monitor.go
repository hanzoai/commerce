// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"strconv"
	"time"

	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/outcome"
	"github.com/hanzoai/commerce/models/reserve"
	"github.com/hanzoai/commerce/models/screen"
	"github.com/hanzoai/commerce/models/types/currency"
)

// Standing is what one merchant's own record says about it, and what the
// scoring plane makes of that. It is the account risk score a platform watches
// continuously, and every number in it is counted from this org's rows — no
// other tenant's behaviour reaches it.
//
// The rates are BASIS POINTS, not fractions. A dispute rate is a money-adjacent
// number that gets compared against a threshold and quoted in an appeal; a
// float would make two services disagree in the fourth decimal about whether a
// merchant crossed 1%.
type Standing struct {
	Subject Subject `json:"subject"`

	Screens  int `json:"screens"`
	Refused  int `json:"refused"`
	Disputes int `json:"disputes"`
	Lost     int `json:"lost"`
	Refunds  int `json:"refunds"`
	Failed   int `json:"failed"`
	Negative int `json:"negative"`

	// DisputeRate and RefusalRate are basis points of screened moves.
	DisputeRate int64 `json:"disputeRate"`
	RefusalRate int64 `json:"refusalRate"`

	VolumeIn  currency.Cents `json:"volumeIn"`
	VolumeOut currency.Cents `json:"volumeOut"`
	Held      currency.Cents `json:"held"`

	// Window is how many of the subject's most recent screens and outcomes the
	// counts are over. The standing is a ROLLING WINDOW and says so: a number
	// counted over "everything ever" cannot be produced without a read that
	// grows with the merchant, and a rate over a merchant's whole lifetime is
	// the wrong number anyway — what a platform watches is behaviour now.
	Window int `json:"window"`

	// Reserved is what the reserve ledger currently withholds from this subject,
	// per currency, in exact minor units.
	Reserved map[string]int64 `json:"reserved,omitempty"`

	Controls []*control.Control `json:"controls,omitempty"`

	// Screen is the merchant-stage judgement just recorded — the score, and the
	// decision id an appeal cites.
	Screen *screen.Screen `json:"screen,omitempty"`
	// Placed names a control this review placed, when it was asked to act.
	Placed string `json:"placed,omitempty"`
}

// Count is the standing counted from the org's own rows, with no scoring hop.
// It is separated from [Monitor] because counting is what a cron does on every
// merchant every cycle, and asking is what it does when the counts move.
//
// The counts are over the subject's most recent [screen.Page] screens and
// [outcome.Page] outcomes — a bounded, newest-first window, reported in
// Standing.Window so the numbers are interpretable. Counting "everything" would
// make one merchant's history the size of one request.
func Count(s *Screener, subject Subject) (*Standing, error) {
	if err := subject.Valid(); err != nil {
		return nil, err
	}
	st := &Standing{Subject: subject, Window: screen.Page}

	for _, row := range screen.For(s.DB, subject.Kind, subject.ID, 0) {
		st.Screens++
		if !Action(row.Action).Moves() {
			st.Refused++
		}
		if row.Out {
			st.VolumeOut += currency.Cents(row.Allowed)
		} else {
			st.VolumeIn += currency.Cents(row.Allowed)
		}
		st.Held += currency.Cents(row.Held)
	}

	for _, row := range outcome.For(s.DB, subject.Kind, subject.ID, 0) {
		switch row.Event {
		case outcome.Dispute:
			st.Disputes++
		case outcome.Lost:
			st.Lost++
		case outcome.Refund:
			st.Refunds++
		case outcome.PayoutFail:
			st.Failed++
		case outcome.Negative:
			st.Negative++
		}
	}

	if st.Screens > 0 {
		st.DisputeRate = int64(st.Disputes) * control.FullRate / int64(st.Screens)
		st.RefusalRate = int64(st.Refused) * control.FullRate / int64(st.Screens)
	}

	live, err := control.LiveFor(s.DB, subject.Kind, subject.ID, s.now())
	if err != nil {
		return nil, err
	}
	st.Controls = live

	// What the reserve LEDGER holds, which is a different fact from what the
	// screens withheld: the screens are what was judged, the ledger is what is
	// actually being kept from the merchant right now.
	for _, b := range reserve.Balances(s.DB, subject.Kind, subject.ID, 0) {
		if b.Held == 0 {
			continue
		}
		if st.Reserved == nil {
			st.Reserved = map[string]int64{}
		}
		st.Reserved[string(b.Currency)] = b.Held
	}
	return st, nil
}

// Monitor counts the standing, puts it to the scoring plane as a merchant-stage
// question, and — when the plane restricts — places the control the answer
// implies.
//
// The mapping from an answer to a control is fixed and small, because a
// judgement is not an instruction: restrict means money stops leaving, block
// means money stops moving, and everything softer places nothing. A reserve is
// the one control the plane cannot imply on its own, because it needs a RATE
// and a CEILING; the caller states both, and stating neither means "hold
// instead of reserving".
func Monitor(ctx context.Context, s *Screener, subject Subject, rate, ceiling int64, act bool) (*Standing, error) {
	st, err := Count(s, subject)
	if err != nil {
		return nil, err
	}

	rec, err := s.Screen(ctx, Move{
		Stage:   Merchant,
		Subject: subject,
		Signals: st.signals(),
	})
	if err != nil {
		return nil, err
	}
	st.Screen = rec

	if !act {
		return st, nil
	}
	effect, rate, ceiling := implied(Action(rec.Action), rate, ceiling)
	if effect == "" {
		return st, nil
	}
	c, err := Place(s, subject, effect, rate, ceiling, time.Time{}, "risk review "+rec.Id())
	if err != nil {
		return nil, err
	}
	st.Placed = c.Id()
	st.Controls = append(st.Controls, c)
	return st, nil
}

// implied is the fixed mapping from a judgement to the control it implies. A
// reserve needs a rate AND a ceiling, neither of which a judgement can supply,
// so it is implied only when the caller stated both.
func implied(a Action, rate, ceiling int64) (string, int64, int64) {
	switch a {
	case Block:
		return control.Block, 0, 0
	case Restrict:
		if rate > 0 && rate < control.FullRate && ceiling > 0 {
			return control.Reserve, rate, ceiling
		}
		return control.Hold, 0, 0
	default:
		return "", 0, 0
	}
}

// Place writes a control, or returns the live one that already says the same
// thing. Placing is idempotent on (subject, effect, rate, cap) while a control
// is in force: a monitor that runs every cycle must not accumulate a hundred
// identical holds on one merchant, and releasing should take one act, not a
// hundred.
//
// A reserve MUST state a ceiling. See [errCap]: a rate with no total is not a
// reserve.
func Place(s *Screener, subject Subject, effect string, rate, ceiling int64, until time.Time, reason string) (*control.Control, error) {
	if err := subject.Valid(); err != nil {
		return nil, err
	}
	if !control.Effects(effect) {
		return nil, ErrKind
	}
	if effect == control.Reserve {
		if rate <= 0 || rate > control.FullRate {
			return nil, errRate
		}
		if ceiling <= 0 {
			return nil, errCap
		}
	} else {
		rate, ceiling = 0, 0
	}

	now := s.now()
	live, err := control.LiveFor(s.DB, subject.Kind, subject.ID, now)
	if err != nil {
		return nil, err
	}
	for _, c := range live {
		if c.Effect == effect && c.Rate == rate && c.Cap == ceiling {
			return c, nil
		}
	}

	c := control.New(s.DB)
	c.Effect = effect
	c.SubjectKind = subject.Kind
	c.Subject = subject.ID
	c.Rate = rate
	c.Cap = ceiling
	c.Until = until
	c.Reason = reason
	c.By = s.By
	if err := c.Create(); err != nil {
		return nil, err
	}
	return c, nil
}

// Release lifts one control and, when it was the last reserve standing over the
// subject, frees what the reserve ledger was holding. Releasing a restraint
// that keeps the money it withheld is not a release.
//
// Releasing twice is a no-op: the first release's author and time stand, and
// the pool was already freed.
func Release(s *Screener, c *control.Control) (*control.Control, error) {
	if c.Released {
		return c, nil
	}
	now := s.now()
	c.Release(s.By, now)
	if err := c.Update(); err != nil {
		return nil, err
	}
	if c.Effect != control.Reserve {
		return c, nil
	}

	subject := Subject{Kind: c.SubjectKind, ID: c.Subject}
	live, err := control.LiveFor(s.DB, subject.Kind, subject.ID, now)
	if err != nil {
		return nil, err
	}
	for _, other := range live {
		if other.Effect == control.Reserve {
			// Another reserve still stands over this subject, and the pool is
			// per subject — the strictest reserve sets the rate, so the money
			// stays held until none is left in force.
			return c, nil
		}
	}
	if _, err := s.Free(subject, "reserve "+c.Ref()+" was released"); err != nil {
		return nil, err
	}
	return c, nil
}

// signals renders the counted standing as the facts the scoring plane reads.
// Every value is an exact integer rendered as a string; nothing is rounded on
// the way out. The WINDOW travels with the counts, because a count without its
// denominator is a number a model can only misread.
//
// Every key here is in the closed set that [Facts] admits — see the note on
// signalKeys. Two vocabularies, one gate.
func (st *Standing) signals() map[string]string {
	n := func(v int64) string { return strconv.FormatInt(v, 10) }
	out := map[string]string{
		"screens":     n(int64(st.Screens)),
		"refused":     n(int64(st.Refused)),
		"disputes":    n(int64(st.Disputes)),
		"lost":        n(int64(st.Lost)),
		"refunds":     n(int64(st.Refunds)),
		"failed":      n(int64(st.Failed)),
		"negative":    n(int64(st.Negative)),
		"disputerate": n(st.DisputeRate),
		"refusalrate": n(st.RefusalRate),
		"volumein":    n(int64(st.VolumeIn)),
		"volumeout":   n(int64(st.VolumeOut)),
		"held":        n(int64(st.Held)),
		"window":      n(int64(st.Window)),
	}
	var reserved int64
	for _, v := range st.Reserved {
		reserved += v
	}
	if reserved > 0 {
		out["reserved"] = n(reserved)
	}
	return out
}
