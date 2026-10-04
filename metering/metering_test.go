package metering_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/hanzoai/commerce/metering"
)

// fakeCommerce records the last request and replies with a canned status+body.
type fakeCommerce struct {
	mu      sync.Mutex
	method  string
	path    string
	query   url.Values
	auth    string
	org     string
	testHdr string
	ctype   string
	body    []byte
	status  int
	reply   string
}

func (f *fakeCommerce) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		// Authorize also consults the per-scope spend-cap endpoint after funds pass
		// (issue #70). These tests pin the FUNDS contract (balance/usage), so
		// answer the scope call with the canned body but don't record it as the
		// asserted request.
		if strings.HasPrefix(r.URL.Path, "/v1/billing/alerts") {
			if f.status != 0 {
				w.WriteHeader(f.status)
			}
			_, _ = io.WriteString(w, f.reply)
			return
		}
		f.method = r.Method
		f.path = r.URL.Path
		f.query = r.URL.Query()
		f.auth = r.Header.Get("Authorization")
		f.org = r.Header.Get("X-Org-Id")
		f.testHdr = r.Header.Get("X-Hanzo-Test")
		f.ctype = r.Header.Get("Content-Type")
		f.body, _ = io.ReadAll(r.Body)
		if f.status == 0 {
			f.status = 200
		}
		w.WriteHeader(f.status)
		_, _ = io.WriteString(w, f.reply)
	}
}

