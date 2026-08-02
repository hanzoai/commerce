// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/reserve"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// reserve_test.go is the gate on a reserve that bounded nothing: a percentage
// haircut on a caller-chosen number, with no ceiling it converges on, no
// account of what was taken, and nothing to return when the declaration is
// lifted.
//
// Every test that concerns MONEY runs the whole path — judge, then disburse —
// because the judgement moves nothing. What the ceiling does to a judgement is
// a forecast; what it does at the disbursement is the money. See
// account_test.go for the properties of the boundary itself.

// disburse is one whole money move: the judgement, then the withholding the
// payout boundary does. It reports the authoritative split.
func disburse(t *testing.T, s *Screener, m Move) (held, allowed int64) {
	t.Helper()
	rec, err := s.Screen(context.Background(), m)
	if err != nil {
		t.Fatalf("screen %s: %v", m.Idem, err)
	}
	a, h, err := s.Withhold(rec)
	if err != nil {
		t.Fatalf("withhold %s: %v", m.Idem, err)
	}
	if int64(a+h) != int64(m.Amount) {
		t.Fatalf("%s: the split lost a cent: %d + %d != %d", m.Idem, h, a, m.Amount)
	}
	return int64(h), int64(a)
}

// TestReserve_StopsAtItsCeiling — the rate says what share, the ceiling says
// how much in total. Past the ceiling the reserve takes nothing further and the
// money goes out.
func TestReserve_StopsAtItsCeiling(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("ceiling", ctx, &oracle{answer: &Decision{Action: Allow}})
	m := merchant("m1")
	if _, err := Place(s, Placement{
		Subject: m, Effect: control.Reserve, Rate: 5000, Cap: 1000, Currency: currency.USD,
	}); err != nil {
		t.Fatalf("place: %v", err)
	}
	pay := func(idem string, amount currency.Cents) (held, allowed int64) {
		t.Helper()
		return disburse(t, s, Move{
			Stage: Payout, Subject: m, Amount: amount, Currency: currency.USD, Out: true, Idem: idem,
		})
	}

	// 50% of 1200 is 600, under the 1000 ceiling.
	if held, allowed := pay("p1", 1200); held != 600 || allowed != 600 {
		t.Fatalf("first payout held=%d allowed=%d, want 600/600", held, allowed)
	}
	// 50% of 1200 is 600 again, but only 400 of headroom is left.
	if held, allowed := pay("p2", 1200); held != 400 || allowed != 800 {
		t.Fatalf("second payout held=%d allowed=%d, want 400/800 — the ceiling bounds the total", held, allowed)
	}
	// The ceiling is reached: nothing further is withheld.
	if held, allowed := pay("p3", 1200); held != 0 || allowed != 1200 {
		t.Fatalf("third payout held=%d allowed=%d, want 0/1200 — a reserve at its ceiling takes nothing", held, allowed)
	}
}

// TestReserve_WithoutACeilingIsUnchanged — the ceiling is opt-in, and a reserve
// that declares none behaves exactly as it did.
func TestReserve_WithoutACeilingIsUnchanged(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("nocap", ctx, &oracle{answer: &Decision{Action: Allow}})
	m := merchant("m1")
	if _, err := Place(s, Placement{Subject: m, Effect: control.Reserve, Rate: 2500}); err != nil {
		t.Fatalf("place: %v", err)
	}
	for i, idem := range []string{"a", "b", "c"} {
		held, allowed := disburse(t, s, Move{
			Stage: Payout, Subject: m, Amount: 400, Currency: currency.USD, Out: true, Idem: idem,
		})
		if held != 100 || allowed != 300 {
			t.Fatalf("move %d held=%d allowed=%d, want 100/300", i, held, allowed)
		}
	}
}

