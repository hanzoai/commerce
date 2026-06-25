package proxy_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hanzoai/commerce/metering"
	"github.com/hanzoai/commerce/metering/proxy"
)

// fakeCommerce models the commerce billing API: a per-org balance ledger that
// the balance endpoint reads and the usage endpoint debits. This lets the test
// prove a real gate+debit loop in-process, mirroring the live commerce contract
// (X-Hanzo-Org namespacing, /v1/billing/balance, /v1/billing/usage).
type fakeCommerce struct {
	mu       sync.Mutex
	balances map[string]int64 // org -> cents
	debits   []debit
}

type debit struct {
	org      string
	user     string
	cents    int64
	provider string
}

func newFakeCommerce() *fakeCommerce {
	return &fakeCommerce{balances: map[string]int64{}}
}

func (f *fakeCommerce) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		org := r.Header.Get("X-Hanzo-Org")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/billing/balance":
			f.mu.Lock()
			avail := f.balances[org]
			f.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user": org, "currency": "usd", "balance": avail, "holds": 0, "available": avail,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/billing/usage":
			var body struct {
				User     string `json:"user"`
				Amount   int64  `json:"amount"`
				Provider string `json:"provider"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			f.balances[org] -= body.Amount
			f.debits = append(f.debits, debit{org: org, user: body.User, cents: body.Amount, provider: body.Provider})
			f.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{
				"transactionId": "tx_test", "user": body.User, "amount": body.Amount, "type": "withdraw",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func (f *fakeCommerce) balance(org string) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.balances[org]
}

func (f *fakeCommerce) debitCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.debits)
}

// buildProxy wires a meter (pointed at the fake commerce) + the fake upstream
// into a metering proxy with a vector-style price table.
func buildProxy(t *testing.T, commerceURL, upstreamURL string) http.Handler {
	t.Helper()
	meter, err := metering.New(metering.Config{BaseURL: commerceURL, Token: "svc", Org: "hanzo"})
	if err != nil {
		t.Fatalf("metering.New: %v", err)
	}
	h, err := proxy.New(proxy.Config{
		Upstream:  upstreamURL,
		Provider:  "vector",
		Prices:    proxy.ParsePriceTable("POST|/collections|2 ; *|/collections|1 ; default:0"),
		SkipPaths: []string{"/healthz"},
		Meter:     meter,
	})
	if err != nil {
		t.Fatalf("proxy.New: %v", err)
	}
	return h
}

// waitDebits polls until the async usage record lands (Record runs in a
// goroutine), so the test is deterministic without sleeping a fixed time.
func waitDebits(f *fakeCommerce, want int) {
	for i := 0; i < 200; i++ {
		if f.debitCount() >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestProxy_GatesForwardsAndDebitsPerOrg(t *testing.T) {
	commerce := newFakeCommerce()
	commerce.balances["acme"] = 100 // acme org funded with 100c

	var upstreamHits int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits++
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"result":"ok"}`)
	}))
	defer upstream.Close()

	csrv := httptest.NewServer(commerce.handler())
	defer csrv.Close()

	h := buildProxy(t, csrv.URL, upstream.URL)
	psrv := httptest.NewServer(h)
	defer psrv.Close()

	// A funded org's vector upsert: gate passes -> upstream served -> 2c debit.
	req, _ := http.NewRequest(http.MethodPost, psrv.URL+"/collections/docs/points",
		strings.NewReader(`{"points":[]}`))
	req.Header.Set("X-Org-Id", "acme")
	req.Header.Set("X-User-Id", "alice")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (gate should pass for funded org)", resp.StatusCode)
	}
	resp.Body.Close()

	if upstreamHits != 1 {
		t.Fatalf("upstream hits = %d, want 1 (request must forward)", upstreamHits)
	}

	waitDebits(commerce, 1)
	if got := commerce.debitCount(); got != 1 {
		t.Fatalf("debits = %d, want 1", got)
	}
	// 100c funded - 2c upsert = 98c, debited against ACME (per-org).
	if got := commerce.balance("acme"); got != 98 {
		t.Errorf("acme balance = %d, want 98 (100 - 2c upsert)", got)
	}
	// Per-org isolation: a different org's ledger was never touched.
	if got := commerce.balance("hanzo"); got != 0 {
		t.Errorf("hanzo balance = %d, want 0 (untouched — debit is per-org)", got)
	}
	commerce.mu.Lock()
	d := commerce.debits[0]
	commerce.mu.Unlock()
	// Per-org billing: the debit user IS the org slug (not org/sub), so it hits
	// the same ledger the gate checked.
	if d.org != "acme" || d.user != "acme" || d.cents != 2 || d.provider != "vector" {
		t.Errorf("debit = %+v, want {org:acme user:acme cents:2 provider:vector}", d)
	}
}

func TestProxy_DeniesUnfundedOrg_402_NoUpstream(t *testing.T) {
	commerce := newFakeCommerce() // no balances -> everyone is at 0

	var upstreamHits int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits++
	}))
	defer upstream.Close()
	csrv := httptest.NewServer(commerce.handler())
	defer csrv.Close()

	psrv := httptest.NewServer(buildProxy(t, csrv.URL, upstream.URL))
	defer psrv.Close()

	req, _ := http.NewRequest(http.MethodPost, psrv.URL+"/collections/docs/points", nil)
	req.Header.Set("X-Org-Id", "broke")
	req.Header.Set("X-User-Id", "bob")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	// Fail-closed: zero balance -> 402, and the upstream is NEVER reached.
	if resp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402 (insufficient balance)", resp.StatusCode)
	}
	if upstreamHits != 0 {
		t.Errorf("upstream hits = %d, want 0 (denied request must not reach the product)", upstreamHits)
	}
}

func TestProxy_SkipPath_NoGate_NoCharge(t *testing.T) {
	commerce := newFakeCommerce() // unfunded — would deny if gated
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer upstream.Close()
	csrv := httptest.NewServer(commerce.handler())
	defer csrv.Close()

	psrv := httptest.NewServer(buildProxy(t, csrv.URL, upstream.URL))
	defer psrv.Close()

	// Health check bypasses metering: served even with no balance, no debit.
	resp, err := http.Get(psrv.URL + "/healthz")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200 (skipped path)", resp.StatusCode)
	}
	if got := commerce.debitCount(); got != 0 {
		t.Errorf("debits = %d, want 0 (skip path is never charged)", got)
	}
}

func TestProxy_UpstreamDown_502(t *testing.T) {
	commerce := newFakeCommerce()
	commerce.balances["acme"] = 100
	csrv := httptest.NewServer(commerce.handler())
	defer csrv.Close()

	// Upstream that is immediately closed -> dial fails.
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	psrv := httptest.NewServer(buildProxy(t, csrv.URL, deadURL))
	defer psrv.Close()

	req, _ := http.NewRequest(http.MethodGet, psrv.URL+"/collections/docs", nil)
	req.Header.Set("X-Org-Id", "acme")
	req.Header.Set("X-User-Id", "alice")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	// Gate passed (funded), but upstream is down -> 502, distinct from 402/503.
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 (upstream down, billing OK)", resp.StatusCode)
	}
}
