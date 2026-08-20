// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/screen"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// idem_test.go is the regression suite for the worst defect this face has had:
// the idempotency lookup ran BEFORE the controls, so replaying a key returned
// the cached answer and a block, hold or reserve placed since was never
// applied. A retry was a documented, one-header way to lift a live money
// control.
//
// Every test here fails if the lookup moves back above control.LiveFor.

// TestReplay_CannotLiftABlockPlacedAfterTheFirstAnswer is the headline case.
func TestReplay_CannotLiftABlockPlacedAfterTheFirstAnswer(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("replayblock", ctx, &oracle{answer: &Decision{ID: "d1", Action: Allow}})
	move := Move{Stage: Payout, Subject: Subject{Kind: KindMerchant, ID: "m1"},
		Amount: 5000, Currency: currency.USD, Out: true, Idem: "k-1"}

	first, err := s.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("first screen: %v", err)
	}
	if Refused(first) {
		t.Fatalf("the first answer was already refused: %s", first.Action)
	}

	// The platform blocks the merchant between the two attempts.
	if _, err := Place(s, Subject{Kind: KindMerchant, ID: "m1"}, control.Block, 0, 0, time.Time{}, "confirmed fraud"); err != nil {
		t.Fatalf("place: %v", err)
	}

	again, err := s.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !Refused(again) {
		t.Fatalf("A REPLAY LIFTED A LIVE BLOCK: action=%s held=%d allowed=%d", again.Action, again.Held, again.Allowed)
	}
	if again.Allowed != 0 || again.Held != 5000 {
		t.Fatalf("a refused move must let nothing through: held=%d allowed=%d", again.Held, again.Allowed)
	}
	if again.Reasserted != 1 {
		t.Fatalf("the re-assertion was not recorded: reasserted=%d", again.Reasserted)
	}
	// One key, one row: the re-assertion tightened the answer in place.
	if n := len(screen.For(s.DB, "", "", 0)); n != 1 {
		t.Fatalf("%d screens for one key, want 1", n)
	}
}

// TestReplay_CannotLiftAReservePlacedAfterTheFirstAnswer — the same defect in
// its quieter form: not "money moved that must not" but "money moved that was
// supposed to be withheld".
func TestReplay_CannotLiftAReservePlacedAfterTheFirstAnswer(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("replayreserve", ctx, &oracle{answer: &Decision{ID: "d1", Action: Allow}})
	move := Move{Stage: Payout, Subject: Subject{Kind: KindMerchant, ID: "m1"},
		Amount: 101, Currency: currency.USD, Out: true, Idem: "k-2"}

	first, err := s.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("first screen: %v", err)
	}
	if first.Held != 0 || first.Allowed != 101 {
		t.Fatalf("nothing was in force yet: held=%d allowed=%d", first.Held, first.Allowed)
	}

	if _, err := Place(s, Subject{Kind: KindMerchant, ID: "m1"}, control.Reserve, 2500, 1_000_000, time.Time{}, "new merchant"); err != nil {
		t.Fatalf("place: %v", err)
	}

	again, err := s.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if again.Held != 26 || again.Allowed != 75 {
		t.Fatalf("A REPLAY LIFTED A LIVE RESERVE: held=%d allowed=%d want 26/75", again.Held, again.Allowed)
	}
	if again.Held+again.Allowed != 101 {
		t.Fatalf("the split lost a cent on re-assertion: %d + %d", again.Held, again.Allowed)
	}
}

// TestReplay_DoesNotLoosenWhenTheControlIsLIFTED is the other direction, and it
// is the half that makes this idempotency and not merely a re-screen: the
// answer to one question does not change under the caller.
func TestReplay_DoesNotLoosenWhenTheControlIsLifted(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("replaylift", ctx, &oracle{answer: &Decision{ID: "d1", Action: Allow}})
	c, err := Place(s, Subject{Kind: KindMerchant, ID: "m1"}, control.Block, 0, 0, time.Time{}, "suspected")
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	move := Move{Stage: Payout, Subject: Subject{Kind: KindMerchant, ID: "m1"},
		Amount: 900, Currency: currency.USD, Out: true, Idem: "k-3"}

	first, err := s.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("first screen: %v", err)
	}
	if !Refused(first) {
		t.Fatalf("the block did not refuse: %s", first.Action)
	}

	if _, err := Release(s, c); err != nil {
		t.Fatalf("release: %v", err)
	}
	again, err := s.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !Refused(again) {
		t.Fatalf("the replay of a refused move came back permitted: %s", again.Action)
	}

	// A NEW key is a new question, and the lifted block does not restrain it.
	fresh := move
	fresh.Idem = "k-4"
	rec, err := s.Screen(context.Background(), fresh)
	if err != nil {
		t.Fatalf("fresh screen: %v", err)
	}
	if Refused(rec) {
		t.Fatalf("a released control still refuses a NEW move: %s", rec.Action)
	}
}

