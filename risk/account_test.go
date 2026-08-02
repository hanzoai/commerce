// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/reserve"
	"github.com/hanzoai/commerce/models/screen"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// account_test.go is the gate on the defect that made a ceiling worse than no
// ceiling: the money was accounted for by the JUDGEMENT, so anything that could
// ask a question could spend the ceiling, and two questions asked at once could
// spend it twice.
//
// The property every test here defends is one sentence: THE ACCOUNT MOVES WHERE
// THE MONEY MOVES, FOR WHAT THE MONEY MOVED.

// screened is one judgement, the way the payout boundary gets one.
func screened(t *testing.T, s *Screener, m Move) *screen.Screen {
	t.Helper()
	rec, err := s.Screen(context.Background(), m)
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	return rec
}

// account is what one declaration is actually holding.
func account(s *Screener, c *control.Control) int64 { return reserve.Held(s.DB, c) }

// ledger is what the entries say is held, net of what they returned. It must
// equal the account at every instant a test can observe — that is the whole
// point of writing them together.
func ledgerOf(s *Screener, m Subject) (held, released int64) {
	for _, e := range reserve.For(s.DB, m.Kind, m.ID, 0) {
		held += e.Held
		released += e.Released
	}
	return held, released
}

// TestScreen_AnswersWithoutSpendingTheCeiling — SB-1. A screen is a question.
// Asking it must not consume the reserve, or any authenticated caller disarms
// its own reserve with one money-free request and the NEXT real payout goes out
// whole.
func TestScreen_AnswersWithoutSpendingTheCeiling(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("noburn", ctx, &oracle{answer: &Decision{Action: Allow}})
	m := merchant("m1")
	c, err := Place(s, Placement{
		Subject: m, Effect: control.Reserve, Rate: 2500, Cap: 10000, Currency: currency.USD,
	})
	if err != nil {
		t.Fatalf("place: %v", err)
	}

	// One fictitious $400 payout, screened and never disbursed.
	screened(t, s, Move{Stage: Payout, Subject: m, Amount: 40000, Currency: currency.USD, Out: true, Idem: "burn-1"})

	if got := account(s, c); got != 0 {
		t.Fatalf("a screen moved the account to %d — a question spent the ceiling", got)
	}
	if h, r := ledgerOf(s, m); h != 0 || r != 0 {
		t.Fatalf("a screen posted to the ledger: held=%d released=%d", h, r)
	}
	if got := reserve.Headroom(s.DB, c); got != 10000 {
		t.Fatalf("headroom=%d, want the full 10000 — the ceiling is not a consumable", got)
	}

	// The reserve is therefore still armed for the REAL payout.
	real1 := screened(t, s, Move{Stage: Payout, Subject: m, Amount: 1000000, Currency: currency.USD, Out: true, Idem: "real-1"})
	if real1.Held != 10000 || real1.Allowed != 990000 {
		t.Fatalf("the real payout answered held=%d allowed=%d — the reserve was disarmed", real1.Held, real1.Allowed)
	}
}

// TestScreen_ARefusedMoveTakesNothing — SB-1 through the payout door. A move
// the plane BLOCKED moved no money, so nothing may be withheld from it: a
// refusal that consumes the ceiling burns the reserve on payouts that never
// happen, and writes a ledger saying money was taken from a move that did not
// occur.
func TestScreen_ARefusedMoveTakesNothing(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("refusednoburn", ctx, &oracle{answer: &Decision{Action: Block}})
	m := merchant("m1")
	c, err := Place(s, Placement{
		Subject: m, Effect: control.Reserve, Rate: 5000, Cap: 10000, Currency: currency.USD,
	})
	if err != nil {
		t.Fatalf("place: %v", err)
	}

	for _, idem := range []string{"r1", "r2"} {
		rec := screened(t, s, Move{
			Stage: Payout, Subject: m, Amount: 20000, Currency: currency.USD, Out: true, Idem: idem,
		})
		if !Refused(rec) {
			t.Fatalf("%s: expected a refusal, got action=%s", idem, rec.Action)
		}
		// A refusal NAMES NO RESERVE. Nothing moved, so nothing took a share, and
		// a row that named one would let a disbursement boundary withhold against
		// a payout the plane blocked — a ceiling burned on moves that never
		// happened, and a ledger saying money came out of nothing.
		if rec.Reserve != "" {
			t.Fatalf("%s: a refused move names reserve %q", idem, rec.Reserve)
		}
		// And even if the boundary tried anyway, the refusal takes nothing.
		allowed, held, err := s.Withhold(rec)
		if err != nil {
			t.Fatalf("%s: withhold: %v", idem, err)
		}
		if held != 0 {
			t.Fatalf("%s: a refused move withheld %d", idem, held)
		}
		_ = allowed
	}

	if got := account(s, c); got != 0 {
		t.Fatalf("two refused payouts consumed %d of the ceiling", got)
	}
	if h, _ := ledgerOf(s, m); h != 0 {
		t.Fatalf("the ledger records %d withheld from moves that never happened", h)
	}
}