func newClient(t *testing.T, srv *httptest.Server, cfg metering.Config) *metering.Client {
	t.Helper()
	cfg.BaseURL = srv.URL
	if cfg.Token == "" {
		cfg.Token = "svc-token"
	}
	if cfg.Org == "" {
		cfg.Org = "hanzo"
	}
	c, err := metering.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestAuthorize_Allows_WhenAvailablePositive(t *testing.T) {
	fc := &fakeCommerce{reply: `{"user":"hanzo/alice","currency":"usd","balance":5000,"holds":0,"available":5000}`}
	srv := httptest.NewServer(fc.handler())
	defer srv.Close()

	c := newClient(t, srv, metering.Config{})
	if err := c.Authorize(context.Background(), metering.AuthInput{User: "hanzo/alice"}); err != nil {
		t.Fatalf("Authorize allowed should be nil, got %v", err)
	}

	// Verify the exact commerce contract.
	if fc.method != http.MethodGet {
		t.Errorf("method = %s, want GET", fc.method)
	}
	if fc.path != "/v1/billing/balance" {
		t.Errorf("path = %s, want /v1/billing/balance", fc.path)
	}
	if got := fc.query.Get("user"); got != "hanzo/alice" {
		t.Errorf("user query = %q, want hanzo/alice", got)
	}
	if got := fc.query.Get("currency"); got != "usd" {
		t.Errorf("currency query = %q, want usd", got)
	}
	if fc.auth != "Bearer svc-token" {
		t.Errorf("auth = %q, want Bearer svc-token", fc.auth)
	}
	if fc.org != "hanzo" {
		t.Errorf("X-Org-Id = %q, want hanzo", fc.org)
	}
}

func TestAuthorize_Denies_WhenAvailableZero(t *testing.T) {
	fc := &fakeCommerce{reply: `{"available":0}`}
	srv := httptest.NewServer(fc.handler())
	defer srv.Close()

	c := newClient(t, srv, metering.Config{})
	err := c.Authorize(context.Background(), metering.AuthInput{User: "hanzo/alice"})
	if err != metering.ErrInsufficientBalance {
		t.Fatalf("want ErrInsufficientBalance, got %v", err)
	}
}

func TestAuthorize_FailClosed_OnCommerceError(t *testing.T) {
	fc := &fakeCommerce{status: 500, reply: `boom`}
	srv := httptest.NewServer(fc.handler())
	defer srv.Close()

	c := newClient(t, srv, metering.Config{})
	err := c.Authorize(context.Background(), metering.AuthInput{User: "hanzo/alice"})
	if err == nil {
		t.Fatal("fail-closed: commerce 500 must deny, got nil")
	}
	if err == metering.ErrInsufficientBalance {
		t.Fatal("a 500 is 'unknown', not 'insufficient' — must be a connectivity error")
	}
}

// There is no client without a commerce to ask, and a nil one refuses every
// call: nothing is served or dropped unchecked.
func TestNoCommerceRefuses(t *testing.T) {
	if _, err := metering.New(metering.Config{}); err != metering.ErrNotConfigured {
		t.Fatalf("New with no BaseURL = %v, want ErrNotConfigured", err)
	}
	var c *metering.Client
	if err := c.Authorize(context.Background(), metering.AuthInput{User: "hanzo/alice"}); err != metering.ErrNotConfigured {
		t.Fatalf("nil client Authorize = %v, want ErrNotConfigured", err)
	}
	if _, err := c.Record(context.Background(), metering.Usage{User: "hanzo/alice", AmountCents: 100}); err != metering.ErrNotConfigured {
		t.Fatalf("nil client Record = %v, want ErrNotConfigured", err)
	}
	if _, err := c.ScopeRules(context.Background(), "hanzo"); err != metering.ErrNotConfigured {
		t.Fatalf("nil client ScopeRules = %v, want ErrNotConfigured", err)
	}
}

// No call ever routes to commerce's sandbox ledger: the mode is the org's, not
// the meter's.
func TestNoTestHeaderIsEverSent(t *testing.T) {
	fc := &fakeCommerce{reply: `{"available":1}`}
	srv := httptest.NewServer(fc.handler())
	defer srv.Close()

	c := newClient(t, srv, metering.Config{})
	if err := c.Authorize(context.Background(), metering.AuthInput{User: "hanzo/alice"}); err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if fc.testHdr != "" {
		t.Errorf("X-Hanzo-Test = %q; the meter must never pick the sandbox ledger", fc.testHdr)
	}
}

func TestAuthorize_PerCallOrgOverride(t *testing.T) {
	fc := &fakeCommerce{reply: `{"available":1}`}
	srv := httptest.NewServer(fc.handler())
	defer srv.Close()

	c := newClient(t, srv, metering.Config{}) // default org hanzo
	_ = c.Authorize(context.Background(), metering.AuthInput{User: "zoo/bob", Org: "zoo"})
	if fc.org != "zoo" {
		t.Errorf("per-call org override: X-Org-Id = %q, want zoo", fc.org)
	}
}

func TestRecord_PostsCanonicalPayload(t *testing.T) {
	fc := &fakeCommerce{status: 201, reply: `{"transactionId":"tx_123","user":"hanzo/alice","amount":250,"currency":"usd","type":"withdraw"}`}
	srv := httptest.NewServer(fc.handler())
	defer srv.Close()

	c := newClient(t, srv, metering.Config{})
	res, err := c.Record(context.Background(), metering.Usage{
		User:        "hanzo/alice",
		AmountCents: 250,
		Provider:    "search",
		RequestID:   "req-9",
		Status:      "success",
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if res == nil || res.TransactionID != "tx_123" || res.Amount != 250 {
		t.Fatalf("unexpected RecordResult: %+v", res)
	}

	if fc.method != http.MethodPost {
		t.Errorf("method = %s, want POST", fc.method)
	}
	if fc.path != "/v1/billing/usage" {
		t.Errorf("path = %s, want /v1/billing/usage", fc.path)
	}
	if fc.ctype != "application/json" {
		t.Errorf("content-type = %q", fc.ctype)
	}
	if fc.auth != "Bearer svc-token" {
		t.Errorf("auth = %q", fc.auth)
	}
	if fc.org != "hanzo" {
		t.Errorf("X-Org-Id = %q, want hanzo", fc.org)
	}

	// Verify the JSON body matches commerce's usageRequest field names.
	var body map[string]any
	if err := json.Unmarshal(fc.body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["user"] != "hanzo/alice" {
		t.Errorf("body.user = %v", body["user"])
	}
	if body["amount"].(float64) != 250 {
		t.Errorf("body.amount = %v, want 250", body["amount"])
	}
	if body["currency"] != "usd" {
		t.Errorf("body.currency = %v, want usd (defaulted)", body["currency"])
	}
	if body["provider"] != "search" {
		t.Errorf("body.provider = %v, want search", body["provider"])
	}
	if body["requestId"] != "req-9" {
		t.Errorf("body.requestId = %v", body["requestId"])
	}
	// Org must NOT be in the body (it travels via the header).
	if _, ok := body["Org"]; ok {
		t.Error("Org leaked into the JSON body; it must be a header only")
	}
}

// A $0 call is recorded like any other: commerce writes its row.
func TestRecord_ZeroAmountIsRecorded(t *testing.T) {
	fc := &fakeCommerce{status: 201, reply: `{"transactionId":"tx_0","amount":0,"type":"withdraw"}`}
	srv := httptest.NewServer(fc.handler())
	defer srv.Close()

	c := newClient(t, srv, metering.Config{})
	res, err := c.Record(context.Background(), metering.Usage{User: "hanzo/alice", CostMicros: 400, PaidBy: "plan"})
	if err != nil || res == nil || res.TransactionID != "tx_0" {
		t.Fatalf("zero-amount Record = (%+v, %v), want the written row", res, err)
	}
	var body map[string]any
	if err := json.Unmarshal(fc.body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if fc.path != "/v1/billing/usage" || body["amount"].(float64) != 0 || body["costMicros"].(float64) != 400 || body["paidBy"] != "plan" {
		t.Fatalf("POST %s body %v; want amount 0, costMicros 400, paidBy plan", fc.path, body)
	}
}

func TestRecord_RefusesANegativeCharge(t *testing.T) {
	fc := &fakeCommerce{}
	srv := httptest.NewServer(fc.handler())
	defer srv.Close()
	c := newClient(t, srv, metering.Config{})
	if _, err := c.Record(context.Background(), metering.Usage{User: "hanzo/alice", AmountCents: -1}); err == nil {
		t.Fatal("a negative charge was sent")
	}
}

func TestNew_RejectsBadURL(t *testing.T) {
	if _, err := metering.New(metering.Config{BaseURL: "://bad"}); err == nil {
		t.Fatal("expected error for unparseable BaseURL")
	}
}

func TestConfigFromEnv_Defaults(t *testing.T) {
	t.Setenv(metering.EnvBaseURL, "")
	t.Setenv(metering.EnvOrg, "")
	cfg := metering.ConfigFromEnv()
	if cfg.BaseURL != metering.DefaultBaseURL {
		t.Errorf("default BaseURL = %q, want %q", cfg.BaseURL, metering.DefaultBaseURL)
	}
	if cfg.Org != "hanzo" {
		t.Errorf("default Org = %q, want hanzo", cfg.Org)
	}
}

// The retired switches are inert: setting them changes nothing about the client.
func TestConfigFromEnv_NoSwitchTurnsEnforcementOff(t *testing.T) {
	t.Setenv(metering.EnvBaseURL, "http://commerce:8001")
	want := metering.ConfigFromEnv()
	for _, k := range []string{"METERING_DISABLED", "METERING_FAIL_OPEN", "METERING_TEST", "METERING_TIER_AWARE"} {
		t.Setenv(k, "true")
	}
	if got := metering.ConfigFromEnv(); got != want {
		t.Fatalf("with the retired switches set the config became %+v, want %+v", got, want)
	}
}

func TestConfigFromEnv_ReadsToken(t *testing.T) {
	t.Setenv(metering.EnvToken, "kms-sourced-token")
	cfg := metering.ConfigFromEnv()
	if cfg.Token != "kms-sourced-token" {
		t.Errorf("token = %q", cfg.Token)
	}
}

// TestContractMatchesGateway pins the wire format the gateway already uses, so
// this client and the gateway gate on the identical balance source.
func TestContractMatchesGateway(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture ONLY the balance request. Authorize also consults the per-scope
		// spend-cap endpoint after funds pass (issue #70); this test pins the
		// balance contract, so ignore the follow-on /alerts/authorize call.
		if strings.HasPrefix(r.URL.Path, "/v1/billing/balance") {
			gotURL = r.URL.String()
		}
		_, _ = io.WriteString(w, `{"available":1}`)
	}))
	defer srv.Close()

	c := newClient(t, srv, metering.Config{})
	_ = c.Authorize(context.Background(), metering.AuthInput{User: "hanzo/alice"})

	// Gateway: GET {base}/v1/billing/balance?user=hanzo%2Falice&currency=usd
	if !strings.HasPrefix(gotURL, "/v1/billing/balance?") {
		t.Fatalf("URL %q must start with /v1/billing/balance?", gotURL)
	}
	if !strings.Contains(gotURL, "user=hanzo%2Falice") {
		t.Errorf("URL %q must url-encode the user as hanzo%%2Falice", gotURL)
	}
	if !strings.Contains(gotURL, "currency=usd") {
		t.Errorf("URL %q must carry currency=usd", gotURL)
	}
}
