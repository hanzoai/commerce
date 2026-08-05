package billing

import (
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestDeposit_OverCeiling_400 — H1: a single deposit is bounded by the
// server-authoritative ceiling (COMMERCE_DEPOSIT_MAX_CENTS). No unbounded mint in
// one request, even for a trusted caller.
func TestDeposit_OverCeiling_400(t *testing.T) {
	t.Setenv("COMMERCE_DEPOSIT_MAX_CENTS", "1000")
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("h1d-ceiling")

	body := `{"user":"h1d-ceiling/alice","amount":2000}` // over the 1000 ceiling
	resp := invokeMoneyHandler(org, ctx, Deposit, body, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("over-ceiling deposit: status=%d body=%s, want 400", resp.StatusCode, func() string { b, _ := io.ReadAll(resp.Body); return string(b) }())
	}
}

// TestDeposit_UnderCeiling_201 — control: a deposit at/under the ceiling still mints.
func TestDeposit_UnderCeiling_201(t *testing.T) {
	t.Setenv("COMMERCE_DEPOSIT_MAX_CENTS", "1000")
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("h1d-under")

	body := `{"user":"h1d-under/alice","amount":1000}`
	resp := invokeMoneyHandler(org, ctx, Deposit, body, map[string]string{"X-Idempotency-Key": "settlement-under"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("at-ceiling deposit: status=%d body=%s, want 201", resp.StatusCode, func() string { b, _ := io.ReadAll(resp.Body); return string(b) }())
	}
}

// TestDeposit_IdempotencyKey_SingleEffect — H1: a retry/double-submit carrying the
// SAME X-Idempotency-Key credits AT MOST ONCE; the replay returns the first
// result (same transactionId), not a second credit.
func TestDeposit_IdempotencyKey_SingleEffect(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("h1d-idem")

	body := `{"user":"h1d-idem/alice","amount":500}`
	hdr := map[string]string{"X-Idempotency-Key": "dep-key-1"}

	w1 := invokeMoneyHandler(org, ctx, Deposit, body, hdr)
	if w1.StatusCode != http.StatusCreated {
		t.Fatalf("first deposit: status=%d body=%s, want 201", w1.StatusCode, func() string { b, _ := io.ReadAll(w1.Body); return string(b) }())
	}
	tid1 := txID(t, w1)
	if tid1 == "" {
		t.Fatal("first deposit missing transactionId")
	}

	w2 := invokeMoneyHandler(org, ctx, Deposit, body, hdr)
	if w2.StatusCode != http.StatusOK {
		t.Fatalf("replay deposit: status=%d body=%s, want 200 (idempotent replay)", w2.StatusCode, func() string { b, _ := io.ReadAll(w2.Body); return string(b) }())
	}
	if tid2 := txID(t, w2); tid2 != tid1 {
		t.Fatalf("replay minted a NEW transaction %q (first was %q) — double credit", tid2, tid1)
	}
}

// TestDeposit_NoKey_Refused — money-in must NAME the event that caused it.
//
// This replaces an earlier control that asserted keyless deposits are additive.
// That contract cannot be held safely: a retried settlement webhook and a second
// genuine settlement of the same amount are the same request, so "additive"
// funds a wallet twice on every processor retry, and the obvious alternative
// (dedupe on a time window) collapses two real payments into one credit. The
// endpoint refuses rather than picking which way to be wrong.
func TestDeposit_NoKey_Refused(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("h1d-nokey")

	body := `{"user":"h1d-nokey/alice","amount":500}`
	resp := invokeMoneyHandler(org, ctx, Deposit, body, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("keyless deposit: status=%d body=%s, want 400 — an unnamed credit is a replay waiting to happen",
			resp.StatusCode, func() string { b, _ := io.ReadAll(resp.Body); return string(b) }())
	}
}

// TestDeposit_DistinctKeys_DistinctDeposits — the control the refusal must not
// break: two REAL settlements of the same amount, each naming its own event,
// both credit. Requiring a key must not turn into deduping by amount.
func TestDeposit_DistinctKeys_DistinctDeposits(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("h1d-distinct")

	body := `{"user":"h1d-distinct/alice","amount":500}`
	w1 := invokeMoneyHandler(org, ctx, Deposit, body, map[string]string{"X-Idempotency-Key": "settlement-a"})
	w2 := invokeMoneyHandler(org, ctx, Deposit, body, map[string]string{"X-Idempotency-Key": "settlement-b"})
	if w1.StatusCode != http.StatusCreated || w2.StatusCode != http.StatusCreated {
		t.Fatalf("two distinct settlements: statuses=%d,%d, want 201,201", w1.StatusCode, w2.StatusCode)
	}
	if txID(t, w1) == txID(t, w2) {
		t.Fatal("two distinct settlements collapsed to one transaction — over-deduped, a customer paid twice and was credited once")
	}
}

// TestDeposit_GuardUnavailable_FailsClosed — when the guard store cannot say
// whether this is a first attempt or a retry, the credit does NOT land. Unknown
// is the one case where crediting anyway turns a store blip into a duplicate
// mint, so it is a 503 the caller can safely retry with the same key.
func TestDeposit_GuardUnavailable_FailsClosed(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("h1d-guarddown")

	orig := idemBegin
	idemBegin = func(*datastore.Datastore, string, string) (*idempotencykey.IdempotencyKey, bool, error) {
		return nil, false, errors.New("guard store unavailable")
	}
	t.Cleanup(func() { idemBegin = orig })

	body := `{"user":"h1d-guarddown/alice","amount":500}`
	resp := invokeMoneyHandler(org, ctx, Deposit, body, map[string]string{"X-Idempotency-Key": "settlement-x"})
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("guard-unavailable deposit: status=%d body=%s, want 503 — an unverifiable credit must not land",
			resp.StatusCode, func() string { b, _ := io.ReadAll(resp.Body); return string(b) }())
	}
}