// TestWithhold_ConcurrentDisbursementsNeverBreachTheCeiling — SB-3. The ceiling
// is enforced inside the store's own transaction, so N simultaneous
// disbursements withhold the ceiling in total and not N times it. A
// read-modify-write here loses increments, and a lost increment on a running
// total that bounds money is a reserve withholding past its own declaration.
func TestWithhold_ConcurrentDisbursementsNeverBreachTheCeiling(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("racecap", ctx, &oracle{answer: &Decision{Action: Allow}})
	m := merchant("m1")
	const cap0 = 1000
	c, err := Place(s, Placement{
		Subject: m, Effect: control.Reserve, Rate: 10000, Cap: cap0, Currency: currency.USD,
	})
	if err != nil {
		t.Fatalf("place: %v", err)
	}

	// Judge them all first: a judgement is not the money, and screening
	// concurrently would only prove screening is safe.
	const n = 8
	recs := make([]*screen.Screen, n)
	for i := range recs {
		recs[i] = screened(t, s, Move{
			Stage: Payout, Subject: m, Amount: 1000, Currency: currency.USD, Out: true,
			Idem: "c" + string(rune('a'+i)),
		})
	}

	var wg sync.WaitGroup
	took := make([]int64, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, held, err := s.Withhold(recs[i])
			took[i], errs[i] = int64(held), err
		}(i)
	}
	wg.Wait()

	var sum int64
	for i, e := range errs {
		if e != nil {
			t.Fatalf("withhold %d: %v", i, e)
		}
		sum += took[i]
	}

	if sum > cap0 {
		t.Fatalf("BREACH: %d concurrent disbursements withheld %d against a declared ceiling of %d", n, sum, cap0)
	}
	if got := account(s, c); got != sum {
		t.Fatalf("the account says %d and the disbursements took %d", got, sum)
	}
	held, released := ledgerOf(s, m)
	if held-released != sum {
		t.Fatalf("the ledger says %d net and the account says %d — they diverged", held-released, sum)
	}
	if sum != cap0 {
		t.Fatalf("withheld %d of a %d ceiling — the reserve stopped short of its own declaration", sum, cap0)
	}
}

// TestLift_ReturnsExactlyWhatWasWithheld — the invariant a merchant's money
// depends on. Whatever the account holds is what a lift gives back, because
// they are the same number written in one act.
func TestLift_ReturnsExactlyWhatWasWithheld(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("liftexact", ctx, &oracle{answer: &Decision{Action: Allow}})
	m := merchant("m1")
	c, err := Place(s, Placement{
		Subject: m, Effect: control.Reserve, Rate: 5000, Cap: 900, Currency: currency.USD,
	})
	if err != nil {
		t.Fatalf("place: %v", err)
	}

	var withheld int64
	for _, idem := range []string{"p1", "p2", "p3"} {
		rec := screened(t, s, Move{
			Stage: Payout, Subject: m, Amount: 601, Currency: currency.USD, Out: true, Idem: idem,
		})
		_, held, err := s.Withhold(rec)
		if err != nil {
			t.Fatalf("withhold %s: %v", idem, err)
		}
		withheld += int64(held)
	}
	if withheld != 900 {
		t.Fatalf("withheld %d, want the whole 900 ceiling", withheld)
	}

	if _, err := Lift(s, c.Id()); err != nil {
		t.Fatalf("lift: %v", err)
	}
	held, released := ledgerOf(s, m)
	if released != withheld {
		t.Fatalf("the lift returned %d of the %d that was withheld", released, withheld)
	}
	if held-released != 0 || account(s, c) != 0 {
		t.Fatalf("after the lift the account holds %d and the ledger nets %d", account(s, c), held-released)
	}

	// Lifting twice returns the money once.
	if _, err := Lift(s, c.Id()); err != nil {
		t.Fatalf("second lift: %v", err)
	}
	if _, again := ledgerOf(s, m); again != withheld {
		t.Fatalf("a retried lift returned the money twice on paper: %d", again)
	}
}

