package billing

// credit_ledger_test.go proves the INJECTED-ledger path (commerce embedded in the
// cloud binary): POST /v1/billing/credit routes to the host-injected CreditLedger,
// GET balance reads it, they key the SAME org account (account-consistency
// invariant), and the mint-gate still 403s a non-admin — all with a fake ledger.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/util/test/ae"
)

// fakeLedger is an in-memory CreditLedger keyed by the WHOLE address a credit names
// — (org, subject, currency, test) — because that is how the real one is keyed and a
// fake that folded any of them away would pass a test the ledger fails. The host
// holds sandbox money in a separate ledger per org and dedups an idempotency key
// WITHIN one ledger, so `test` is part of the address AND part of the dedup scope.
//
// It records the LAST CreditInput and every credit it took, so a test can assert the
// handler passed exactly the right fields and nothing credited twice.
type fakeLedger struct {
	mu     sync.Mutex
	lastIn creditledger.CreditInput
	posted []creditledger.CreditInput
	calls  int
	bal    map[string]int64
	seen   map[string]string
	nextID int
}

func newFakeLedger() *fakeLedger {
	return &fakeLedger{bal: map[string]int64{}, seen: map[string]string{}}
}

// account is the address a credit lands at / a balance is read from. An empty
// subject is the org's pool, the SAME default both halves of the real seam apply.
func (f *fakeLedger) account(org, subject, cur string, test bool) string {
	if subject == "" {
		subject = org
	}
	return fmt.Sprintf("%s|%s|%s|%t", org, subject, cur, test)
}

// scope is where an idempotency key is unique: one ledger, which is one (org, test)
// pair. The same key in two different ledgers is two credits — the property that
// makes an unresolved payer dangerous rather than merely wrong.
func (f *fakeLedger) scope(org string, test bool, key string) string {
	return fmt.Sprintf("%s|%t|%s", org, test, key)
}

func (f *fakeLedger) Credit(_ context.Context, in creditledger.CreditInput) (string, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastIn = in
	f.calls++
	k := f.account(in.Org, in.Subject, in.Currency, in.Test)
	if in.IdempotencyKey != "" {
		if id, ok := f.seen[f.scope(in.Org, in.Test, in.IdempotencyKey)]; ok {
			return id, f.bal[k], nil // idempotent replay: no second credit
		}
	}
	f.nextID++
	id := fmt.Sprintf("post_%d", f.nextID)
	f.bal[k] += in.AmountCents
	f.posted = append(f.posted, in)
	if in.IdempotencyKey != "" {
		f.seen[f.scope(in.Org, in.Test, in.IdempotencyKey)] = id
	}
	return id, f.bal[k], nil
}

func (f *fakeLedger) Balance(_ context.Context, org, subject, cur string, test bool) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bal[f.account(org, subject, cur, test)], nil
}

// credits returns the credits actually POSTED (replays excluded), for the tests that
// care about how many times money moved rather than how many calls were made.
func (f *fakeLedger) credits() []creditledger.CreditInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]creditledger.CreditInput(nil), f.posted...)
}

// injectLedger installs a ledger for the test and clears it afterward so it never
// leaks into the datastore-path tests in this package.
func injectLedger(t *testing.T, l creditledger.CreditLedger) {
	t.Helper()
	creditledger.Set(l)
	t.Cleanup(func() { creditledger.Set(nil) })
}

// ledgerEngine builds the test engine holding a reference to the shared test
// datastore for the life of the test.
//
// These tests exercise the INJECTED-ledger path, but a request still resolves its
// org THROUGH the datastore before the handler ever reaches the ledger — so the
// store must be open, or the org lookup fails and the route 503s. ae.NewContext is
// REFERENCE-COUNTED and closes the store when the last holder Closes it, so a test
// that never opens one is not "not using the store": it is free-riding on whichever
// other test file happens to be holding it open, and it breaks the day that file
// changes. Holding the reference here makes each test self-sufficient and
// order-independent.
func ledgerEngine(t *testing.T, seed func(*zip.Ctx)) *zip.App {
	t.Helper()
	ctx := ae.NewContext()
	t.Cleanup(ctx.Close)
	return engineWithSeed(func(c *zip.Ctx) {
		c.SetContext(ctx)
		if seed != nil {
			seed(c)
		}
	})
}

