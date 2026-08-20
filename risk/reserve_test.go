// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/reserve"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// reserve_test.go pins the accounting a reserve did not have: a CEILING, a
// LEDGER, and cumulative totals. Before this, a reserve was a percentage
// haircut off a caller-chosen number — nothing recorded that money was held,
// nothing bounded how much could accumulate, and nothing ever gave it back.

func merchantOf(id string) Subject { return Subject{Kind: KindMerchant, ID: id} }

// TestReserve_TheCeilingBoundsTheTotalEverWithheld — the ceiling is measured
// against the LEDGER, not against one move, which is the whole point: a
// per-move bound bounds nothing when there are a thousand moves.
func TestReserve_TheCeilingBoundsTheTotalEverWithheld(t *testing.T) {
	cases := []struct {
		held, amount, want currency.Cents
	}{
		{held: 0, amount: 1000, want: 250},   // 25% of 1000, nothing held yet
		{held: 900, amount: 1000, want: 100}, // only 100 of room is left
		{held: 1000, amount: 1000, want: 0},  // the reserve is full
		{held: 1200, amount: 1000, want: 0},  // and cannot go past full
	}
	for _, c := range cases {
		r := Restrain([]*control.Control{capped(2500, 1000)}, c.amount, true, now, c.held)
		if r.Held != c.want {
			t.Fatalf("held=%d with %d already held, want %d", r.Held, c.held, c.want)
		}
		if r.Held+r.Allowed != c.amount {
			t.Fatalf("the split lost a cent: %d + %d != %d", r.Held, r.Allowed, c.amount)
		}
	}
}

// TestReserve_AFullReserveStopsWithholdingAndDoesNotStopTheMoney — when the
// declared total has been taken, it is the RESERVE that is finished, not the
// merchant.
func TestReserve_AFullReserveStopsWithholdingAndDoesNotStopTheMoney(t *testing.T) {
	r := Restrain([]*control.Control{capped(5000, 500)}, 10_000, true, now, 500)
	if r.Blocked {
		t.Fatalf("a full reserve blocked the move: %+v", r)
	}
	if r.Held != 0 || r.Allowed != 10_000 {
		t.Fatalf("held=%d allowed=%d, want the whole move to leave", r.Held, r.Allowed)
	}
	if r.Room != 0 {
		t.Fatalf("room=%d, want 0 so a reader can tell full from absent", r.Room)
	}
}

// TestPlace_AReserveMustStateACeiling — a rate with no total is an open-ended
// seizure and is refused at the boundary.
func TestPlace_AReserveMustStateACeiling(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("nocap", ctx, &oracle{answer: &Decision{Action: Allow}})
	for _, ceiling := range []int64{0, -1} {
		if _, err := Place(s, merchantOf("m1"), control.Reserve, 2500, ceiling, time.Time{}, ""); !errors.Is(err, errCap) {
			t.Fatalf("a reserve with cap=%d was accepted (err=%v)", ceiling, err)
		}
	}
	if _, err := Place(s, merchantOf("m1"), control.Reserve, 2500, 1000, time.Time{}, ""); err != nil {
		t.Fatalf("a reserve with a ceiling was refused: %v", err)
	}
}

// TestReserve_ScreeningWithholdsNothing — a judgement is a QUESTION. A merchant
// that asks it a thousand times must not thereby build its own reserve.
func TestReserve_ScreeningWithholdsNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("askonly", ctx, &oracle{answer: &Decision{ID: "d1", Action: Allow}})
	if _, err := Place(s, merchantOf("m1"), control.Reserve, 2500, 1_000_000, time.Time{}, "new"); err != nil {
		t.Fatalf("place: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, err := s.Screen(context.Background(), Move{
			Stage: Payout, Subject: merchantOf("m1"), Amount: 1000, Currency: currency.USD, Out: true,
		}); err != nil {
			t.Fatalf("screen %d: %v", i, err)
		}
	}
	if held := reserve.Held(s.DB, KindMerchant, "m1", currency.USD); held != 0 {
		t.Fatalf("asking ten questions withheld %d", held)
	}
}

// TestReserve_HoldPostsOncePerScreen — the ledger is idempotent per move, so a
// retried payout (which replays the SAME screen row) withholds the share once.
func TestReserve_HoldPostsOncePerScreen(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("holdonce", ctx, &oracle{answer: &Decision{ID: "d1", Action: Allow}})
	if _, err := Place(s, merchantOf("m1"), control.Reserve, 2500, 1_000_000, time.Time{}, "new"); err != nil {
		t.Fatalf("place: %v", err)
	}
	rec, err := s.Screen(context.Background(), Move{
		Stage: Payout, Subject: merchantOf("m1"), Amount: 1000, Currency: currency.USD, Out: true, Idem: "k-1",
	})
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	if rec.Held != 250 {
		t.Fatalf("held=%d want 250", rec.Held)
	}

	for i := 0; i < 3; i++ {
		if _, err := s.Hold(rec, "payout po_1"); err != nil {
			t.Fatalf("hold %d: %v", i, err)
		}
	}
	if held := reserve.Held(s.DB, KindMerchant, "m1", currency.USD); held != 250 {
		t.Fatalf("the ledger holds %d after three posts of one screen, want 250", held)
	}
	if n := len(reserve.Entries(s.DB, KindMerchant, "m1", 0)); n != 1 {
		t.Fatalf("%d ledger entries for one screen, want 1", n)
	}
}

