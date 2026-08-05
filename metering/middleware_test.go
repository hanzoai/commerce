package metering_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/hanzoai/account"
	"github.com/hanzoai/commerce/metering"
)

// commerceStub serves balance (GET) and records usage (POST), capturing the
// recorded amount so the test can assert post-request metering happened.
type commerceStub struct {
	available int64

	mu          sync.Mutex
	recordedAmt int64
	recordedCnt int
	recordedUsr string
}

func (s *commerceStub) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			s.mu.Lock()
			defer s.mu.Unlock()
			body, _ := io.ReadAll(r.Body)
			// crude parse: amount + user
			s.recordedCnt++
			s.recordedAmt = extractInt(body, `"amount":`)
			s.recordedUsr = extractStr(body, `"user":"`)
			w.WriteHeader(201)
			_, _ = io.WriteString(w, `{"transactionId":"tx","type":"withdraw"}`)
			return
		}
		// balance
		_, _ = io.WriteString(w, `{"available":`+itoa(s.available)+`}`)
	}))
}

func (s *commerceStub) records() (int, int64, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recordedCnt, s.recordedAmt, s.recordedUsr
}

func gatewayReq() *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/search?q=foo", nil)
	r.Header.Set(metering.HeaderOrgID, "hanzo")
	r.Header.Set(metering.HeaderUserID, "alice")
	return r
}

func TestMiddleware_GatesAndRecords(t *testing.T) {
	stub := &commerceStub{available: 5000}
	srv := stub.server()
	defer srv.Close()

	c, _ := metering.New(metering.Config{BaseURL: srv.URL, Token: "t", Org: "hanzo"})

	handlerHit := false
	h := c.Middleware(metering.MiddlewareConfig{
		Provider: "search",
		Price: func(_ *http.Request, status int, _ metering.AuthInput) int64 {
			if status == http.StatusOK {
				return 7 // 7 cents per successful search
			}
			return 0
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerHit = true
		w.WriteHeader(200)
		_, _ = w.Write([]byte("results"))
	}))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, gatewayReq())

	if !handlerHit {
		t.Fatal("handler should have run (balance positive)")
	}
	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	// Record happens async; poll briefly.
	waitFor(t, func() bool { n, _, _ := stub.records(); return n == 1 })
	n, amt, usr := stub.records()
	if n != 1 || amt != 7 {
		t.Fatalf("recorded (count=%d amount=%d), want (1, 7)", n, amt)
	}
	// gatewayReq is a person in the SHARED SIGNUP org, who holds their own account:
	// its members are strangers, not a team, so a shared org is not a shared wallet.
	// This asserted "hanzo" — the org pool — while ai's gate debited "hanzo/alice",
	// which is the funded-pool-then-402 split. The account is whatever the one rule
	// says, so assert against the rule rather than restating a premise it disproved.
	want := account.Payer(account.Credential{Owner: "hanzo", Name: "alice"}).Subject()
	if usr != want {
		t.Errorf("recorded user = %q, want %q (the account the shared rule resolves)", usr, want)
	}
	if usr == "hanzo" {
		t.Errorf("recorded user = %q — the org pool, but this caller's usage is debited from their own account", usr)
	}
}

func TestMiddleware_Denies402_WhenNoBalance(t *testing.T) {
	stub := &commerceStub{available: 0}
	srv := stub.server()
	defer srv.Close()

	c, _ := metering.New(metering.Config{BaseURL: srv.URL, Token: "t", Org: "hanzo"})
	handlerHit := false
	h := c.Middleware(metering.MiddlewareConfig{
		Provider: "search",
		Price:    func(*http.Request, int, metering.AuthInput) int64 { return 7 },
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { handlerHit = true }))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, gatewayReq())

	if handlerHit {
		t.Fatal("handler must NOT run when balance is zero")
	}
	if rr.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402", rr.Code)
	}
	if n, _, _ := stub.records(); n != 0 {
		t.Fatalf("denied request must not record usage, got %d records", n)
	}
}

func TestMiddleware_FailClosed503_WhenCommerceDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	c, _ := metering.New(metering.Config{BaseURL: srv.URL, Token: "t", Org: "hanzo"})
	h := c.Middleware(metering.MiddlewareConfig{
		Provider: "search",
		Price:    func(*http.Request, int, metering.AuthInput) int64 { return 7 },
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not run when balance is unknown (fail-closed)")
	}))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, gatewayReq())
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (fail-closed)", rr.Code)
	}
}

func TestMiddleware_Skip_Bypasses(t *testing.T) {
	stub := &commerceStub{available: 0} // would deny if gated
	srv := stub.server()
	defer srv.Close()

	c, _ := metering.New(metering.Config{BaseURL: srv.URL, Token: "t", Org: "hanzo"})
	handlerHit := false
	h := c.Middleware(metering.MiddlewareConfig{
		Provider: "search",
		Price:    func(*http.Request, int, metering.AuthInput) int64 { return 7 },
		Skip:     func(r *http.Request) bool { return r.URL.Path == "/healthz" },
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerHit = true
		w.WriteHeader(200)
	}))

	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.ServeHTTP(rr, r)

	if !handlerHit {
		t.Fatal("skipped path must run handler without gating")
	}
	if n, _, _ := stub.records(); n != 0 {
		t.Fatal("skipped path must not record usage")
	}
}

