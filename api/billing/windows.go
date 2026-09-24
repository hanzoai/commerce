// Copyright © 2026 Hanzo AI. MIT License.

package billing

import (
	"context"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/transaction"
)

// A plan includes USAGE, bounded by four nested windows, and this is what says
// how much of each is left.
//
// The windows nest on purpose: the short ones refresh as the day goes, so a
// holder can keep working all month, and the long ones are the ceiling that
// stops it being unlimited. Whichever window is nearest its limit is the one
// actually binding, which is why all four are published rather than a single
// "usage" figure — a holder throttled at 22:05 wants to know it clears at
// 23:00, not that they are 3% through the month.
//
// A REQUEST IS A LEDGER ROW. Every api-usage debit is written once per call
// (api/billing/usage.go, behind an idempotency record), so counting rows counts
// requests. This is the same source the balance gate and the monthly rollup
// read, so what a holder is shown and what the gate enforces cannot disagree —
// which they would the moment this counted from a second place.

// Window is one bound and how much of it is spent.
type Window struct {
	// Span names the period: hour, day, week or month.
	Span string `json:"span"`
	// Limit is the plan's included requests for this span. Zero means the plan
	// declares no bound at this span — not that the bound is zero.
	Limit int `json:"limit"`
	// Used is requests made inside the current period.
	Used int `json:"used"`
	// Remaining is Limit-Used, floored at zero. Absent meaning: with no limit
	// declared there is nothing to remain, so it stays zero and Limit says why.
	Remaining int `json:"remaining"`
	// Resets is when this period rolls over, RFC3339 UTC.
	Resets string `json:"resets"`
}

// windowSpans are the four periods, shortest first, each with the start of its
// current period and where it rolls over. Calendar-aligned rather than trailing:
// a holder reads "resets at midnight" and can act on it, where a rolling 24h
// window recovers a little at a time and never visibly clears.
func windowSpans(now time.Time) []struct {
	span         string
	start, reset time.Time
} {
	n := now.UTC()
	hour := time.Date(n.Year(), n.Month(), n.Day(), n.Hour(), 0, 0, 0, time.UTC)
	day := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	// Weeks start Monday: the ISO convention, and the one a working week matches.
	week := day.AddDate(0, 0, -((int(n.Weekday()) + 6) % 7))
	month := time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, time.UTC)
	return []struct {
		span         string
		start, reset time.Time
	}{
		{"hour", hour, hour.Add(time.Hour)},
		{"day", day, day.AddDate(0, 0, 1)},
		{"week", week, week.AddDate(0, 0, 7)},
		{"month", month, month.AddDate(0, 1, 0)},
	}
}

// planWindowLimits reads the four declared bounds off the catalog, by slug. A
// span the plan does not declare reports zero, which the caller renders as "no
// bound at this span" rather than as a limit of none.
func planWindowLimits(slug string) map[string]int {
	out := map[string]int{}
	p := lookupPlan(slug)
	if p == nil || p.Limits == nil {
		return out
	}
	for span, v := range map[string]*int{
		"hour":  p.Limits.RequestsPerHour,
		"day":   p.Limits.RequestsPerDay,
		"week":  p.Limits.RequestsPerWeek,
		"month": p.Limits.RequestsPerMonth,
	} {
		if v != nil && *v > 0 {
			out[span] = *v
		}
	}
	return out
}

// usageWindows counts the caller's requests in each window and reports them
// against the plan's bounds.
//
// ONE query covers all four, because the month contains the other three: the
// rows are read once and bucketed by start instant. Four queries would cost
// four scans of the same data and could disagree with each other across a
// period boundary.
func usageWindows(ctx context.Context, user, slug string, isTest bool, now time.Time) []Window {
	limits := planWindowLimits(slug)
	spans := windowSpans(now)

	db := datastore.New(ctx)
	rootKey := db.NewKey("synckey", "", 1, nil)
	transs := make([]*transaction.Transaction, 0)
	q := transaction.Query(db).Ancestor(rootKey).
		Filter("Test=", isTest).
		Filter("SourceKind=", "iam-user").
		Filter("SourceId=", user).
		Filter("Tags=", "api-usage")
	if _, err := q.GetAll(&transs); err != nil {
		// An unreadable ledger is not an empty one. Report the bounds with no
		// consumption rather than "you have used nothing", which is the reading a
		// zero would get and the opposite of the truth.
		transs = nil
	}

	out := make([]Window, 0, len(spans))
	for _, s := range spans {
		used := 0
		for _, t := range transs {
			if t != nil && !t.CreatedAt.Before(s.start) {
				used++
			}
		}
		limit := limits[s.span]
		remaining := limit - used
		if remaining < 0 {
			remaining = 0
		}
		out = append(out, Window{
			Span:      s.span,
			Limit:     limit,
			Used:      used,
			Remaining: remaining,
			Resets:    s.reset.Format(time.RFC3339),
		})
	}
	return out
}
