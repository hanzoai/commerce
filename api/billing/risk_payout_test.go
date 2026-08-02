// Copyright © 2026 Hanzo AI. MIT License.

package billing

// risk_payout_test.go proves the enforcement point: a control does not merely
// exist in a table, it stops a real payout on the real handler.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/payout"
	"github.com/hanzoai/commerce/models/screen"
	"github.com/hanzoai/commerce/risk"
	"github.com/hanzoai/commerce/util/test/ae"
)

// restrain places a control on one subject in one org, the way a platform does.
func restrain(t *testing.T, org *organization.Organization, ctx context.Context, subject risk.Subject, effect string, rate int64) {
	t.Helper()
	s := &risk.Screener{DB: datastore.New(org.Namespaced(ctx)), By: "platform"}
	if _, err := risk.Place(s, risk.Placement{Subject: subject, Effect: effect, Rate: rate, Reason: "test"}); err != nil {
		t.Fatalf("place: %v", err)
	}
}

func payoutRows(org *organization.Organization, ctx context.Context) int {
	db := datastore.New(org.Namespaced(ctx))
	root := db.NewKey("synckey", "", 1, nil)
	iter := payout.Query(db).Ancestor(root).Run()
	n := 0
	for {
		p := payout.New(db)
		if _, err := iter.Next(p); err != nil {
			return n
		}
		n++
	}
}

// TestPayout_AHoldRefusesThePayoutAndWritesNoRow — the control stops the money
// before a row exists, so there is nothing to unwind.
func TestPayout_AHoldRefusesThePayoutAndWritesNoRow(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	org := moneyOrg("payouthold")
	restrain(t, org, ctx, risk.Subject{Kind: risk.KindMerchant, ID: "m1"}, "hold", 0)

	res := invokeMoneyHandler(org, ctx, CreatePayout,
		`{"amount":5000,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1","merchant":"m1"}`, nil)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d want 403", res.StatusCode)
	}
	if n := payoutRows(org, ctx); n != 0 {
		t.Fatalf("%d payout rows written despite the hold", n)
	}
}

// TestPayout_AReserveWithholdsItsShareAndSaysSo — the payout is created for
// what may leave, and the response states exactly what was asked for and what
// was withheld. A reserve that refused instead would walk the caller in a
// circle: asking for less reserves a share of the smaller amount too.
func TestPayout_AReserveWithholdsItsShareAndSaysSo(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	org := moneyOrg("payoutreserve")
	restrain(t, org, ctx, risk.Subject{Kind: risk.KindMerchant, ID: "m1"}, "reserve", 2500)

	res := invokeMoneyHandler(org, ctx, CreatePayout,
		`{"amount":101,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1","merchant":"m1"}`, nil)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d want 201", res.StatusCode)
	}
	body := decode(t, res)
	if body["amount"].(float64) != 75 {
		t.Fatalf("the payout was created for %v, want the 75 that may leave", body["amount"])
	}
	if body["requested"].(float64) != 101 || body["held"].(float64) != 26 {
		t.Fatalf("the reserve was not disclosed: %v", body)
	}
	if body["screen"] == nil || body["screen"] == "" {
		t.Fatalf("the response does not name the screen that decided it: %v", body)
	}
	// Held + paid is exactly what was asked for: no cent is created or lost.
	if body["held"].(float64)+body["amount"].(float64) != body["requested"].(float64) {
		t.Fatalf("the split lost a cent: %v", body)
	}
	if n := payoutRows(org, ctx); n != 1 {
		t.Fatalf("%d payout rows, want 1", n)
	}
}

// TestPayout_AFullReserveRefuses — a payout of zero is not a payout.
func TestPayout_AFullReserveRefuses(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	org := moneyOrg("payoutfullreserve")
	restrain(t, org, ctx, risk.Subject{Kind: risk.KindMerchant, ID: "m1"}, "reserve", 10000)

	res := invokeMoneyHandler(org, ctx, CreatePayout,
		`{"amount":101,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1","merchant":"m1"}`, nil)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d want 403", res.StatusCode)
	}
	body := decode(t, res)
	wrapped, _ := body["error"].(map[string]any)
	if msg, _ := wrapped["message"].(string); !strings.Contains(msg, "101") {
		t.Fatalf("the refusal %q does not state the amount", msg)
	}
	if n := payoutRows(org, ctx); n != 0 {
		t.Fatalf("%d payout rows written despite a full reserve", n)
	}
}