// TestWithhold_ARepeatThatTightensIsAccountedForWhenTheMoneyMoves — SB-4. A
// repeat that tightens onto a reserve placed AFTER the first answer withholds
// real money, and the entry that names it is posted by the same act that moves
// the money, so there is no path on which a cent is held and not recorded.
func TestWithhold_ARepeatThatTightensIsAccountedForWhenTheMoneyMoves(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("tighten", ctx, &oracle{answer: &Decision{Action: Allow}})
	m := merchant("m1")
	move := Move{Stage: Payout, Subject: m, Amount: 101, Currency: currency.USD, Out: true, Idem: "pay-2"}

	first := screened(t, s, move)
	if first.Held != 0 {
		t.Fatalf("the first answer withheld %d with no reserve in force", first.Held)
	}

	c, err := Place(s, Placement{
		Subject: m, Effect: control.Reserve, Rate: 2500, Cap: 1000, Currency: currency.USD,
	})
	if err != nil {
		t.Fatalf("place: %v", err)
	}

	again := screened(t, s, move)
	if again.Held != 26 || again.Allowed != 75 {
		t.Fatalf("the repeat answered held=%d allowed=%d, want 26/75", again.Held, again.Allowed)
	}
	// Still a judgement: nothing has moved.
	if got := account(s, c); got != 0 {
		t.Fatalf("the repeat moved the account to %d before any money left", got)
	}

	allowed, held, err := s.Withhold(again)
	if err != nil {
		t.Fatalf("withhold: %v", err)
	}
	if held != 26 || allowed != 75 {
		t.Fatalf("the disbursement withheld %d and released %d, want 26/75", held, allowed)
	}
	if got := account(s, c); got != 26 {
		t.Fatalf("26 cents were withheld and the account says %d", got)
	}
	rows := reserve.For(s.DB, m.Kind, m.ID, 0)
	if len(rows) != 1 || rows[0].Held != 26 || rows[0].Screen != again.Id() {
		t.Fatalf("the withheld cents are recorded nowhere that names them: %+v", rows)
	}
}

// TestWithhold_IsIdempotentOnTheJudgement — the screen is idempotent, so the
// money it moves must be too: a retried disbursement under one judgement takes
// the share once.
func TestWithhold_IsIdempotentOnTheJudgement(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("withholdidem", ctx, &oracle{answer: &Decision{Action: Allow}})
	m := merchant("m1")
	c, err := Place(s, Placement{Subject: m, Effect: control.Reserve, Rate: 2500, Cap: 10000, Currency: currency.USD})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	rec := screened(t, s, Move{Stage: Payout, Subject: m, Amount: 400, Currency: currency.USD, Out: true, Idem: "p1"})

	for i := 0; i < 3; i++ {
		allowed, held, err := s.Withhold(rec)
		if err != nil {
			t.Fatalf("withhold %d: %v", i, err)
		}
		if held != 100 || allowed != 300 {
			t.Fatalf("attempt %d: held=%d allowed=%d, want 100/300", i, held, allowed)
		}
	}
	if got := account(s, c); got != 100 {
		t.Fatalf("three attempts at one disbursement withheld %d", got)
	}
	if n := len(reserve.For(s.DB, m.Kind, m.ID, 0)); n != 1 {
		t.Fatalf("%d ledger entries for one move", n)
	}
}