// TestReserve_TheLedgerRecordsWhatWasWithheld — a withheld cent that nothing
// records is not a reserve, it is a shortfall with no name.
func TestReserve_TheLedgerRecordsWhatWasWithheld(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("ledger", ctx, &oracle{answer: &Decision{Action: Allow}})
	m := merchant("m1")
	c, err := Place(s, Placement{Subject: m, Effect: control.Reserve, Rate: 2500})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	rec, err := s.Screen(context.Background(), Move{
		Stage: Payout, Subject: m, Amount: 401, Currency: currency.USD, Out: true,
		Reference: "po_9", Idem: "p1",
	})
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	if n := len(reserve.For(s.DB, m.Kind, m.ID, 0)); n != 0 {
		t.Fatalf("the judgement alone posted %d ledger entries", n)
	}
	if _, held, err := s.Withhold(rec); err != nil || held != 101 {
		t.Fatalf("withhold: held=%d err=%v, want 101", held, err)
	}

	rows := reserve.For(s.DB, m.Kind, m.ID, 0)
	if len(rows) != 1 {
		t.Fatalf("%d ledger entries, want 1", len(rows))
	}
	e := rows[0]
	if e.Held != rec.Held || e.Released != 0 {
		t.Fatalf("entry held=%d released=%d, want %d/0", e.Held, e.Released, rec.Held)
	}
	if e.Control != c.Id() || e.Screen != rec.Id() || e.Reference != "po_9" {
		t.Fatalf("the entry does not join back to the declaration, the judgement and the payout: %+v", e)
	}
	if e.Currency != currency.USD {
		t.Fatalf("entry currency=%q — reserved money is denominated", e.Currency)
	}
}

// TestReserve_ABlockPostsNothing — a blocked move withholds everything by not
// happening. Nothing was taken, so nothing is accounted for.
func TestReserve_ABlockPostsNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("blockledger", ctx, &oracle{answer: &Decision{Action: Allow}})
	m := merchant("m1")
	if _, err := Place(s, Placement{Subject: m, Effect: control.Reserve, Rate: 2500}); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if _, err := Place(s, Placement{Subject: m, Effect: control.Block}); err != nil {
		t.Fatalf("block: %v", err)
	}
	rec, err := s.Screen(context.Background(), Move{
		Stage: Payout, Subject: m, Amount: 400, Currency: currency.USD, Out: true, Idem: "p1",
	})
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	if !Refused(rec) {
		t.Fatalf("the block did not refuse: %s", rec.Action)
	}
	if _, held, err := s.Withhold(rec); err != nil || held != 0 {
		t.Fatalf("a blocked move withheld %d (%v)", held, err)
	}
	if n := len(reserve.For(s.DB, m.Kind, m.ID, 0)); n != 0 {
		t.Fatalf("%d ledger entries for a move that never happened", n)
	}
}

// TestLift_RecordsWhoLiftedIt — releasing a declaration closes its account and
// says who closed it. What the release RETURNS is
// [TestLift_ReturnsExactlyWhatWasWithheld]; this is the audit half.
func TestLift_RecordsWhoLiftedIt(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("lift", ctx, &oracle{answer: &Decision{Action: Allow}})
	m := merchant("m1")
	c, err := Place(s, Placement{Subject: m, Effect: control.Reserve, Rate: 5000})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if held, _ := disburse(t, s, Move{
		Stage: Payout, Subject: m, Amount: 600, Currency: currency.USD, Out: true, Idem: "p1",
	}); held != 300 {
		t.Fatalf("withheld %d, want 300", held)
	}

	lifted, err := Lift(s, c.Id())
	if err != nil {
		t.Fatalf("lift: %v", err)
	}
	if !lifted.Released || lifted.ReleasedBy != "u_test" {
		t.Fatalf("lifted=%+v — a release records who lifted it", lifted)
	}
	if got := reserve.Held(s.DB, c); got != 0 {
		t.Fatalf("the account still holds %d after the declaration was lifted", got)
	}
}

