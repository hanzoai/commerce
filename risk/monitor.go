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

// Window is how many recent screens and outcomes a standing is counted over.
//
// A standing is a view of RECENT behaviour, and it is bounded because it is
// read on the money path: a merchant with a million screens must not be able to
// make one request materialise a million rows in a process shared with every
// other tenant. The window is DISCLOSED on the answer ([Standing.Window],
// [Standing.Truncated]) rather than implied, because a dispute rate quoted
// without saying what it is a rate OF is a number nobody can check.
const Window = screen.Max

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

	// Window is how many recent rows these counts are over, and Truncated says
	// the subject has more than that — so a rate is never quoted as if it
	// covered everything when it covered a window.
	Window    int  `json:"window"`
	Truncated bool `json:"truncated,omitempty"`

	// DisputeRate and RefusalRate are basis points of screened moves.
	DisputeRate int64 `json:"disputeRate"`
	RefusalRate int64 `json:"refusalRate"`

	VolumeIn  currency.Cents `json:"volumeIn"`
	VolumeOut currency.Cents `json:"volumeOut"`
	Held      currency.Cents `json:"held"`

	// Reserved is what the reserves in force are currently HOLDING of this
	// subject's money, cumulative and exact — the running total the ceiling is
	// measured against, read off the declarations themselves. Held above is what
	// the counted window's screens withheld; this is the account.
	Reserved currency.Cents `json:"reserved"`

	Controls []*control.Control `json:"controls,omitempty"`

	// Ledger is the recent movements of this subject's reserved money. A
	// reserve nobody can read back is not an account, it is a shortfall on a
	// payout with no explanation attached.
	Ledger []*reserve.Entry `json:"ledger,omitempty"`

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
	st := &Standing{Subject: subject, Window: Window}

	rows := screen.For(s.DB, subject.Kind, subject.ID, Window)
	st.Truncated = len(rows) >= Window
	for _, row := range rows {
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

	events := outcome.For(s.DB, subject.Kind, subject.ID, Window)
	if len(events) >= Window {
		st.Truncated = true
	}
	for _, row := range events {
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
	held := reserve.Accounts(s.DB) // one read for the whole set, never one per control
	for _, c := range live {
		st.Reserved += currency.Cents(held[c.Ref()])
	}
	st.Ledger = reserve.For(s.DB, subject.Kind, subject.ID, ledger)
	return st, nil
}

// ledger bounds how much of the reserve account one standing carries. It is a
// recent view for reconciliation, not the whole history — the history is in the
// store and stays there.
const ledger = 50

// Monitor counts the standing, puts it to the scoring plane as a merchant-stage
// question, and — when reserve is a rate and the plane restricts — places the
// control the answer implies.
//
// The mapping from an answer to a control is fixed and small, because a
// judgement is not an instruction: restrict means money stops leaving, block
// means money stops moving, and everything softer places nothing. A reserve is
// the one control the plane cannot imply on its own, since it needs a RATE, so
// the caller states it and a rate of zero means "hold instead of reserving".
func Monitor(ctx context.Context, s *Screener, subject Subject, rate int64, act bool) (*Standing, error) {
	st, err := Count(s, subject)
	if err != nil {
		return nil, err
	}

	rec, err := s.Screen(ctx, Move{
		Stage:    Merchant,
		Subject:  subject,
		Standing: st.counted(),
	})
	if err != nil {
		return nil, err
	}
	st.Screen = rec

	if !act {
		return st, nil
	}
	effect, share := implied(Action(rec.Action), rate)
	if effect == "" {
		return st, nil
	}
	c, err := Place(s, Placement{
		Subject: subject,
		Effect:  effect,
		Rate:    share,
		Reason:  "risk review " + rec.Id(),
	})
	if err != nil {
		return nil, err
	}
	st.Placed = c.Id()
	st.Controls = append(st.Controls, c)
	return st, nil
}

// implied is the fixed mapping from a judgement to the control it implies.
func implied(a Action, rate int64) (string, int64) {
	switch a {
	case Block:
		return control.Block, 0
	case Restrict:
		if rate > 0 && rate < control.FullRate {
			return control.Reserve, rate
		}
		return control.Hold, 0
	default:
		return "", 0
	}
}

// Placement is one standing restraint a caller asks for. It is a VALUE and not
// a parameter list because a reserve is now four facts that must agree — rate,
// ceiling, currency, expiry — and four positional arguments is how a ceiling
// ends up in the rate.
type Placement struct {
	Subject Subject
	// Effect is reserve, hold or block.
	Effect string
	// Rate is basis points withheld from each outbound move, for a reserve.
	Rate int64
	// Cap is the CEILING on what a reserve may withhold in total, exact minor
	// units of Currency. Zero declares no ceiling.
	Cap currency.Cents
	// Currency denominates the ceiling and scopes the reserve to one currency.
	// Required with a ceiling, because an amount without a currency is a number.
	Currency currency.Type
	// Until lapses the control. Zero stands until released, which is what a
	// fraud restraint should do.
	Until  time.Time
	Reason string
}

// Place writes a control, or returns the live one that already says the same
// thing. Placing is idempotent on (subject, effect, rate) while a control is in
// force: a monitor that runs every cycle must not accumulate a hundred
// identical holds on one merchant, and releasing should take one act, not a
// hundred.
func Place(s *Screener, p Placement) (*control.Control, error) {
	if err := p.Subject.Valid(); err != nil {
		return nil, err
	}
	// A reason is read back on every list, up to control.Max of them at a time.
	// The row cap bounds the COUNT; [Text] is what bounds the bytes.
	if err := Bound(p.Effect, p.Subject.Kind, p.Subject.ID, string(p.Currency), p.Reason); err != nil {
		return nil, err
	}
	if !control.Effects(p.Effect) {
		return nil, ErrKind
	}
	if p.Effect == control.Reserve {
		if p.Rate <= 0 || p.Rate > control.FullRate {
			return nil, errRate
		}
		if p.Cap < 0 || (p.Cap > 0 && p.Currency == "") {
			return nil, errCap
		}
	} else {
		// A rate, a ceiling and a currency describe a reserve. On a hold or a
		// block they would be stored and silently never applied, which is worse
		// than refusing them: a platform would believe it had capped something.
		p.Rate, p.Cap, p.Currency = 0, 0, ""
	}

	now := s.now()
	live, err := control.LiveFor(s.DB, p.Subject.Kind, p.Subject.ID, now)
	if err != nil {
		return nil, err
	}
	for _, c := range live {
		if c.Effect == p.Effect && c.Rate == p.Rate && c.Cap == int64(p.Cap) && c.Currency == p.Currency {
			return c, nil
		}
	}

	c := control.New(s.DB)
	c.Effect = p.Effect
	c.SubjectKind = p.Subject.Kind
	c.Subject = p.Subject.ID
	c.Rate = p.Rate
	c.Cap = int64(p.Cap)
	c.Currency = p.Currency
	c.Until = p.Until
	c.Reason = p.Reason
	c.By = s.By
	if err := c.Create(); err != nil {
		return nil, err
	}
	return c, nil
}

// Lift releases the control named by id and RETURNS EXACTLY what it was
// holding, so the account closes instead of leaving money withheld under a
// declaration that no longer exists.
//
// What comes back is the account's own total, zeroed and posted to the ledger
// in ONE store transaction — so the release cannot disagree with the holds it
// returns. Releasing twice returns the money once, because the second Close
// finds an empty account.
//
// It takes an ID and not a control, so it cannot be handed a row from a query —
// which in this ORM is a row whose writes land nowhere (see control.Lift).
func Lift(s *Screener, id string) (*control.Control, error) {
	c, err := control.Lift(s.DB, id, s.By, s.now())
	if err != nil {
		return nil, err
	}
	if c.Effect != control.Reserve {
		return c, nil
	}
	if _, err := reserve.Close(s.DB, c); err != nil {
		return nil, err
	}
	return c, nil
}

// counted renders the counted standing as the facts the scoring plane reads.
// Every value is an exact integer rendered as a string; nothing is rounded on
// the way out, and none of it is reachable from a caller — see [Move.Standing].
func (st *Standing) counted() map[string]string {
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
		"reserved":    n(int64(st.Reserved)),
	}
}
