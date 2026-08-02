// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/reserve"
	"github.com/hanzoai/commerce/models/screen"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// repeat_test.go is the gate on the defect that made an idempotency key a way
// to LIFT A LIVE CONTROL.
//
// The shape was: answer a repeat from the stored row BEFORE reading the
// controls. Screen once while the subject is clean, keep the key, replay it
// after the block lands — the block is in the store, in force, and the money
// moves anyway on the strength of an answer given before it existed. These
// tests hold the order that fixes it: controls, then idempotency.

// TestScreen_ARepeatCannotOutrunAControlPlacedAfterTheFirstAnswer is THE test.
// It fails against any implementation that returns the stored screen before
// applying the controls.
func TestScreen_ARepeatCannotOutrunAControlPlacedAfterTheFirstAnswer(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("replayblock", ctx, &oracle{answer: &Decision{ID: "d1", Action: Allow}})
	move := Move{
		Stage: Payout, Subject: Subject{Kind: KindMerchant, ID: "m1"},
		Amount: 5000, Currency: currency.USD, Out: true, Idem: "pay-1",
	}

	first, err := s.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if Refused(first) || first.Allowed != 5000 {
		t.Fatalf("first answer: action=%s allowed=%d, want an allow of 5000", first.Action, first.Allowed)
	}

	// The platform blocks the merchant AFTER the first answer.
	if _, err := Place(s, Placement{
		Subject: Subject{Kind: KindMerchant, ID: "m1"}, Effect: control.Block, Reason: "mule",
	}); err != nil {
		t.Fatalf("place: %v", err)
	}

	again, err := s.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("repeat: %v", err)
	}
	if !Refused(again) {
		t.Fatalf("a repeat under key %q lifted a live block: action=%s — the control was in the store and money moved anyway",
			move.Idem, again.Action)
	}
	if again.Allowed != 0 || again.Held != 5000 {
		t.Fatalf("a blocked repeat allowed=%d held=%d, want 0/5000", again.Allowed, again.Held)
	}
	if again.Id() != first.Id() {
		t.Fatalf("the repeat wrote a second record %s (first %s)", again.Id(), first.Id())
	}
	if again.Reasserts != 1 {
		t.Fatalf("reasserts=%d, want the row to record that it was re-judged", again.Reasserts)
	}
	if again.Detail["asFirstAnswered"] == nil {
		t.Fatal("the answer this row first gave was overwritten without a trace — a record that rewrites itself is not evidence")
	}
}

// TestScreen_ARepeatTightensToAReservePlacedAfterTheFirstAnswer — the same
// defect in its quiet form: not a block, a haircut. A repeat must not pay out
// the full amount an earlier answer allowed once a reserve is in force.
func TestScreen_ARepeatTightensToAReservePlacedAfterTheFirstAnswer(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("replayreserve", ctx, &oracle{answer: &Decision{Action: Allow}})
	move := Move{
		Stage: Payout, Subject: Subject{Kind: KindMerchant, ID: "m1"},
		Amount: 101, Currency: currency.USD, Out: true, Idem: "pay-2",
	}
	if _, err := s.Screen(context.Background(), move); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := Place(s, Placement{
		Subject: Subject{Kind: KindMerchant, ID: "m1"}, Effect: control.Reserve, Rate: 2500,
	}); err != nil {
		t.Fatalf("place: %v", err)
	}

	again, err := s.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("repeat: %v", err)
	}
	if again.Held != 26 || again.Allowed != 75 {
		t.Fatalf("the repeat withheld held=%d allowed=%d, want 26/75 — a reserve placed after the first answer still applies",
			again.Held, again.Allowed)
	}
	if again.Held+again.Allowed != 101 {
		t.Fatalf("the split lost a cent: %d + %d", again.Held, again.Allowed)
	}
}