func TestMiddleware_OnlyChargesSuccess(t *testing.T) {
	stub := &commerceStub{available: 5000}
	srv := stub.server()
	defer srv.Close()

	c, _ := metering.New(metering.Config{BaseURL: srv.URL, Token: "t", Org: "hanzo"})
	h := c.Middleware(metering.MiddlewareConfig{
		Provider: "search",
		Price: func(_ *http.Request, status int, _ metering.AuthInput) int64 {
			if status == http.StatusOK {
				return 7
			}
			return 0 // don't charge for failures
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // handler failed
	}))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, gatewayReq())
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
	// Give any (erroneous) async record a chance, then assert none happened.
	time.Sleep(50 * time.Millisecond)
	if n, _, _ := stub.records(); n != 0 {
		t.Fatalf("failed request must not be charged, got %d records", n)
	}
}

func TestIdentityFromGatewayHeaders_RealOrgPools(t *testing.T) {
	// A real tenant pools: every member spends the one org balance, so the billing
	// key (User) is the org slug and the full org/sub is Actor, for audit only.
	// Keying THIS caller per-user would gate an empty per-user ledger and deny a
	// funded org — the bug this guards, and the reason the rule is not "always the
	// person" either. Which of the two applies is not this middleware's opinion.
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(metering.HeaderOrgID, "zoo")
	r.Header.Set(metering.HeaderUserID, "bob")
	in := metering.IdentityFromGatewayHeaders(r)
	if in.User != "zoo" {
		t.Errorf("User (billing key) = %q, want zoo (a real org pools)", in.User)
	}
	if in.Actor != "zoo/bob" {
		t.Errorf("Actor (audit) = %q, want zoo/bob", in.Actor)
	}
	if in.Org != "zoo" {
		t.Errorf("Org = %q, want zoo", in.Org)
	}
}

// TestIdentityFromGatewayHeaders_AgreesWithTheGate is the anti-regression proof:
// whatever this middleware keys, the gate that authorizes the request and the
// ledger that records it must key the SAME account. They agree because there is
// one function, not because two copies were kept in step.
func TestIdentityFromGatewayHeaders_AgreesWithTheGate(t *testing.T) {
	for _, tc := range []struct{ org, sub, claim string }{
		{"zoo", "bob", ""},                      // a real org pools
		{"hanzo", "alice", ""},                  // a signup-org person holds their own
		{"acme", "bob", "person:acme/bob"},      // the claim names a person
		{"hanzo", "alice", "org:hanzo"},         // the claim names the pool
		{"acme", "bob", "project:acme/website"}, // the claim names a project
	} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set(metering.HeaderOrgID, tc.org)
		r.Header.Set(metering.HeaderUserID, tc.sub)
		if tc.claim != "" {
			r.Header.Set(metering.HeaderAccount, tc.claim)
		}
		got := metering.IdentityFromGatewayHeaders(r).User
		want := account.Payer(account.Credential{Owner: tc.org, Name: tc.sub, Account: tc.claim}).Subject()
		if got != want {
			t.Errorf("org=%q sub=%q claim=%q: metering keys %q, the gate keys %q — fund one, 402 off the other",
				tc.org, tc.sub, tc.claim, got, want)
		}
	}
}

func TestIdentityFromGatewayHeaders_OrgLessFallback(t *testing.T) {
	// No org header (org-less token): fall back to the bare sub so a per-user
	// balance can still gate.
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(metering.HeaderUserID, "solo")
	in := metering.IdentityFromGatewayHeaders(r)
	if in.User != "solo" {
		t.Errorf("org-less User = %q, want solo", in.User)
	}
	if in.Actor != "solo" {
		t.Errorf("org-less Actor = %q, want solo", in.Actor)
	}
}

// ---- tiny helpers (avoid pulling strconv/fmt into hot asserts) ----

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within deadline")
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func extractInt(body []byte, key string) int64 {
	s := string(body)
	idx := indexOf(s, key)
	if idx < 0 {
		return 0
	}
	idx += len(key)
	var n int64
	for idx < len(s) && (s[idx] == ' ') {
		idx++
	}
	neg := false
	if idx < len(s) && s[idx] == '-' {
		neg = true
		idx++
	}
	for idx < len(s) && s[idx] >= '0' && s[idx] <= '9' {
		n = n*10 + int64(s[idx]-'0')
		idx++
	}
	if neg {
		return -n
	}
	return n
}

func extractStr(body []byte, key string) string {
	s := string(body)
	idx := indexOf(s, key)
	if idx < 0 {
		return ""
	}
	idx += len(key)
	end := idx
	for end < len(s) && s[end] != '"' {
		end++
	}
	return s[idx:end]
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

var _ = context.Background