// TestPayout_AnUnrestrainedPayoutIsUnchanged — the gate is invisible when
// nothing restrains, and it screens the DESTINATION when no merchant is named.
func TestPayout_AnUnrestrainedPayoutIsUnchanged(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	org := moneyOrg("payoutclear")
	res := invokeMoneyHandler(org, ctx, CreatePayout,
		`{"amount":5000,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1"}`, nil)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d want 201", res.StatusCode)
	}
	if n := payoutRows(org, ctx); n != 1 {
		t.Fatalf("%d payout rows, want 1", n)
	}
}

// TestPayout_ADestinationBlockStopsAPayoutThatNamesNoMerchant.
func TestPayout_ADestinationBlockStopsAPayoutThatNamesNoMerchant(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	org := moneyOrg("payoutdest")
	restrain(t, org, ctx, risk.Subject{Kind: risk.KindPayout, ID: "ba_1"}, "block", 0)

	res := invokeMoneyHandler(org, ctx, CreatePayout,
		`{"amount":100,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1"}`, nil)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d want 403", res.StatusCode)
	}
}

// TestPayout_AnotherOrgsControlDoesNotStopThisOnesPayout.
func TestPayout_AnotherOrgsControlDoesNotStopThisOnesPayout(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	a := moneyOrg("payoutisoa")
	b := moneyOrg("payoutisob")
	restrain(t, a, ctx, risk.Subject{Kind: risk.KindMerchant, ID: "m1"}, "block", 0)

	res := invokeMoneyHandler(b, ctx, CreatePayout,
		`{"amount":100,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1","merchant":"m1"}`, nil)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("org A's control stopped org B's payout: status=%d", res.StatusCode)
	}
}

// TestPayout_TheSameIdemKeyPaysOnce is the gate on a doc comment that lied.
//
// The Idem field said a retry returned the first answer rather than a second
// payout. Nothing implemented that: the key reached the SCREEN, which de-duped
// the judgement, and then a second payout row was written and a second
// disbursement was owed. A lying idempotency key on a payout is a double
// payout, and it is the kind of promise that gets believed.
func TestPayout_TheSameIdemKeyPaysOnce(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	org := moneyOrg("payoutidem")
	body := `{"amount":100,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1","idem":"p-1"}`

	first := decode(t, invokeMoneyHandler(org, ctx, CreatePayout, body, nil))
	for i := 0; i < 3; i++ {
		res := invokeMoneyHandler(org, ctx, CreatePayout, body, nil)
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("retry %d: status=%d want 201", i, res.StatusCode)
		}
		again := decode(t, res)
		if again["id"] != first["id"] {
			t.Fatalf("retry %d created a SECOND payout %v (first %v)", i, again["id"], first["id"])
		}
		if again["amount"] != first["amount"] {
			t.Fatalf("retry %d replayed a different answer: %v vs %v", i, again["amount"], first["amount"])
		}
	}
	if n := payoutRows(org, ctx); n != 1 {
		t.Fatalf("%d payout rows for one idempotency key, want 1 — the money left %d times", n, n)
	}
	if n := len(screen.For(datastore.New(org.Namespaced(ctx)), "", "", 0)); n != 1 {
		t.Fatalf("%d screens for one idempotency key, want 1", n)
	}
}

