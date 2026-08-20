// Copyright © 2026 Hanzo AI. MIT License.

package billing

import (
	"testing"
	"time"
)

// The windows have to be CALENDAR-aligned and they have to nest, because both
// are what a holder is told: "resets at midnight" is actionable, and a rolling
// 24h window that recovers a little at a time never visibly clears.
func TestWindowSpansAreCalendarAlignedAndNested(t *testing.T) {
	// A Wednesday, mid-hour, mid-month — nothing lands on a boundary by luck.
	now := time.Date(2026, 8, 19, 14, 37, 42, 0, time.UTC)
	byspan := map[string]struct{ start, reset time.Time }{}
	for _, s := range windowSpans(now) {
		byspan[s.span] = struct{ start, reset time.Time }{s.start, s.reset}
	}

	for span, want := range map[string]time.Time{
		"hour":  time.Date(2026, 8, 19, 14, 0, 0, 0, time.UTC),
		"day":   time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
		"week":  time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC), // Monday
		"month": time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
	} {
		if got := byspan[span].start; !got.Equal(want) {
			t.Errorf("%s starts %s, want %s", span, got, want)
		}
	}

	// Each period must CONTAIN the one below it, or a request counted in the
	// hour could be missing from the day and the four numbers would describe
	// different worlds.
	order := []string{"hour", "day", "week", "month"}
	for i := 1; i < len(order); i++ {
		outer, inner := byspan[order[i]], byspan[order[i-1]]
		if outer.start.After(inner.start) {
			t.Errorf("%s starts after %s; the longer window must contain the shorter", order[i], order[i-1])
		}
	}

	// Every period must roll over in the future, or "resets" names a moment that
	// has already passed and the holder is told to wait for nothing.
	for span, w := range byspan {
		if !w.reset.After(now) {
			t.Errorf("%s resets at %s, which is not after %s", span, w.reset, now)
		}
	}
}

// A Monday is the week's first day, not its last. Sunday is the off-by-one this
// gets wrong: Go's Weekday() puts Sunday at 0, so the naive subtraction moves a
// Sunday back six days into the PREVIOUS week and reports a nearly-spent bound
// as fresh.
func TestTheWeekStartsOnMonday(t *testing.T) {
	for _, tc := range []struct{ day, wantStart int }{
		{17, 17}, // Monday   -> itself
		{19, 17}, // Wednesday
		{23, 17}, // Sunday   -> the Monday BEFORE it, not the one after
		{24, 24}, // Monday   -> itself again, a new week
	} {
		now := time.Date(2026, 8, tc.day, 12, 0, 0, 0, time.UTC)
		var start time.Time
		for _, s := range windowSpans(now) {
			if s.span == "week" {
				start = s.start
			}
		}
		if start.Day() != tc.wantStart {
			t.Errorf("%s: week starts on the %d, want the %d",
				now.Format("Mon Jan 2"), start.Day(), tc.wantStart)
		}
	}
}

// A span the plan does not declare reports limit 0 — and that has to read as
// "no bound here", never as "you may make zero requests". The two are opposite
// and a renderer can only tell them apart if the number is honest.
func TestAnUndeclaredSpanIsNotALimitOfZero(t *testing.T) {
	limits := planWindowLimits("no-such-plan")
	if len(limits) != 0 {
		t.Errorf("unknown plan declares %v; it declares nothing", limits)
	}
	// The published ladder does declare all four on its personal rungs, which is
	// what makes the zero above unambiguous when it appears.
	for _, slug := range []string{"free", "go", "dev", "pro", "max"} {
		l := planWindowLimits(slug)
		for _, span := range []string{"hour", "day", "week", "month"} {
			if l[span] <= 0 {
				t.Errorf("%s declares no %s bound; the ladder sells usage and must say how much", slug, span)
			}
		}
		if !(l["hour"] < l["day"] && l["day"] < l["week"] && l["week"] < l["month"]) {
			t.Errorf("%s windows do not ascend: %v", slug, l)
		}
	}
}
