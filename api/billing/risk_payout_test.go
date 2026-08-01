// Copyright © 2026 Hanzo AI. MIT License.

package billing

// risk_payout_test.go proves the enforcement point: a control does not merely
// exist in a table, it stops a real payout on the real handler.

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
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
	if _, err := risk.Place(s, subject, effect, rate, time.Time{}, "test"); err != nil {
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

// TestPayout_TheSameIdemKeyScreensOnce — a retried payout is judged once, so
// two answers to one question cannot disagree.
func TestPayout_TheSameIdemKeyScreensOnce(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	org := moneyOrg("payoutidem")
	body := `{"amount":100,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1","idem":"p-1"}`
	for i := 0; i < 2; i++ {
		if res := invokeMoneyHandler(org, ctx, CreatePayout, body, nil); res.StatusCode != http.StatusCreated {
			t.Fatalf("attempt %d: status=%d", i, res.StatusCode)
		}
	}
	if n := len(screen.For(datastore.New(org.Namespaced(ctx)), "", "", 0)); n != 1 {
		t.Fatalf("%d screens for one idempotency key, want 1", n)
	}
}
