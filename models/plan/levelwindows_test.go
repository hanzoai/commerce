// Copyright © 2026 Hanzo AI. MIT License.

package plan

import (
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
)

func maxPlan() *Plan {
	n := func(v int) *int { return &v }
	return &Plan{
		Slug:  "max",
		Price: 9900, // $99 — the base, and the denominator
		Prices: []currency.Cents{
			9900, 19900, 29900, 39900, 49900, 59900, 69900, 79900, 89900, 99900, // $99 … $999
		},
		Limits: &Limits{
			RequestsPerMinute: n(5000),
			RequestsPerHour:   n(2500),
			RequestsPerDay:    n(15000),
			RequestsPerWeek:   n(75000),
			RequestsPerMonth:  n(250000),
		},
	}
}

// A ladder sells ten levels and carries ONE set of windows. What each level
// includes is derived from what it costs, so the prices and the allowance cannot
// disagree and a new level needs no second edit.
func TestUsageScalesWithWhatYouPay(t *testing.T) {
	p := maxPlan()

	base, err := p.LevelWindows(0)
	if err != nil {
		t.Fatalf("level 0: %v", err)
	}
	if *base.RequestsPerDay != 15000 {
		t.Fatalf("level 0 day = %d, want the base 15000", *base.RequestsPerDay)
	}

	// $999 is 999/99 of $99, so it includes 10.09x the volume.
	top, err := p.LevelWindows(9)
	if err != nil {
		t.Fatalf("level 9: %v", err)
	}
	if want := 15000 * 99900 / 9900; *top.RequestsPerDay != want {
		t.Errorf("level 9 day = %d, want %d", *top.RequestsPerDay, want)
	}
	if *top.RequestsPerDay <= *base.RequestsPerDay {
		t.Error("the $999 level includes no more than the $99 level")
	}

	// Monotonic: paying more never includes less, at any step of the ladder.
	prev := 0
	for level := range p.Prices {
		w, err := p.LevelWindows(level)
		if err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
		if *w.RequestsPerMonth <= prev {
			t.Errorf("level %d month = %d, not more than level %d's %d",
				level, *w.RequestsPerMonth, level-1, prev)
		}
		prev = *w.RequestsPerMonth
	}
}

// A burst rate is what the service absorbs at once. Paying more does not make a
// spike cheaper to serve, and scaling it would turn a safety limit into a quota.
func TestTheBurstRateDoesNotScale(t *testing.T) {
	p := maxPlan()
	top, err := p.LevelWindows(9)
	if err != nil {
		t.Fatalf("level 9: %v", err)
	}
	if *top.RequestsPerMinute != 5000 {
		t.Errorf("per-minute = %d at the top level, want the base 5000 — a rate is not a quota",
			*top.RequestsPerMinute)
	}
}

// The base Limits is shared by every caller asking about any level. Returning it
// directly would let one question about $999 rewrite what $99 includes.
func TestAskingAboutOneLevelDoesNotChangeAnother(t *testing.T) {
	p := maxPlan()
	if _, err := p.LevelWindows(9); err != nil {
		t.Fatalf("level 9: %v", err)
	}
	if *p.Limits.RequestsPerDay != 15000 {
		t.Fatalf("the plan's own base was mutated to %d", *p.Limits.RequestsPerDay)
	}
	base, _ := p.LevelWindows(0)
	if *base.RequestsPerDay != 15000 {
		t.Errorf("level 0 now reads %d — an earlier question changed it", *base.RequestsPerDay)
	}
}

// A level nobody sells has no allowance to report. Answering the base instead
// would hand an unsold level the entry tier's usage.
func TestALevelThatIsNotSoldHasNoWindows(t *testing.T) {
	p := maxPlan()
	for _, level := range []int{-1, 10, 99} {
		if _, err := p.LevelWindows(level); err == nil {
			t.Errorf("level %d answered; it is not on the ladder", level)
		}
	}
}

// A plan with no ladder still sells at its own price, so every level it does
// answer includes exactly its declared windows — no scaling, no division by zero.
func TestAPlanWithNoLadderIncludesItsOwnWindows(t *testing.T) {
	n := func(v int) *int { return &v }
	p := &Plan{Slug: "go", Price: 900, Limits: &Limits{RequestsPerDay: n(1000)}}
	w, err := p.LevelWindows(0)
	if err != nil {
		t.Fatalf("level 0: %v", err)
	}
	if *w.RequestsPerDay != 1000 {
		t.Errorf("day = %d, want 1000", *w.RequestsPerDay)
	}

	free := &Plan{Slug: "free", Price: 0, Limits: &Limits{RequestsPerDay: n(20)}}
	w, err = free.LevelWindows(0)
	if err != nil {
		t.Fatalf("free level 0: %v", err)
	}
	if *w.RequestsPerDay != 20 {
		t.Errorf("free day = %d, want 20 — a zero price is not a zero allowance", *w.RequestsPerDay)
	}
}