// TestReplay_AKeyNamesExactlyOneMove — reusing a key for a different move is
// the swap that turns idempotency into an exploit: screen one cent, spend the
// verdict on ten thousand dollars.
func TestReplay_AKeyNamesExactlyOneMove(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("replayswap", ctx, &oracle{answer: &Decision{ID: "d1", Action: Allow}})
	cheap := Move{Stage: Payout, Subject: Subject{Kind: KindMerchant, ID: "m1"},
		Amount: 1, Currency: currency.USD, Out: true, Idem: "k-5"}
	if _, err := s.Screen(context.Background(), cheap); err != nil {
		t.Fatalf("first screen: %v", err)
	}

	for _, swapped := range []Move{
		{Stage: Payout, Subject: Subject{Kind: KindMerchant, ID: "m1"}, Amount: 1_000_000, Currency: currency.USD, Out: true, Idem: "k-5"},
		{Stage: Payout, Subject: Subject{Kind: KindMerchant, ID: "m2"}, Amount: 1, Currency: currency.USD, Out: true, Idem: "k-5"},
		{Stage: Payment, Subject: Subject{Kind: KindMerchant, ID: "m1"}, Amount: 1, Currency: currency.USD, Out: true, Idem: "k-5"},
		{Stage: Payout, Subject: Subject{Kind: KindMerchant, ID: "m1"}, Amount: 1, Currency: currency.USD, Out: false, Idem: "k-5"},
		{Stage: Payout, Subject: Subject{Kind: KindMerchant, ID: "m1"}, Amount: 1, Currency: currency.USD, Out: true, Idem: "k-5",
			Signals: map[string]string{"ip": "203.0.113.9"}},
	} {
		got, err := s.Screen(context.Background(), swapped)
		if !errors.Is(err, ErrReused) {
			t.Fatalf("a key was reused for a different move and answered %+v (err=%v)", got, err)
		}
	}
	if n := len(screen.For(s.DB, "", "", 0)); n != 1 {
		t.Fatalf("%d screens written, want 1 — a refused swap must write nothing", n)
	}
}

// TestReplay_AnUnchangedPostureWritesNothing — a retry storm against a stable
// posture must be pure reads, or the fix for the replay hole becomes a write
// amplifier a caller controls.
func TestReplay_AnUnchangedPostureWritesNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("replaystable", ctx, &oracle{answer: &Decision{ID: "d1", Action: Allow}})
	move := Move{Stage: Payment, Subject: customer("c1"), Amount: 500, Currency: currency.USD, Idem: "k-6"}

	for i := 0; i < 25; i++ {
		rec, err := s.Screen(context.Background(), move)
		if err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}
		if rec.Reasserted != 0 {
			t.Fatalf("attempt %d rewrote a row nothing had changed: reasserted=%d", i, rec.Reasserted)
		}
	}
	if n := len(screen.For(s.DB, "", "", 0)); n != 1 {
		t.Fatalf("%d screens for 25 identical attempts, want 1", n)
	}
	// And the plane was asked exactly once: idempotency still means one score.
	if p := s.Plane.(*oracle); p.count() != 1 {
		t.Fatalf("the scoring plane was asked %d times for one key", p.count())
	}
}

// TestReplay_TheProvenanceOfTheFirstJudgementSurvives — a re-assertion changes
// the ENFORCEMENT, never the evidence. The score and the decision id are what a
// dispute is defended with, and they did not happen twice.
func TestReplay_TheProvenanceOfTheFirstJudgementSurvives(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("replayprov", ctx, &oracle{answer: &Decision{ID: "d-first", Action: Allow, Score: 0.25}})
	move := Move{Stage: Payout, Subject: Subject{Kind: KindMerchant, ID: "m1"},
		Amount: 400, Currency: currency.USD, Out: true, Idem: "k-7"}
	if _, err := s.Screen(context.Background(), move); err != nil {
		t.Fatalf("first screen: %v", err)
	}
	if _, err := Place(s, Subject{Kind: KindMerchant, ID: "m1"}, control.Hold, 0, 0, time.Time{}, "held"); err != nil {
		t.Fatalf("place: %v", err)
	}

	again, err := s.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !Refused(again) {
		t.Fatalf("the hold was not re-asserted: %s", again.Action)
	}
	if again.Decision != "d-first" || again.Score != 0.25 {
		t.Fatalf("the original judgement was overwritten: decision=%q score=%v", again.Decision, again.Score)
	}
	// The plane was NOT asked again — a replay re-applies controls, it does not
	// re-score.
	if p := s.Plane.(*oracle); p.count() != 1 {
		t.Fatalf("the scoring plane was asked %d times, want 1", p.count())
	}
}
