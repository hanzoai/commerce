// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"math"
	"testing"
	"time"

	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/types/currency"
)

// ctl builds a control without a store — Restrain is pure, so its tests need no
// datastore and no clock but the one they pass.
func ctl(effect string, rate int64) *control.Control {
	return &control.Control{Effect: effect, Rate: rate}
}

func lapsed(effect string, rate int64, at time.Time) *control.Control {
	c := ctl(effect, rate)
	c.Until = at
	return c
}

func released(effect string, rate int64) *control.Control {
	c := ctl(effect, rate)
	c.Released = true
	return c
}

var now = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

// TestRestrain_HeldPlusAllowedIsExactlyTheAmount is the money invariant: the
// arithmetic neither creates nor loses a cent, at any rate, on any amount.
func TestRestrain_HeldPlusAllowedIsExactlyTheAmount(t *testing.T) {
	rates := []int64{1, 7, 250, 333, 2500, 3333, 5000, 6667, 9999}
	amounts := []currency.Cents{1, 2, 3, 7, 99, 100, 101, 4999, 100000, 123456789}

	for _, rate := range rates {
		for _, amount := range amounts {
			r := Restrain([]*control.Control{ctl(control.Reserve, rate)}, amount, true, now)
			if r.Held+r.Allowed != amount {
				t.Fatalf("rate=%d amount=%d: held=%d allowed=%d, sum=%d want %d",
					rate, amount, r.Held, r.Allowed, r.Held+r.Allowed, amount)
			}
			if r.Held < 0 || r.Allowed < 0 {
				t.Fatalf("rate=%d amount=%d: negative split held=%d allowed=%d", rate, amount, r.Held, r.Allowed)
			}
		}
	}
}

// TestRestrain_ReserveRoundsUp: a platform that declares a 25% reserve never
// holds back less than a quarter of a move that does not divide evenly.
func TestRestrain_ReserveRoundsUp(t *testing.T) {
	cases := []struct {
		amount currency.Cents
		rate   int64
		held   currency.Cents
	}{
		{100, 2500, 25},       // exact
		{101, 2500, 26},       // 25.25 -> 26
		{1, 1, 1},             // 0.0001 -> 1, never zero
		{3, 3333, 1},          // 0.99999 -> 1
		{10000, 10000, 10000}, // a full reserve withholds everything
		{0, 5000, 0},          // nothing to withhold
	}
	for _, c := range cases {
		r := Restrain([]*control.Control{ctl(control.Reserve, c.rate)}, c.amount, true, now)
		if r.Held != c.held {
			t.Fatalf("amount=%d rate=%d: held=%d want %d", c.amount, c.rate, r.Held, c.held)
		}
	}
}

// TestRestrain_StrictestNotSum: two reserves compose to the strictest, never
// their sum — summing would withhold more money than the move contains.
func TestRestrain_StrictestNotSum(t *testing.T) {
	r := Restrain([]*control.Control{ctl(control.Reserve, 6000), ctl(control.Reserve, 6000)}, 1000, true, now)
	if r.Held != 600 {
		t.Fatalf("two 60%% reserves: held=%d want 600 (the strictest, not the sum)", r.Held)
	}
	r = Restrain([]*control.Control{ctl(control.Reserve, 1000), ctl(control.Reserve, 7500)}, 1000, true, now)
	if r.Held != 750 {
		t.Fatalf("10%% and 75%%: held=%d want 750", r.Held)
	}
}

// TestRestrain_HoldStopsOnlyMoneyLeaving.
func TestRestrain_HoldStopsOnlyMoneyLeaving(t *testing.T) {
	out := Restrain([]*control.Control{ctl(control.Hold, 0)}, 500, true, now)
	if !out.Blocked || out.Allowed != 0 || out.Held != 500 {
		t.Fatalf("hold on an outbound move: %+v, want blocked with everything held", out)
	}
	in := Restrain([]*control.Control{ctl(control.Hold, 0)}, 500, false, now)
	if in.Blocked || in.Allowed != 500 {
		t.Fatalf("hold on an inbound move: %+v, want the charge to proceed", in)
	}
}

// TestRestrain_ReserveDoesNotTouchAnInboundCharge: withholding part of an
// arriving payment would mean refusing part of a customer's money.
func TestRestrain_ReserveDoesNotTouchAnInboundCharge(t *testing.T) {
	r := Restrain([]*control.Control{ctl(control.Reserve, 5000)}, 900, false, now)
	if r.Held != 0 || r.Allowed != 900 || r.Blocked {
		t.Fatalf("reserve on an inbound charge: %+v, want untouched", r)
	}
}

