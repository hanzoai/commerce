// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/outcome"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

func merchant(id string) Subject { return Subject{Kind: KindMerchant, ID: id} }

// seedOutcome writes one observed fact about the merchant.
func seedOutcome(s *Screener, subject Subject, event string) {
	o := outcome.New(s.DB)
	o.Event = event
	o.SubjectKind = subject.Kind
	o.Subject = subject.ID
	o.MustCreate()
}

// TestCount_RatesAreExactBasisPoints — a dispute rate gets compared against a
// threshold and quoted in an appeal, so it is integer arithmetic end to end.
func TestCount_RatesAreExactBasisPoints(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("count", ctx, &oracle{answer: &Decision{Action: Allow}})
	m := merchant("m1")

	// Three screens: two allowed inbound, one blocked outbound.
	for i := 0; i < 2; i++ {
		if _, err := s.Screen(context.Background(), Move{
			Stage: Payment, Subject: m, Amount: 1000, Currency: currency.USD,
		}); err != nil {
			t.Fatalf("screen: %v", err)
		}
	}
	s.Plane = &oracle{answer: &Decision{Action: Block}}
	if _, err := s.Screen(context.Background(), Move{Stage: Payout, Subject: m, Amount: 500, Out: true}); err != nil {
		t.Fatalf("screen: %v", err)
	}
	seedOutcome(s, m, outcome.Dispute)

	st, err := Count(s, m)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if st.Screens != 3 || st.Refused != 1 || st.Disputes != 1 {
		t.Fatalf("counts: screens=%d refused=%d disputes=%d", st.Screens, st.Refused, st.Disputes)
	}
	// 1 of 3 = 3333 basis points, floor — an exact integer, not 0.3333...
	if st.DisputeRate != 3333 || st.RefusalRate != 3333 {
		t.Fatalf("rates: dispute=%d refusal=%d want 3333/3333", st.DisputeRate, st.RefusalRate)
	}
	// Volume counts only what was ALLOWED to move, in the direction it moved.
	if st.VolumeIn != 2000 || st.VolumeOut != 0 {
		t.Fatalf("volume: in=%d out=%d want 2000/0", st.VolumeIn, st.VolumeOut)
	}
	if st.Held != 500 {
		t.Fatalf("held=%d want 500 — the blocked payout withheld everything", st.Held)
	}
}

// TestMonitor_PlacesTheControlTheAnswerImplies.
func TestMonitor_PlacesTheControlTheAnswerImplies(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	cases := []struct {
		answer  Action
		reserve int64
		effect  string
		rate    int64
	}{
		{Block, 0, control.Block, 0},
		{Restrict, 0, control.Hold, 0},
		{Restrict, 2500, control.Reserve, 2500},
		{Review, 2500, "", 0},
		{Allow, 2500, "", 0},
	}
	for i, c := range cases {
		s := tenant("monitor", ctx, &oracle{answer: &Decision{Action: c.answer}})
		m := merchant(string(rune('a' + i)))

		st, err := Monitor(context.Background(), s, m, c.reserve, true)
		if err != nil {
			t.Fatalf("%s: %v", c.answer, err)
		}
		if c.effect == "" {
			if st.Placed != "" {
				t.Fatalf("%s placed a control %s", c.answer, st.Placed)
			}
			continue
		}
		if st.Placed == "" {
			t.Fatalf("%s placed nothing, want a %s", c.answer, c.effect)
		}
		live, err := control.LiveFor(s.DB, m.Kind, m.ID, s.now())
		if err != nil || len(live) != 1 {
			t.Fatalf("%s: %d controls in force (err=%v)", c.answer, len(live), err)
		}
		if live[0].Effect != c.effect || live[0].Rate != c.rate {
			t.Fatalf("%s placed effect=%s rate=%d want %s/%d", c.answer, live[0].Effect, live[0].Rate, c.effect, c.rate)
		}
	}
}

// TestMonitor_DoesNotActUnlessAsked — a review that silently restrains money is
// a review nobody can run.
func TestMonitor_DoesNotActUnlessAsked(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("noact", ctx, &oracle{answer: &Decision{Action: Block}})
	m := merchant("m1")
	st, err := Monitor(context.Background(), s, m, 0, false)
	if err != nil {
		t.Fatalf("monitor: %v", err)
	}
	if st.Placed != "" {
		t.Fatalf("a read-only review placed control %s", st.Placed)
	}
	if got := control.All(s.DB); len(got) != 0 {
		t.Fatalf("%d controls were written by a review that was not asked to act", len(got))
	}
	if st.Screen == nil || Action(st.Screen.Action) != Block {
		t.Fatalf("the judgement was not recorded: %+v", st.Screen)
	}
}

// TestMonitor_RepeatedCyclesPlaceOneControl — the whole point of a continuous
// monitor is that it runs every cycle.
func TestMonitor_RepeatedCyclesPlaceOneControl(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("cycles", ctx, &oracle{answer: &Decision{Action: Restrict}})
	m := merchant("m1")
	for i := 0; i < 4; i++ {
		if _, err := Monitor(context.Background(), s, m, 0, true); err != nil {
			t.Fatalf("cycle %d: %v", i, err)
		}
	}
	if got := control.All(s.DB); len(got) != 1 {
		t.Fatalf("%d controls after 4 cycles, want 1", len(got))
	}
}

// TestCount_IsTenantScoped — one merchant id in two orgs is two merchants.
func TestCount_IsTenantScoped(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	a := tenant("counta", ctx, &oracle{answer: &Decision{Action: Allow}})
	b := tenant("countb", ctx, &oracle{answer: &Decision{Action: Allow}})
	m := merchant("shared")

	for i := 0; i < 3; i++ {
		if _, err := a.Screen(context.Background(), Move{Stage: Payment, Subject: m, Amount: 100}); err != nil {
			t.Fatalf("a screen: %v", err)
		}
	}
	seedOutcome(a, m, outcome.Dispute)

	st, err := Count(b, m)
	if err != nil {
		t.Fatalf("b count: %v", err)
	}
	if st.Screens != 0 || st.Disputes != 0 || st.VolumeIn != 0 {
		t.Fatalf("org B counted org A's merchant: %+v", st)
	}
}