// TestWithhold_AHoldThatNamesNoReserveIsPaidOut — the shortfall-with-no-name,
// closed at the disbursement.
//
// A judgement carrying Held with no reserve to take it can only come from a row
// this code did not write (an earlier build, a hand-edited record). Paying that
// row's Allowed would keep the difference: money not disbursed, no ledger entry
// naming it, no ceiling counting it and no lift returning it — which is exactly
// what the reserve ledger exists to make impossible. So the whole amount goes
// out, and the merchant is never short by an amount nothing records.
func TestWithhold_AHoldThatNamesNoReserveIsPaidOut(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	s := tenant("nameless", ctx, &oracle{answer: &Decision{Action: Allow}})
	m := merchant("m1")

	rec := screen.New(s.DB)
	rec.Stage, rec.SubjectKind, rec.Subject = string(Payout), m.Kind, m.ID
	rec.Currency, rec.Out = currency.USD, true
	rec.Action = string(Allow)
	rec.Amount, rec.Held, rec.Allowed = 1000, 250, 750
	rec.Reserve = "" // nothing declares the hold
	if err := rec.Create(); err != nil {
		t.Fatalf("seed: %v", err)
	}

	allowed, held, err := s.Withhold(rec)
	if err != nil {
		t.Fatalf("withhold: %v", err)
	}
	if held != 0 {
		t.Fatalf("withheld %d under no declaration", held)
	}
	if allowed != 1000 {
		t.Fatalf("paid out %d of 1000 — %d cents kept with nothing naming them",
			allowed, 1000-allowed)
	}
	if h, _ := ledgerOf(s, m); h != 0 {
		t.Fatalf("the ledger records %d withheld under no declaration", h)
	}
}

// TestRestore_GivesBackAShareWhoseMoveDidNotHappen — a disbursement that failed
// after its share was withheld must not leave the merchant short forever under
// a ceiling that believes it is that much fuller.
func TestRestore_GivesBackAShareWhoseMoveDidNotHappen(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("restore", ctx, &oracle{answer: &Decision{Action: Allow}})
	m := merchant("m1")
	c, err := Place(s, Placement{Subject: m, Effect: control.Reserve, Rate: 5000, Cap: 1000, Currency: currency.USD})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	rec := screened(t, s, Move{Stage: Payout, Subject: m, Amount: 600, Currency: currency.USD, Out: true, Idem: "p1"})
	if _, held, err := s.Withhold(rec); err != nil || held != 300 {
		t.Fatalf("withhold: held=%d err=%v", held, err)
	}

	for i := 0; i < 2; i++ { // idempotent
		if err := s.Restore(rec); err != nil {
			t.Fatalf("restore %d: %v", i, err)
		}
	}
	if got := account(s, c); got != 0 {
		t.Fatalf("the account still holds %d after the move was undone", got)
	}
	held, released := ledgerOf(s, m)
	if held != 300 || released != 300 {
		t.Fatalf("the ledger says held=%d released=%d — a hold that happened and was returned once", held, released)
	}
	// And the headroom is genuinely back.
	if got := reserve.Headroom(s.DB, c); got != 1000 {
		t.Fatalf("headroom=%d after the return, want the full 1000", got)
	}
}

// TestTake_HasExactlyOneCallerInThisTree pins the whole shape of the fix.
//
// [reserve.Take] is the one door that moves money into the account. It is safe
// only because it is called at the disbursement boundary and nowhere else — a
// second caller in a judgement, a list, a review or a cron re-creates the exact
// defect this package was rebuilt to close, and it would do so silently. So the
// call site is an invariant, and an invariant nobody checks is a comment.
func TestTake_HasExactlyOneCallerInThisTree(t *testing.T) {
	callers := callSites(t, ".", "reserve", "Take", "Return", "Close")
	want := map[string][]string{
		"reserve.Take":   {"risk/screen.go:Withhold"},
		"reserve.Return": {"risk/screen.go:Restore"},
		"reserve.Close":  {"risk/monitor.go:Lift"},
	}
	for fn, expect := range want {
		got := callers[fn]
		if strings.Join(got, ",") != strings.Join(expect, ",") {
			t.Fatalf("%s is called from %v, want exactly %v — the account has one door per direction",
				fn, got, expect)
		}
	}
}

// TestHeld_IsAskedForOneDeclarationAndNeverForAPage — A BOUND ON ROWS IS NOT A
// BOUND ON READS.
//
// reserve.Held is one store read about ONE declaration. Called from a loop over
// a page it turns one request into up to control.Max round trips against a
// store shared with every other tenant — the same amplification a page limit
// exists to prevent, moved one layer down and invisible in the response. A page
// asks reserve.Accounts once. So the call sites are the invariant.
func TestHeld_IsAskedForOneDeclarationAndNeverForAPage(t *testing.T) {
	got := callSites(t, ".", "reserve", "Held")["reserve.Held"]
	want := []string{
		"api/billing/risk.go:riskControlPlace",   // one declaration, just placed
		"api/billing/risk.go:riskControlRelease", // one declaration, just lifted
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("reserve.Held is called from %v, want exactly %v — a page reads "+
			"reserve.Accounts once", got, want)
	}
}