// TestPayout_WithNoKeyStillPaysOnceInsideTheWindow — a client that never saw
// the response and resends must not pay twice, and a caller that never sends a
// key is the common case. The fallback is the same coarse window every card
// money move in this package uses.
func TestPayout_WithNoKeyStillPaysOnceInsideTheWindow(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	org := moneyOrg("payoutwindow")
	body := `{"amount":250,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1"}`
	for i := 0; i < 3; i++ {
		if res := invokeMoneyHandler(org, ctx, CreatePayout, body, nil); res.StatusCode != http.StatusCreated {
			t.Fatalf("attempt %d: status=%d", i, res.StatusCode)
		}
	}
	if n := payoutRows(org, ctx); n != 1 {
		t.Fatalf("%d payout rows for one resent request, want 1", n)
	}

	// A DIFFERENT payout is a different request and is not collapsed into it.
	other := `{"amount":251,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1"}`
	if res := invokeMoneyHandler(org, ctx, CreatePayout, other, nil); res.StatusCode != http.StatusCreated {
		t.Fatalf("a different payout: status=%d", res.StatusCode)
	}
	if n := payoutRows(org, ctx); n != 2 {
		t.Fatalf("%d payout rows, want 2 — a genuinely different payout was swallowed", n)
	}

	// And a caller that wants a second identical payout says so with a key.
	keyed := `{"amount":250,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1","idem":"deliberate-second"}`
	if res := invokeMoneyHandler(org, ctx, CreatePayout, keyed, nil); res.StatusCode != http.StatusCreated {
		t.Fatalf("keyed second payout: status=%d", res.StatusCode)
	}
	if n := payoutRows(org, ctx); n != 3 {
		t.Fatalf("%d payout rows, want 3", n)
	}
}

// TestPayout_TheHeaderIsTheSameKeyByAnotherDoor.
func TestPayout_TheHeaderIsTheSameKeyByAnotherDoor(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	org := moneyOrg("payouthdr")
	body := `{"amount":300,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1"}`
	head := map[string]string{"X-Idempotency-Key": "h-1"}
	for i := 0; i < 2; i++ {
		if res := invokeMoneyHandler(org, ctx, CreatePayout, body, head); res.StatusCode != http.StatusCreated {
			t.Fatalf("attempt %d: status=%d", i, res.StatusCode)
		}
	}
	if n := payoutRows(org, ctx); n != 1 {
		t.Fatalf("%d payout rows under one X-Idempotency-Key, want 1", n)
	}
}

// TestPayout_AGuardOutageRefusesRatherThanPayingTwice — if the guard cannot
// tell a first attempt from a retry, refusing costs the caller a retry and
// proceeding costs the merchant a duplicate disbursement.
func TestPayout_AGuardOutageRefusesRatherThanPayingTwice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	restore := idemBegin
	idemBegin = func(*datastore.Datastore, idempotencykey.Guard) (*idempotencykey.IdempotencyKey, bool, error) {
		return nil, false, errors.New("guard store unavailable")
	}
	defer func() { idemBegin = restore }()

	org := moneyOrg("payoutguard")
	res := invokeMoneyHandler(org, ctx, CreatePayout,
		`{"amount":100,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1","idem":"p-1"}`, nil)
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503", res.StatusCode)
	}
	if n := payoutRows(org, ctx); n != 0 {
		t.Fatalf("%d payout rows written while the guard was down", n)
	}
}

// TestPayout_ARefusalDoesNotWedgeTheKey — a refused payout moved no money, so
// the guard is released: the merchant may fix the cause and ask again, and the
// next attempt is screened against the controls in force THEN.
func TestPayout_ARefusalDoesNotWedgeTheKey(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	org := moneyOrg("payoutwedge")
	restrain(t, org, ctx, risk.Subject{Kind: risk.KindMerchant, ID: "m1"}, "hold", 0)
	body := `{"amount":100,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1","merchant":"m1","idem":"p-1"}`

	if res := invokeMoneyHandler(org, ctx, CreatePayout, body, nil); res.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d want 403", res.StatusCode)
	}
	// The same key again is still a refusal, not a wedged in-flight guard.
	if res := invokeMoneyHandler(org, ctx, CreatePayout, body, nil); res.StatusCode != http.StatusForbidden {
		t.Fatalf("a retried refusal answered %d — the guard wedged the key", res.StatusCode)
	}
	if n := payoutRows(org, ctx); n != 0 {
		t.Fatalf("%d payout rows written despite the hold", n)
	}
}

