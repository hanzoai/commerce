// Copyright © 2026 Hanzo AI. MIT License.

package billing

// risk_payout_race_test.go is the gate on duplicate disbursement.
//
// A payout is money leaving with no natural backstop: no nonce is consumed and
// no card refuses the second one. Everything here asserts the same sentence —
// ONE IDEMPOTENCY KEY IS ONE PAYOUT — under the conditions that actually break
// it: two clicks at once, a client that resends, and a key reused by mistake.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/risk"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestPayout_ConcurrentRequestsUnderOneKeyPayOnce — the guard is a CLAIM, so
// exactly one of N simultaneous posts performs the disbursement and the rest
// are told the key is taken. A read-then-write guard lets all N through: one
// guard row in the store and N payouts in the world.
func TestPayout_ConcurrentRequestsUnderOneKeyPayOnce(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	org := moneyOrg("payoutrace")
	body := `{"amount":100000,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1","idem":"one-key"}`

	const n = 6
	var wg sync.WaitGroup
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = invokeMoneyHandler(org, ctx, CreatePayout, body, nil).StatusCode
		}(i)
	}
	wg.Wait()

	created := 0
	for _, c := range codes {
		if c == http.StatusCreated {
			created++
		}
	}
	rows := payoutRows(org, ctx)
	if rows != 1 {
		t.Fatalf("DUPLICATE DISBURSEMENT: %d payout rows for one idempotency key (codes %v)", rows, codes)
	}
	if created < 1 {
		t.Fatalf("no caller was told the payout was created (codes %v)", codes)
	}
	for _, c := range codes {
		if c != http.StatusCreated && c != http.StatusConflict {
			t.Fatalf("a losing caller got %d; want 201 (replayed) or 409 (in flight) — codes %v", c, codes)
		}
	}
}

// TestPayout_ASequentialRetryUnderOneKeyReplaysTheReceipt — the other half: the
// key must not merely refuse the second attempt, it must give back the first
// answer, or a client that lost the response has no way to learn what happened.
func TestPayout_ASequentialRetryUnderOneKeyReplaysTheReceipt(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	org := moneyOrg("payoutreplay")
	body := `{"amount":700,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1","idem":"retry-key"}`

	first := decode(t, invokeMoneyHandler(org, ctx, CreatePayout, body, nil))
	second := decode(t, invokeMoneyHandler(org, ctx, CreatePayout, body, nil))
	if first["id"] != second["id"] || first["amount"] != second["amount"] {
		t.Fatalf("a retry got a different receipt: %v then %v", first, second)
	}
	if n := payoutRows(org, ctx); n != 1 {
		t.Fatalf("%d payout rows after one request and its retry", n)
	}
}

// TestPayout_OneKeyForADifferentMoveIsRefused — a key names ONE payout. Told
// "created" with the first payout's receipt, a caller that asked for $10,000
// believes a $10,000 disbursement exists. The screen door already answers 409
// for exactly this mistake; the payout door must answer the same way.
func TestPayout_OneKeyForADifferentMoveIsRefused(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	org := moneyOrg("payoutmismatch")
	small := `{"amount":100,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1","idem":"same-key"}`
	large := `{"amount":1000000,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1","idem":"same-key"}`

	first := decode(t, invokeMoneyHandler(org, ctx, CreatePayout, small, nil))
	res := invokeMoneyHandler(org, ctx, CreatePayout, large, nil)
	if res.StatusCode != http.StatusConflict {
		second := decode(t, res)
		t.Fatalf("asked for 1000000 under a used key and got %d id=%v amount=%v (first was %v)",
			res.StatusCode, second["id"], second["amount"], first["amount"])
	}
	if n := payoutRows(org, ctx); n != 1 {
		t.Fatalf("%d payout rows after a refused key reuse", n)
	}
}

// TestPayout_ACancelledPayoutStaysInTheList — this repository documents the ORM
// property that causes the loss (models/screen.Query: "a row read by id and
// written back comes out of that group"), and CancelPayout is a read by id and
// a write, in the same file as the list. A merchant's own cancelled payout must
// not disappear from its own history.
func TestPayout_ACancelledPayoutStaysInTheList(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	risk.Set(answers{})

	org := moneyOrg("payoutvanish")
	created := decode(t, invokeMoneyHandler(org, ctx, CreatePayout,
		`{"amount":700,"currency":"usd","destinationType":"bank_account","destinationId":"ba_1"}`, nil))
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("no payout created: %v", created)
	}

	before := listPayouts(t, org, ctx)

	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Post("/v1/billing/payouts/:id/cancel", func(c *zip.Ctx) error {
		c.Locals("organization", org)
		c.SetContext(ctx)
		return c.Next()
	}, CancelPayout)
	res, err := app.Fiber().Test(httptest.NewRequest(http.MethodPost, "/v1/billing/payouts/"+id+"/cancel", nil))
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("cancel → %d", res.StatusCode)
	}

	after := listPayouts(t, org, ctx)
	if len(after) != len(before) {
		t.Fatalf("the cancelled payout vanished from GET /v1/billing/payouts (%d → %d)", len(before), len(after))
	}
	for _, p := range after {
		if p["id"] == id && p["status"] != "canceled" {
			t.Fatalf("the payout is listed with status %v after being cancelled", p["status"])
		}
	}
}

// listPayouts drives the real list handler and decodes the page.
func listPayouts(t *testing.T, org *organization.Organization, ctx context.Context) []map[string]any {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Get("/v1/billing/payouts", func(c *zip.Ctx) error {
		c.Locals("organization", org)
		c.SetContext(ctx)
		return c.Next()
	}, ListPayouts)
	res, err := app.Fiber().Test(httptest.NewRequest(http.MethodGet, "/v1/billing/payouts", nil))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	raw, _ := io.ReadAll(res.Body)
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("list body %q: %v", string(raw), err)
	}
	return out
}