// TestReserve_AccumulatesUntilTheCeilingThenStops — the cumulative accounting
// the haircut never had.
func TestReserve_AccumulatesUntilTheCeilingThenStops(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("accumulate", ctx, &oracle{answer: &Decision{ID: "d1", Action: Allow}})
	if _, err := Place(s, merchantOf("m1"), control.Reserve, 5000, 700, time.Time{}, "new"); err != nil {
		t.Fatalf("place: %v", err)
	}

	want := []int64{500, 200, 0} // 50% of 1000, then only the 200 of room left, then nothing
	for i, expect := range want {
		rec, err := s.Screen(context.Background(), Move{
			Stage: Payout, Subject: merchantOf("m1"), Amount: 1000, Currency: currency.USD, Out: true,
			Idem: string(rune('a' + i)),
		})
		if err != nil {
			t.Fatalf("payout %d: %v", i, err)
		}
		if rec.Held != expect {
			t.Fatalf("payout %d withheld %d, want %d", i, rec.Held, expect)
		}
		if _, err := s.Hold(rec, "payout"); err != nil {
			t.Fatalf("hold %d: %v", i, err)
		}
	}
	if held := reserve.Held(s.DB, KindMerchant, "m1", currency.USD); held != 700 {
		t.Fatalf("the ledger holds %d, want exactly the declared ceiling 700", held)
	}
}

// TestReserve_ReleasingTheControlFreesThePool — a release that keeps the money
// it withheld is not a release.
func TestReserve_ReleasingTheControlFreesThePool(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("freepool", ctx, &oracle{answer: &Decision{ID: "d1", Action: Allow}})
	c, err := Place(s, merchantOf("m1"), control.Reserve, 5000, 10_000, time.Time{}, "new")
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	rec, err := s.Screen(context.Background(), Move{
		Stage: Payout, Subject: merchantOf("m1"), Amount: 1000, Currency: currency.USD, Out: true, Idem: "k-1",
	})
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	if _, err := s.Hold(rec, "payout"); err != nil {
		t.Fatalf("hold: %v", err)
	}
	if held := reserve.Held(s.DB, KindMerchant, "m1", currency.USD); held != 500 {
		t.Fatalf("held=%d want 500 before the release", held)
	}

	if _, err := Release(s, c); err != nil {
		t.Fatalf("release: %v", err)
	}
	if held := reserve.Held(s.DB, KindMerchant, "m1", currency.USD); held != 0 {
		t.Fatalf("the release kept %d of the merchant's money", held)
	}
	// And it is RECORDED as a movement, not silently zeroed.
	entries := reserve.Entries(s.DB, KindMerchant, "m1", 0)
	if len(entries) != 2 {
		t.Fatalf("%d ledger entries, want a hold and a release", len(entries))
	}
	var sum int64
	for _, e := range entries {
		sum += e.Amount
	}
	if sum != 0 {
		t.Fatalf("the ledger does not net to zero after a full release: %d", sum)
	}
}

// TestReserve_AStrongerReserveKeepsThePoolWhenAWeakerOneIsReleased — the pool
// is per SUBJECT and the strictest reserve sets the rate, so releasing one of
// two must not free money the other still requires.
func TestReserve_AStrongerReserveKeepsThePoolWhenAWeakerOneIsReleased(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("twoholds", ctx, &oracle{answer: &Decision{ID: "d1", Action: Allow}})
	weak, err := Place(s, merchantOf("m1"), control.Reserve, 1000, 10_000, time.Time{}, "weak")
	if err != nil {
		t.Fatalf("place weak: %v", err)
	}
	if _, err := Place(s, merchantOf("m1"), control.Reserve, 5000, 10_000, time.Time{}, "strong"); err != nil {
		t.Fatalf("place strong: %v", err)
	}
	rec, err := s.Screen(context.Background(), Move{
		Stage: Payout, Subject: merchantOf("m1"), Amount: 1000, Currency: currency.USD, Out: true, Idem: "k-1",
	})
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	if rec.Held != 500 {
		t.Fatalf("held=%d, want the STRICTEST reserve's 500", rec.Held)
	}
	if _, err := s.Hold(rec, "payout"); err != nil {
		t.Fatalf("hold: %v", err)
	}

	if _, err := Release(s, weak); err != nil {
		t.Fatalf("release weak: %v", err)
	}
	if held := reserve.Held(s.DB, KindMerchant, "m1", currency.USD); held != 500 {
		t.Fatalf("releasing the weaker reserve freed the pool the stronger one holds: held=%d", held)
	}
}