// TestScreen_ARepeatNeverLoosens — the composition is one-way. A refusal that
// was recorded stays a refusal for that key even after the restraint is lifted:
// an idempotency key replays an ANSWER, and a fresh attempt at a move that was
// refused is a new decision that takes a new key.
func TestScreen_ARepeatNeverLoosens(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("replaysticky", ctx, &oracle{answer: &Decision{Action: Allow}})
	subject := Subject{Kind: KindMerchant, ID: "m1"}
	c, err := Place(s, Placement{Subject: subject, Effect: control.Hold, Reason: "review"})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	move := Move{Stage: Payout, Subject: subject, Amount: 900, Out: true, Idem: "pay-3"}
	first, err := s.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if !Refused(first) {
		t.Fatalf("the hold did not refuse: %s", first.Action)
	}

	if _, err := Lift(s, c.Id()); err != nil {
		t.Fatalf("lift: %v", err)
	}
	again, err := s.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("repeat: %v", err)
	}
	if !Refused(again) {
		t.Fatalf("a repeat loosened a recorded refusal: %s", again.Action)
	}

	// A NEW key is a new decision, and nothing restrains any more.
	fresh := move
	fresh.Idem = "pay-4"
	rec, err := s.Screen(context.Background(), fresh)
	if err != nil {
		t.Fatalf("fresh: %v", err)
	}
	if Refused(rec) {
		t.Fatalf("a released control still restrained a new decision: %s", rec.Action)
	}
}

// TestScreen_AKeyReusedForADifferentMoveIsRefused — the answer carries Allowed,
// and the payout boundary pays out exactly that. Without the request in the
// key, one $1 screen answers for a $1,000,000 payout.
func TestScreen_AKeyReusedForADifferentMoveIsRefused(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("idemconflict", ctx, &oracle{answer: &Decision{Action: Allow}})
	base := Move{
		Stage: Payout, Subject: Subject{Kind: KindMerchant, ID: "m1"},
		Amount: 100, Currency: currency.USD, Out: true, Reference: "po_1", Idem: "one-key",
	}
	if _, err := s.Screen(context.Background(), base); err != nil {
		t.Fatalf("first: %v", err)
	}

	for name, other := range map[string]Move{
		"a bigger amount":     with(base, func(m *Move) { m.Amount = 100000000 }),
		"another subject":     with(base, func(m *Move) { m.Subject = Subject{Kind: KindMerchant, ID: "m2"} }),
		"another currency":    with(base, func(m *Move) { m.Currency = currency.EUR }),
		"the other direction": with(base, func(m *Move) { m.Out = false }),
		"another payout":      with(base, func(m *Move) { m.Reference = "po_2" }),
		"another stage":       with(base, func(m *Move) { m.Stage = Payment }),
	} {
		if _, err := s.Screen(context.Background(), other); err == nil {
			t.Fatalf("%s under the same key was answered from the first move's record", name)
		}
	}

	// The same move under the same key is still one answer, not a conflict.
	if _, err := s.Screen(context.Background(), base); err != nil {
		t.Fatalf("the same move under the same key was refused: %v", err)
	}
}

// TestScreen_ARepeatWithNothingChangedCostsNoScoringHopAndNoSecondRow — the
// property the fix must not have cost: the guard still de-dups.
func TestScreen_ARepeatWithNothingChangedCostsNoScoringHopAndNoSecondRow(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	p := &oracle{answer: &Decision{ID: "d1", Action: Allow}}
	s := tenant("idemstable", ctx, p)
	move := Move{Stage: Payment, Subject: customer("c1"), Amount: 500, Idem: "k-1"}

	first, err := s.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	for i := 0; i < 4; i++ {
		again, err := s.Screen(context.Background(), move)
		if err != nil {
			t.Fatalf("repeat %d: %v", i, err)
		}
		if again.Id() != first.Id() {
			t.Fatalf("repeat %d wrote a second record", i)
		}
		if again.Reasserts != 0 {
			t.Fatalf("repeat %d re-asserted with nothing changed: %d", i, again.Reasserts)
		}
	}
	if got := p.count(); got != 1 {
		t.Fatalf("the plane was asked %d times for one idempotent move", got)
	}
	if n := len(screen.For(s.DB, KindCustomer, "c1", 0)); n != 1 {
		t.Fatalf("%d screens for one key, want 1", n)
	}
}