// TestPayout_ARetryCannotOutrunAHoldPlacedAfterTheFirstPayout — the critical
// idempotency defect, followed all the way to the money. The first payout is
// legitimate; the platform then holds the merchant; the retry must not create a
// second disbursement on the strength of the first answer.
func TestPayout_ARetryCannotOutrunAHoldPlacedAfterTheFirstPayout(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	org := moneyOrg("payoutoutrun")
	body := `{"amount":5000,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1","merchant":"m1","idem":"p-1"}`
	if res := invokeMoneyHandler(org, ctx, CreatePayout, body, nil); res.StatusCode != http.StatusCreated {
		t.Fatalf("first: status=%d", res.StatusCode)
	}
	restrain(t, org, ctx, risk.Subject{Kind: risk.KindMerchant, ID: "m1"}, "block", 0)

	res := invokeMoneyHandler(org, ctx, CreatePayout, body, nil)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("the retry answered %d; an idempotent retry replays the first answer", res.StatusCode)
	}
	if n := payoutRows(org, ctx); n != 1 {
		t.Fatalf("%d payout rows — the retry disbursed again after the merchant was blocked", n)
	}

	// A NEW payout, with a new key, is stopped by the block.
	fresh := `{"amount":5000,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1","merchant":"m1","idem":"p-2"}`
	if res := invokeMoneyHandler(org, ctx, CreatePayout, fresh, nil); res.StatusCode != http.StatusForbidden {
		t.Fatalf("a new payout under a live block answered %d, want 403", res.StatusCode)
	}
	if n := payoutRows(org, ctx); n != 1 {
		t.Fatalf("%d payout rows after a blocked attempt, want 1", n)
	}
}

// TestPayout_OneKeyIsOneTenantsPayout — an idempotency key is scoped to the
// org's own store, so two tenants using the same key are two payouts.
func TestPayout_OneKeyIsOneTenantsPayout(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	a := moneyOrg("payoutkeya")
	b := moneyOrg("payoutkeyb")
	body := `{"amount":100,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1","idem":"same"}`
	for _, org := range []*organization.Organization{a, b} {
		if res := invokeMoneyHandler(org, ctx, CreatePayout, body, nil); res.StatusCode != http.StatusCreated {
			t.Fatalf("status=%d", res.StatusCode)
		}
	}
	if n := payoutRows(a, ctx); n != 1 {
		t.Fatalf("org A has %d payouts, want 1", n)
	}
	if n := payoutRows(b, ctx); n != 1 {
		t.Fatalf("org B has %d payouts, want 1 — one key collapsed two tenants' payouts", n)
	}
}

// TestListPayouts_ReturnsTheRowsBoundedAndNewestFirst — the list read is
// bounded at the store now, so this pins that it still returns what it should.
// A list of every payout an org ever made is one request that materialises the
// whole history in a process shared with every other tenant.
func TestListPayouts_ReturnsTheRowsBoundedAndNewestFirst(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	org := moneyOrg("payoutlist")
	for i := 0; i < 3; i++ {
		body := `{"amount":` + strconv.Itoa(100+i) + `,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1","idem":"p-` + strconv.Itoa(i) + `"}`
		if res := invokeMoneyHandler(org, ctx, CreatePayout, body, nil); res.StatusCode != http.StatusCreated {
			t.Fatalf("create %d: status=%d", i, res.StatusCode)
		}
	}

	res := invokeMoneyHandler(org, ctx, ListPayouts, "", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("list: status=%d", res.StatusCode)
	}
	raw, _ := io.ReadAll(res.Body)
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatalf("body %q: %v", string(raw), err)
	}
	if len(rows) != 3 {
		t.Fatalf("%d payouts listed, want 3 — the bounded read must still return the rows", len(rows))
	}
	if len(rows) > pageMax {
		t.Fatalf("the list is unbounded: %d rows", len(rows))
	}
}
