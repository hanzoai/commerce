// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"time"

	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/reserve"
	"github.com/hanzoai/commerce/models/screen"
	"github.com/hanzoai/commerce/models/types/currency"
)

// reserve.go is where a withheld share becomes ACCOUNTING.
//
// The split is deliberate and it is the whole reason the ledger is trustworthy:
//
//	SCREENING decides how much of a move a reserve withholds. It moves no
//	money and posts nothing, so a merchant may call POST /screen a thousand
//	times and its reserve balance does not move a cent.
//
//	HOLDING posts that share to the ledger, and it happens at the MONEY
//	BOUNDARY — after the payout row exists — because that is the moment the
//	money actually did not leave. Wired to the screen instead, a caller could
//	inflate its own reserve by asking questions.
//
//	SETTLING releases the pool when no reserve control stands over the
//	subject any more, so a lapsed or lifted reserve gives the money back with
//	no cron to forget to run.

// settle reads what the subject's reserve ledger holds and releases it when
// nothing requires it any more.
//
// It runs only for money LEAVING, which is the only direction a reserve bears
// on. That also keeps it off the authorization path entirely: an inbound charge
// touches the ledger not at all.
//
// The release is LAZY on purpose — evaluated the next time the subject moves
// money out, and on the release of the control itself. A lapsed reserve on a
// merchant that never moves money again keeps a balance that GET /reserves
// still shows and a release still frees; what it does not do is need a cron
// that must be running for the accounting to be correct.
func (s *Screener) settle(live []*control.Control, subject Subject, cur currency.Type, now time.Time) (currency.Cents, error) {
	_ = now
	for _, c := range live {
		if c != nil && c.Effect == control.Reserve {
			// A reserve stands: the ledger holds what it holds.
			return reserve.Held(s.DB, subject.Kind, subject.ID, cur), nil
		}
	}
	if _, err := s.Free(subject, "the reserve is no longer in force"); err != nil {
		return 0, err
	}
	return 0, nil
}

// Free releases everything the subject's reserve ledger holds, in every
// currency it holds it in, and returns the entries it wrote. Releasing an empty
// pool writes nothing and is not an error.
//
// It is what a reserve control's release means in money: the restraint is
// lifted, so the money it was withholding is the merchant's again.
func (s *Screener) Free(subject Subject, reason string) ([]*reserve.Entry, error) {
	out := []*reserve.Entry{}
	for _, b := range reserve.Balances(s.DB, subject.Kind, subject.ID, 0) {
		if b.Held <= 0 {
			continue
		}
		e := reserve.NewEntry(s.DB)
		e.SubjectKind = b.SubjectKind
		e.Subject = b.Subject
		e.Currency = b.Currency
		e.Amount = -b.Held
		e.Reason = reason
		e.By = s.By
		written, _, err := reserve.Post(s.DB, e)
		if err != nil {
			return out, err
		}
		if written != nil {
			out = append(out, written)
		}
	}
	return out, nil
}

// Hold posts a screen's withheld share to the reserve ledger. It is called at
// the money boundary — once the move it judged has actually happened — and it
// is idempotent on the screen: a retried payout replays the SAME screen row, so
// the second call finds the share already posted and writes nothing.
//
// The screen is marked BEFORE the ledger is posted. A crash between the two
// under-holds by one move, which returns money to the merchant that the
// platform meant to keep; the other order over-holds, which takes money from a
// merchant twice for one payout. Between two ways to be wrong, be wrong in the
// direction that does not take somebody's money.
func (s *Screener) Hold(rec *screen.Screen, reason string) (*reserve.Entry, error) {
	if rec == nil || rec.Held <= 0 || rec.Posted {
		return nil, nil
	}
	rec.Posted = true
	if err := rec.Update(); err != nil {
		return nil, err
	}

	e := reserve.NewEntry(s.DB)
	e.SubjectKind = rec.SubjectKind
	e.Subject = rec.Subject
	e.Currency = rec.Currency
	e.Amount = rec.Held
	e.Screen = rec.Id()
	e.Reason = reason
	e.By = s.By
	if id, ok := rec.Detail["reserve"].(string); ok {
		e.Control = id
	}
	written, _, err := reserve.Post(s.DB, e)
	return written, err
}