// TestPlace_RefusesACeilingItCannotDenominate — an amount without a currency is
// a number, and a ceiling nobody can compare against is not a ceiling.
func TestPlace_RefusesACeilingItCannotDenominate(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("capvalid", ctx, &oracle{answer: &Decision{Action: Allow}})
	m := merchant("m1")
	for name, p := range map[string]Placement{
		"a ceiling with no currency": {Subject: m, Effect: control.Reserve, Rate: 2500, Cap: 1000},
		"a negative ceiling":         {Subject: m, Effect: control.Reserve, Rate: 2500, Cap: -1, Currency: currency.USD},
	} {
		if _, err := Place(s, p); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
	if _, err := Place(s, Placement{
		Subject: m, Effect: control.Reserve, Rate: 2500, Cap: 1000, Currency: currency.USD,
	}); err != nil {
		t.Fatalf("a denominated ceiling was refused: %v", err)
	}
}

// TestPlace_AHoldCarriesNoReserveFields — a rate, a ceiling or a currency on a
// hold would be stored and silently never applied, which is worse than refusing
// them: a platform would believe it had capped something.
func TestPlace_AHoldCarriesNoReserveFields(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("holdfields", ctx, &oracle{answer: &Decision{Action: Allow}})
	c, err := Place(s, Placement{
		Subject: merchant("m1"), Effect: control.Hold, Rate: 2500, Cap: 999, Currency: currency.USD,
	})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if c.Rate != 0 || c.Cap != 0 || c.Currency != "" {
		t.Fatalf("a hold kept reserve fields: rate=%d cap=%d currency=%q", c.Rate, c.Cap, c.Currency)
	}
}

// TestReserve_ScopedToItsCurrency — a ceiling declared in USD cannot be
// satisfied by holds taken in EUR, so a denominated reserve bears only on the
// money it names.
func TestReserve_ScopedToItsCurrency(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("curscope", ctx, &oracle{answer: &Decision{Action: Allow}})
	m := merchant("m1")
	if _, err := Place(s, Placement{
		Subject: m, Effect: control.Reserve, Rate: 2500, Cap: 100000, Currency: currency.USD,
	}); err != nil {
		t.Fatalf("place: %v", err)
	}
	if held, _ := disburse(t, s, Move{
		Stage: Payout, Subject: m, Amount: 400, Currency: currency.USD, Out: true, Idem: "u1",
	}); held != 100 {
		t.Fatalf("usd held=%d want 100", held)
	}
	if held, allowed := disburse(t, s, Move{
		Stage: Payout, Subject: m, Amount: 400, Currency: currency.EUR, Out: true, Idem: "e1",
	}); held != 0 || allowed != 400 {
		t.Fatalf("a USD-denominated reserve took %d from a EUR payout", held)
	}
}

// TestCap_IsPureAndKeepsTheSplitExact.
func TestCap_IsPureAndKeepsTheSplitExact(t *testing.T) {
	base := Restraint{Held: 600, Allowed: 600}
	for _, c := range []struct{ headroom, held, allowed currency.Cents }{
		{currency.Cents(reserve.Unlimited), 600, 600},
		{1000, 600, 600},
		{600, 600, 600},
		{250, 250, 950},
		{0, 0, 1200},
		{-5, 0, 1200},
	} {
		got := Cap(base, c.headroom)
		if got.Held != c.held || got.Allowed != c.allowed {
			t.Fatalf("headroom=%d: held=%d allowed=%d, want %d/%d", c.headroom, got.Held, got.Allowed, c.held, c.allowed)
		}
		if got.Held+got.Allowed != 1200 {
			t.Fatalf("headroom=%d: the clamp lost a cent: %d + %d", c.headroom, got.Held, got.Allowed)
		}
	}
	blocked := Restraint{Blocked: true, Held: 1200}
	if got := Cap(blocked, 0); got.Held != 1200 || got.Allowed != 0 {
		t.Fatalf("a ceiling loosened a block: held=%d allowed=%d", got.Held, got.Allowed)
	}
}

// TestReserve_TheLedgerIsTenantScoped.
func TestReserve_TheLedgerIsTenantScoped(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	p := &oracle{answer: &Decision{Action: Allow}}
	a := tenant("ledgerisoa", ctx, p)
	b := tenant("ledgerisob", ctx, p)
	m := merchant("shared")

	if _, err := Place(a, Placement{Subject: m, Effect: control.Reserve, Rate: 2500}); err != nil {
		t.Fatalf("place: %v", err)
	}
	if held, _ := disburse(t, a, Move{
		Stage: Payout, Subject: m, Amount: 400, Currency: currency.USD, Out: true, Idem: "p1",
	}); held != 100 {
		t.Fatalf("org A withheld %d, want 100", held)
	}
	if n := len(reserve.For(b.DB, m.Kind, m.ID, 0)); n != 0 {
		t.Fatalf("org B read %d entries of org A's reserve ledger", n)
	}
	if held, allowed := disburse(t, b, Move{
		Stage: Payout, Subject: m, Amount: 400, Currency: currency.USD, Out: true, Idem: "p1",
	}); held != 0 || allowed != 400 {
		t.Fatalf("org A's reserve withheld %d of org B's payout", held)
	}
}