// TestRestrain_BlockStopsBothDirections.
func TestRestrain_BlockStopsBothDirections(t *testing.T) {
	for _, out := range []bool{true, false} {
		r := Restrain([]*control.Control{ctl(control.Block, 0)}, 42, out, now)
		if !r.Blocked || r.Allowed != 0 || r.Held != 42 {
			t.Fatalf("block out=%v: %+v, want blocked", out, r)
		}
	}
}

// TestRestrain_LapsedOrReleasedRestrainsNothing.
func TestRestrain_LapsedOrReleasedRestrainsNothing(t *testing.T) {
	for name, c := range map[string]*control.Control{
		"lapsed":   lapsed(control.Block, 0, now.Add(-time.Hour)),
		"released": released(control.Block, 0),
	} {
		r := Restrain([]*control.Control{c}, 100, true, now)
		if r.Blocked || r.Allowed != 100 {
			t.Fatalf("%s control still restrains: %+v", name, r)
		}
	}
	// A control that lapses in the future is still in force.
	r := Restrain([]*control.Control{lapsed(control.Block, 0, now.Add(time.Hour))}, 100, true, now)
	if !r.Blocked {
		t.Fatalf("a control lapsing in an hour must still bear on a move now: %+v", r)
	}
}

// TestRestrain_BlockBeatsReserve: the strictest applying control governs, and
// the order the controls arrive in cannot change the answer.
func TestRestrain_BlockBeatsReserve(t *testing.T) {
	a := Restrain([]*control.Control{ctl(control.Reserve, 1000), ctl(control.Block, 0)}, 800, true, now)
	b := Restrain([]*control.Control{ctl(control.Block, 0), ctl(control.Reserve, 1000)}, 800, true, now)
	if !a.Blocked || !b.Blocked || a.Held != b.Held || a.Allowed != b.Allowed {
		t.Fatalf("order changed the answer: %+v vs %+v", a, b)
	}
}

// TestRestrain_RefusesAnAmountTooLargeToMultiplyExactly: the arithmetic never
// wraps silently on a money path.
func TestRestrain_RefusesAnAmountTooLargeToMultiplyExactly(t *testing.T) {
	huge := currency.Cents(math.MaxInt64/control.FullRate + 1)
	r := Restrain([]*control.Control{ctl(control.Reserve, 5000)}, huge, true, now)
	if r.Allowed != 0 || r.Held != huge {
		t.Fatalf("an amount past the exact-multiply bound must withhold everything, got %+v", r)
	}
	if r.Held+r.Allowed != huge {
		t.Fatalf("the invariant broke at the bound: held=%d allowed=%d", r.Held, r.Allowed)
	}
}

// TestRestrain_NegativeAmountIsRefused: a negative amount is a move in the
// other direction wearing the wrong sign.
func TestRestrain_NegativeAmountIsRefused(t *testing.T) {
	r := Restrain(nil, -1, true, now)
	if !r.Blocked || r.Allowed != 0 {
		t.Fatalf("negative amount: %+v, want refused", r)
	}
}

// TestRestrain_NoControlsAllowsEverything.
func TestRestrain_NoControlsAllowsEverything(t *testing.T) {
	r := Restrain(nil, 1234, true, now)
	if r.Blocked || r.Held != 0 || r.Allowed != 1234 {
		t.Fatalf("no controls: %+v, want the whole move allowed", r)
	}
	// A nil entry in the slice must not panic or restrain.
	r = Restrain([]*control.Control{nil}, 1234, true, now)
	if r.Blocked || r.Allowed != 1234 {
		t.Fatalf("nil control: %+v", r)
	}
}

// TestStrictest_NeverLoosens, including an action this plane has not learned:
// a vocabulary the scoring plane grew must fail closed, not fall through.
func TestStrictest_NeverLoosens(t *testing.T) {
	order := []Action{Allow, Challenge, Review, Restrict, Block}
	for i, a := range order {
		for j, b := range order {
			want := order[max(i, j)]
			if got := Strictest(a, b); got != want {
				t.Fatalf("Strictest(%s,%s)=%s want %s", a, b, got, want)
			}
		}
	}
	if got := Strictest(Allow, Action("some-new-verdict")); got.Moves() {
		t.Fatalf("an unknown action must not let money move, got %s", got)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