// TestCredit_InjectedLedger_RoutesAndReflects is the account-consistency proof:
// the endpoint calls CreditLedger.Credit with the exact CreditInput, and
// GET /billing/balance?user=<org> reflects it via the SAME org account.
func TestCredit_InjectedLedger_RoutesAndReflects(t *testing.T) {
	const tok = "svc-ledger"
	t.Setenv("COMMERCE_SERVICE_TOKEN", tok)
	fake := newFakeLedger()
	injectLedger(t, fake)

	eng := ledgerEngine(t, nil)
	resp := postCredit(eng, tok, "acme", `{"org":"acme","amountCents":500,"reason":"welcome","tag":"starter-credit","currency":"usd"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("service-token credit (ledger): status=%d body=%s, want 201", resp.StatusCode, respBodyStr(resp))
	}

	// The handler called Credit exactly once with the right CreditInput.
	if fake.calls != 1 {
		t.Fatalf("CreditLedger.Credit calls=%d, want 1", fake.calls)
	}
	got := fake.lastIn
	if got.Org != "acme" || got.Currency != "usd" || got.Reason != "welcome" || got.Tag != "starter-credit" || got.AmountCents != 500 {
		t.Fatalf("CreditInput=%+v, want {Org:acme Currency:usd Reason:welcome Tag:starter-credit AmountCents:500}", got)
	}

	out := decodeJSON(t, resp)
	if id, _ := out["id"].(string); id == "" {
		t.Fatalf("response missing ledger tx id: %v", out)
	}
	if bc, _ := out["balanceCents"].(float64); int64(bc) != 500 {
		t.Fatalf("response balanceCents=%v, want 500", out["balanceCents"])
	}

	// GET balance reads the SAME org account → reflects the credit (one ledger).
	if bal := getBalanceCents(t, eng, tok, "acme"); bal != 500 {
		t.Fatalf("GET balance for acme=%d, want 500 (credit must be visible on the same account)", bal)
	}
}

// TestCredit_InjectedLedger_OrgAdminStill403 proves the mint-gate is unchanged with
// a ledger injected: an org admin is 403 and the ledger is NEVER called (a user can
// still not credit itself, even through the injected ledger).
func TestCredit_InjectedLedger_OrgAdminStill403(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")
	fake := newFakeLedger()
	injectLedger(t, fake)

	eng := ledgerEngine(t, orgAdminSeed)
	resp := postCredit(eng, "", "", `{"org":"acme","amountCents":100000,"reason":"self-credit"}`)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("org-admin credit (ledger injected): status=%d, want 403", resp.StatusCode)
	}
	if fake.calls != 0 {
		t.Fatalf("CreditLedger.Credit was called %d times for a 403'd org admin — the gate must block BEFORE the ledger", fake.calls)
	}
}

// TestCredit_InjectedLedger_Idempotent proves idempotency is honored through the
// interface: same idempotencyKey ⇒ one credit, same tx id, balance not doubled.
func TestCredit_InjectedLedger_Idempotent(t *testing.T) {
	const tok = "svc-ledger-idem"
	t.Setenv("COMMERCE_SERVICE_TOKEN", tok)
	fake := newFakeLedger()
	injectLedger(t, fake)

	eng := ledgerEngine(t, nil)
	body := `{"org":"idem","amountCents":250,"reason":"promo","idempotencyKey":"once"}`

	first := decodeJSON(t, postCredit(eng, tok, "idem", body))
	second := decodeJSON(t, postCredit(eng, tok, "idem", body))
	if first["id"] != second["id"] || first["id"] == "" {
		t.Fatalf("idempotent replay: first id=%v second id=%v, want equal non-empty", first["id"], second["id"])
	}
	if bal := getBalanceCents(t, eng, tok, "idem"); bal != 250 {
		t.Fatalf("after two same-key credits, balance=%d, want 250 (one grant)", bal)
	}
}

// TestCredit_InjectedLedger_MultiCurrency proves currency flows through to the
// ledger and keys a distinct per-currency account.
func TestCredit_InjectedLedger_MultiCurrency(t *testing.T) {
	const tok = "svc-ledger-cur"
	t.Setenv("COMMERCE_SERVICE_TOKEN", tok)
	fake := newFakeLedger()
	injectLedger(t, fake)

	eng := ledgerEngine(t, nil)
	if resp := postCredit(eng, tok, "fx", `{"org":"fx","amountCents":900,"reason":"eur grant","currency":"eur"}`); resp.StatusCode != http.StatusCreated {
		t.Fatalf("eur credit: status=%d body=%s", resp.StatusCode, respBodyStr(resp))
	}
	if fake.lastIn.Currency != "eur" {
		t.Fatalf("CreditInput.Currency=%q, want eur", fake.lastIn.Currency)
	}
	// usd account is untouched; eur holds the grant.
	req := httptest.NewRequest(http.MethodGet, "/v1/billing/balance?user=fx&currency=eur", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Org-Id", "fx")
	resp, _ := eng.Test(req)
	if bc, _ := decodeJSON(t, resp)["balance"].(float64); int64(bc) != 900 {
		t.Fatalf("eur balance=%v, want 900", bc)
	}
}