// moneyPlane is every package that decides or moves money on this track.
var moneyPlane = []string{
	"risk", "api/billing",
	"models/reserve", "models/control", "models/screen",
	"models/outcome", "models/idempotencykey", "models/payout",
}

// TestTheMoneyPlaneNeverAsksForAFakeTransaction — datastore.RunInTransaction
// takes a function and a TransactionOptions, reads neither, and RUNS THE BODY
// WITH NO ISOLATION AND NO ATOMICITY. It is indistinguishable at the call site
// from the real one on the store, so a read-modify-write placed inside it loses
// updates and reports success — which is exactly how a reserve came to withhold
// eight times its own declared ceiling.
//
// The name cannot be fixed from this track (a dozen callers outside it depend
// on it), so what is pinned instead is that the MONEY PLANE never spells it.
// Indivisibility is asked for at the store, once, in models/reserve.
func TestTheMoneyPlaneNeverAsksForAFakeTransaction(t *testing.T) {
	for _, dir := range moneyPlane {
		for fn, sites := range callSites(t, dir, "datastore", "RunInTransaction") {
			t.Fatalf("%s calls %s at %v — that call provides no transaction. "+
				"Ask the STORE: ds.DB().RunInTransaction(ctx, fn, "+
				"&db.TransactionOptions{Isolation: db.IsolationSerializable, MaxAttempts: n})",
				dir, fn, sites)
		}
	}
}

// callSites reports, for each named function of package pkg, the
// "<file>:<func>" of every non-test call to it under dir.
func callSites(t *testing.T, dir, pkg string, names ...string) map[string][]string {
	t.Helper()
	root := repoRoot(t)
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	out := map[string][]string{}

	err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor":
				return filepath.SkipDir
			case pkg: // a package does not call itself through its own name
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil // not our business to police unparseable files
		}
		rel, _ := filepath.Rel(root, path)
		var fn string
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.FuncDecl:
				fn = v.Name.Name
			case *ast.SelectorExpr:
				id, ok := v.X.(*ast.Ident)
				if ok && id.Name == pkg && want[v.Sel.Name] {
					out[pkg+"."+v.Sel.Name] = append(out[pkg+"."+v.Sel.Name], rel+":"+fn)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return out
}

// repoRoot is the module root, found from this package upwards.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test package")
		}
		dir = parent
	}
}

// TestScreen_AKeyRefusedOnceStaysRefusedAndAFreshKeyDoesNot pins the contract
// that the payout boundary's own comment used to get backwards.
//
// A key names ONE question and its answer never loosens — that is the whole
// reason a replay cannot lift a live control, and it is deliberate. So a key
// that was refused under a hold is refused FOREVER, even after the hold is
// lifted, and the way a merchant asks again is with a NEW key. The payout guard
// being released on a refusal frees the guard, not the judgement; the two
// layers now say the same thing.
func TestScreen_AKeyRefusedOnceStaysRefusedAndAFreshKeyDoesNot(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("wedge", ctx, &oracle{answer: &Decision{Action: Allow}})
	m := merchant("m1")
	c, err := Place(s, Placement{Subject: m, Effect: control.Hold})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	move := Move{Stage: Payout, Subject: m, Amount: 5000, Currency: currency.USD, Out: true, Idem: "k1"}

	if first := screened(t, s, move); !Refused(first) {
		t.Fatalf("the hold did not refuse: %s", first.Action)
	}
	if _, err := Lift(s, c.Id()); err != nil {
		t.Fatalf("lift: %v", err)
	}
	live, err := control.LiveFor(s.DB, m.Kind, m.ID, s.now())
	if err != nil || len(live) != 0 {
		t.Fatalf("the control is still live: %v %d", err, len(live))
	}

	// The SAME key still refuses: an answer never loosens.
	if again := screened(t, s, move); !Refused(again) {
		t.Fatalf("the same key loosened to %s after the control was lifted — "+
			"a replay must never lift a restraint, in either direction", again.Action)
	}
	// A FRESH key is a fresh question, judged by the world as it is now.
	fresh := move
	fresh.Idem = "k2"
	if now := screened(t, s, fresh); Refused(now) {
		t.Fatalf("a fresh key still refuses (%s) although no control is in force — "+
			"the merchant has no way to ask again", now.Action)
	}
}
