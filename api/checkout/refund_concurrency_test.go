// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

//go:build !race

// This full-handler concurrency test is excluded under -race ON PURPOSE. It
// drives the REAL Refund handler, whose success path fires an ASYNC o11y counter
// (counter.IncrOrderRefund → delay.executeTask spawns a fire-and-forget
// goroutine) that escapes the per-order lock and logs via the package-global
// log.std singleton. log.std stores per-call context/requestURI/error on a
// SHARED backend (log/logger.go), so any two concurrent log calls race — a
// SYSTEMIC, pre-existing infra race that has nothing to do with the money
// invariant under test. The money-critical property (the check-then-write on
// ord.Refunded is atomic per order and data-race-clean) is proven under -race by
// TestRefund_OverRefund_RaceClean, which drives the SAME refundLockFor primitive
// + reload + guard + commit without the logging/o11y noise. This test is the
// FUNCTIONAL regression guard: it fails (ok==2) if the reload-in-lock is
// reverted.

package checkout

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/types/accounts"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestRefund_Concurrent_DistinctAmounts_NoOverRefund proves the HIGH-3 fix
// end-to-end through the real handler: two concurrent refunds of DISTINCT
// amounts on the same order can never exceed the captured total. The handler
// reloads the order INSIDE the per-order lock, so the second refund evaluates
// square.Refund's over-refund guard against the FIRST refund's COMMITTED
// Refunded — not a stale pre-lock snapshot. On the pre-fix code both refunds
// evaluated the guard against Refunded=0 and both passed (3000 + 3000 on a 5000
// order = 6000 refunded), so BOTH returned 200 — which this test catches as
// ok==2.
func TestRefund_Concurrent_DistinctAmounts_NoOverRefund(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	const ns = "acme"
	base := nscontext.WithNamespace(context.Background(), ns)
	db := datastore.New(base)

	// Order captured for 5000 via one TEST Square payment (Test=true ⇒
	// square.Refund never touches the real gateway).
	ord := order.New(db)
	ord.Total = 5000
	ord.Paid = 5000
	ord.Type = accounts.SquareType
	if err := ord.Create(); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	// Resolve the id ONCE up front. order.Id() lazily materializes the key
	// (updateId writes), so calling it from both goroutines would itself race —
	// on the TEST's shared object, not the code under test.
	orderId := ord.Id()

	pay := payment.New(db)
	pay.Type = accounts.SquareType
	pay.Amount = 5000
	pay.Currency = "usd"
	pay.Test = true
	pay.Account.Type = accounts.SquareType
	pay.Account.Square.PaymentId = "pmt_test_1"
	// Parent the payment to the order — the canonical link (see initPayment in
	// types.go) that ord.GetPayments()'s ancestor query resolves.
	pay.Parent = ord.Key()
	pay.OrderId = orderId
	if err := pay.Create(); err != nil {
		t.Fatalf("seed payment: %v", err)
	}

	// Sanity: the order must see its payment via the ancestor query, else the
	// refund path can't run and the test would be vacuous.
	if got, err := ord.GetPayments(); err != nil || len(got) != 1 {
		t.Fatalf("GetPayments = %d, err=%v; test needs exactly one seeded payment", len(got), err)
	}

	org := &organization.Organization{}
	org.Name = ns

	// The real Refund handler behind the same locals-injector the idempotency
	// test uses; fiber populates :orderid from the route.
	app := refundApp(org, base, ns)

	// Fire a refund via the real handler as an admin (money move).
	fire := func(amount int64, idem string) int {
		body, _ := json.Marshal(map[string]any{"amount": amount})
		req := httptest.NewRequest(http.MethodPost, "/v1/order/"+orderId+"/refund", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", idem)
		resp, err := app.Test(req)
		if err != nil {
			t.Errorf("refund request: %v", err)
			return 0
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	// Two concurrent 3000 refunds on a 5000 order, DISTINCT idempotency keys so
	// they do NOT collapse on the replay guard — they race on the over-refund
	// guard, exactly what the reload-in-lock protects.
	var wg sync.WaitGroup
	wg.Add(2)
	codes := make([]int, 2)
	go func() { defer wg.Done(); codes[0] = fire(3000, "refund_a") }()
	go func() { defer wg.Done(); codes[1] = fire(3000, "refund_b") }()
	wg.Wait()

	ok, rejected := 0, 0
	for _, code := range codes {
		switch code {
		case http.StatusOK:
			ok++
		case http.StatusBadRequest:
			rejected++
		default:
			t.Fatalf("unexpected refund status %d (want 200 or 400); codes=%v", code, codes)
		}
	}
	if ok != 1 || rejected != 1 {
		t.Fatalf("concurrent refunds: %d ok, %d rejected — want exactly 1 ok + 1 rejected (over-refund not prevented!); codes=%v", ok, rejected, codes)
	}

	// The invariant: total refunded never exceeds the captured total, and exactly
	// one 3000 refund committed.
	final := order.New(db)
	if err := final.GetById(orderId); err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if int64(final.Refunded) > 5000 {
		t.Fatalf("OVER-REFUND: order Refunded=%d exceeds captured total 5000", final.Refunded)
	}
	if int64(final.Refunded) != 3000 {
		t.Fatalf("expected exactly one 3000 refund committed, got Refunded=%d", final.Refunded)
	}
}
