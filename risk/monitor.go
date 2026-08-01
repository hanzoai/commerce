// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"strconv"
	"time"

	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/outcome"
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
func Count(s *Screener, subject Subject) (*Standing, error) {
	if err := subject.Valid(); err != nil {
		return nil, err
	}
	st := &Standing{Subject: subject}

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

	for _, row := range outcome.For(s.DB, subject.Kind, subject.ID) {
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
	return st, nil
}

// Monitor counts the standing, puts it to the scoring plane as a merchant-stage
// question, and — when reserve is a rate and the plane restricts — places the
// control the answer implies.
//
// The mapping from an answer to a control is fixed and small, because a
// judgement is not an instruction: restrict means money stops leaving, block
// means money stops moving, and everything softer places nothing. A reserve is
// the one control the plane cannot imply on its own, since it needs a RATE, so
// the caller states it and a rate of zero means "hold instead of reserving".
func Monitor(ctx context.Context, s *Screener, subject Subject, reserve int64, act bool) (*Standing, error) {
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
	effect, rate := implied(Action(rec.Action), reserve)
	if effect == "" {
		return st, nil
	}
	c, err := Place(s, subject, effect, rate, time.Time{}, "risk review "+rec.Id())
	if err != nil {
		return nil, err
	}
	st.Placed = c.Id()
	st.Controls = append(st.Controls, c)
	return st, nil
}

// implied is the fixed mapping from a judgement to the control it implies.
func implied(a Action, reserve int64) (string, int64) {
	switch a {
	case Block:
		return control.Block, 0
	case Restrict:
		if reserve > 0 && reserve < control.FullRate {
			return control.Reserve, reserve
		}
		return control.Hold, 0
	default:
		return "", 0
	}
}

// Place writes a control, or returns the live one that already says the same
// thing. Placing is idempotent on (subject, kind) while a control is in force:
// a monitor that runs every cycle must not accumulate a hundred identical holds
// on one merchant, and releasing should take one act, not a hundred.
func Place(s *Screener, subject Subject, effect string, rate int64, until time.Time, reason string) (*control.Control, error) {
	if err := subject.Valid(); err != nil {
		return nil, err
	}
	if !control.Effects(effect) {
		return nil, ErrKind
	}
	if effect == control.Reserve && (rate <= 0 || rate > control.FullRate) {
		return nil, errRate
	}
	if effect != control.Reserve {
		rate = 0
	}

	now := s.now()
	live, err := control.LiveFor(s.DB, subject.Kind, subject.ID, now)
	if err != nil {
		return nil, err
	}
	for _, c := range live {
		if c.Effect == effect && c.Rate == rate {
			return c, nil
		}
	}

	c := control.New(s.DB)
	c.Effect = effect
	c.SubjectKind = subject.Kind
	c.Subject = subject.ID
	c.Rate = rate
	c.Until = until
	c.Reason = reason
	c.By = s.By
	if err := c.Create(); err != nil {
		return nil, err
	}
	return c, nil
}

// signals renders the counted standing as the facts the scoring plane reads.
// Every value is an exact integer rendered as a string; nothing is rounded on
// the way out.
func (st *Standing) signals() map[string]string {
	n := func(v int64) string { return strconv.FormatInt(v, 10) }
	return map[string]string{
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
	}
}