// TestReserve_ALapsedReserveGivesTheMoneyBack — the release is lazy, evaluated
// the next time the subject moves money out, so no cron has to be running for
// the accounting to be right.
func TestReserve_ALapsedReserveGivesTheMoneyBack(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	at := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	s := tenant("lapse", ctx, &oracle{answer: &Decision{ID: "d1", Action: Allow}})
	s.Now = func() time.Time { return at }

	if _, err := Place(s, merchantOf("m1"), control.Reserve, 5000, 10_000, at.Add(time.Hour), "for an hour"); err != nil {
		t.Fatalf("place: %v", err)
	}
	rec, err := s.Screen(context.Background(), Move{
		Stage: Payout, Subject: merchantOf("m1"), Amount: 1000, Currency: currency.USD, Out: true, Idem: "k-1",
	})
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	if _, err := s.Hold(rec, "payout"); err != nil {
		t.Fatalf("hold: %v", err)
	}
	if held := reserve.Held(s.DB, KindMerchant, "m1", currency.USD); held != 500 {
		t.Fatalf("held=%d want 500 while the reserve stands", held)
	}

	// The reserve lapses, and the next outbound move settles the pool.
	s.Now = func() time.Time { return at.Add(2 * time.Hour) }
	next, err := s.Screen(context.Background(), Move{
		Stage: Payout, Subject: merchantOf("m1"), Amount: 1000, Currency: currency.USD, Out: true, Idem: "k-2",
	})
	if err != nil {
		t.Fatalf("second screen: %v", err)
	}
	if next.Held != 0 {
		t.Fatalf("a lapsed reserve still withheld %d", next.Held)
	}
	if held := reserve.Held(s.DB, KindMerchant, "m1", currency.USD); held != 0 {
		t.Fatalf("a lapsed reserve still holds %d of the merchant's money", held)
	}
}

// TestReserve_TheLedgerNeverGoesNegative — a ledger that can go below zero has
// invented money.
func TestReserve_TheLedgerNeverGoesNegative(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("negative", ctx, &oracle{answer: &Decision{Action: Allow}})
	e := reserve.NewEntry(s.DB)
	e.SubjectKind, e.Subject, e.Currency, e.Amount = KindMerchant, "m1", currency.USD, -1
	if _, _, err := reserve.Post(s.DB, e); !errors.Is(err, reserve.ErrShort) {
		t.Fatalf("a release from an empty pool was accepted: %v", err)
	}
}

// TestReserve_TheLedgerIsPerTenant — one org's withheld money is invisible to
// another, and releasing in one never touches the other. Same subject id in
// both, deliberately.
func TestReserve_TheLedgerIsPerTenant(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	p := &oracle{answer: &Decision{ID: "d1", Action: Allow}}
	a := tenant("rsva", ctx, p)
	b := tenant("rsvb", ctx, p)

	if _, err := Place(a, merchantOf("shared-id"), control.Reserve, 5000, 10_000, time.Time{}, "a's"); err != nil {
		t.Fatalf("a place: %v", err)
	}
	rec, err := a.Screen(context.Background(), Move{
		Stage: Payout, Subject: merchantOf("shared-id"), Amount: 1000, Currency: currency.USD, Out: true, Idem: "k-1",
	})
	if err != nil {
		t.Fatalf("a screen: %v", err)
	}
	if _, err := a.Hold(rec, "payout"); err != nil {
		t.Fatalf("a hold: %v", err)
	}

	if held := reserve.Held(b.DB, KindMerchant, "shared-id", currency.USD); held != 0 {
		t.Fatalf("org B reads %d of org A's withheld money", held)
	}
	if n := len(reserve.Entries(b.DB, KindMerchant, "shared-id", 0)); n != 0 {
		t.Fatalf("org B reads %d of org A's ledger entries", n)
	}
	if n := len(reserve.Balances(b.DB, "", "", 0)); n != 0 {
		t.Fatalf("org B reads %d of org A's balances", n)
	}

	// B frees its own (empty) pool for the same subject id. A's is untouched.
	if _, err := b.Free(merchantOf("shared-id"), "b's release"); err != nil {
		t.Fatalf("b free: %v", err)
	}
	if held := reserve.Held(a.DB, KindMerchant, "shared-id", currency.USD); held != 500 {
		t.Fatalf("org B's release moved org A's ledger: held=%d want 500", held)
	}
}