// TestScreen_ARepeatDoesNotWithholdTwice — a repeat moves no money, so the
// reserve ledger must not record a second hold for it. A ledger that
// double-posts under retry is worse than no ledger.
func TestScreen_ARepeatDoesNotWithholdTwice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("holdonce", ctx, &oracle{answer: &Decision{Action: Allow}})
	subject := Subject{Kind: KindMerchant, ID: "m1"}
	c, err := Place(s, Placement{Subject: subject, Effect: control.Reserve, Rate: 2500})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	move := Move{Stage: Payout, Subject: subject, Amount: 400, Currency: currency.USD, Out: true, Idem: "pay-5"}

	for i := 0; i < 3; i++ {
		if _, err := s.Screen(context.Background(), move); err != nil {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	rows := reserve.For(s.DB, subject.Kind, subject.ID, 0)
	if len(rows) != 1 {
		t.Fatalf("%d ledger entries for one move retried three times, want 1", len(rows))
	}
	if rows[0].Held != 100 {
		t.Fatalf("ledger held=%d, want the 100 the 25%% reserve took of 400", rows[0].Held)
	}
	fresh := control.New(s.DB)
	if err := fresh.GetById(c.Id()); err != nil {
		t.Fatalf("reload the control: %v", err)
	}
	if fresh.Held != 100 {
		t.Fatalf("the reserve's running total is %d after one move retried three times, want 100", fresh.Held)
	}
}

// TestScreen_TheRepeatPathIsTenantScoped — one key in two orgs is two moves,
// and org B's repeat is judged by org B's controls alone.
func TestScreen_TheRepeatPathIsTenantScoped(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	p := &oracle{answer: &Decision{Action: Allow}}
	a := tenant("repeatisoa", ctx, p)
	b := tenant("repeatisob", ctx, p)
	move := Move{
		Stage: Payout, Subject: Subject{Kind: KindMerchant, ID: "shared"},
		Amount: 1000, Currency: currency.USD, Out: true, Idem: "same-key",
	}

	if _, err := a.Screen(context.Background(), move); err != nil {
		t.Fatalf("a first: %v", err)
	}
	if _, err := b.Screen(context.Background(), move); err != nil {
		t.Fatalf("b first: %v", err)
	}
	if _, err := Place(a, Placement{
		Subject: Subject{Kind: KindMerchant, ID: "shared"}, Effect: control.Block,
	}); err != nil {
		t.Fatalf("a place: %v", err)
	}

	ra, err := a.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("a repeat: %v", err)
	}
	if !Refused(ra) {
		t.Fatalf("org A's own block did not tighten org A's repeat: %s", ra.Action)
	}
	rb, err := b.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("b repeat: %v", err)
	}
	if Refused(rb) {
		t.Fatalf("org A's block restrained org B's repeat under the same key: %s", rb.Action)
	}
	if len(reserve.For(b.DB, KindMerchant, "shared", 0)) != 0 {
		t.Fatal("org B read entries in org A's reserve ledger")
	}
}

// with copies a move and applies one change, so a table of near-identical moves
// reads as the one thing each row varies.
func with(m Move, change func(*Move)) Move {
	change(&m)
	return m
}

// TestReassert_IsPureAndOneWay exercises the composition directly, including
// the cases a store-backed test cannot reach cheaply.
func TestReassert_IsPureAndOneWay(t *testing.T) {
	blocked := Restrain([]*control.Control{ctl(control.Block, 0)}, 100, "", true, now)
	clear := Restrain(nil, 100, "", true, now)

	prior := &screen.Screen{Action: string(Allow), Amount: 100, Allowed: 100}
	if !reassert(prior, blocked) {
		t.Fatal("a live block did not tighten an allow")
	}
	if prior.Action != string(Block) || prior.Allowed != 0 || prior.Held != 100 {
		t.Fatalf("tightened to action=%s allowed=%d held=%d", prior.Action, prior.Allowed, prior.Held)
	}
	if reassert(prior, clear) {
		t.Fatal("an empty restraint loosened a recorded refusal")
	}
}
