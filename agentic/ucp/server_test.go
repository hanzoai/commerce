// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

const platformURL = "https://platform.example/profile"

// peers is a Profiles that serves platform profiles from memory.
type peers struct {
	mu    sync.Mutex
	m     map[string]*Profile
	err   error
	calls int
}

func (p *peers) Platform(_ context.Context, u string, _ bool) (*Profile, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	if pr, ok := p.m[u]; ok {
		return pr, nil
	}
	return nil, fail(CodeProfileUnreachable, "no such profile")
}

func platform(version string, caps ...string) *Profile {
	p := &Profile{UCP: Meta{Version: version, Services: map[string][]Service{}, PaymentHandlers: map[string][]Handler{},
		Capabilities: map[string][]Capability{}}}
	for _, c := range caps {
		p.UCP.Capabilities[c] = []Capability{{Entity: Entity{Version: version, Spec: "https://ucp.dev/x", Schema: "https://ucp.dev/x"}}}
	}
	return p
}

// fakeBackend answers every operation with next, or err, and records the calls.
type fakeBackend struct {
	mu          sync.Mutex
	next        *Checkout
	err         error
	order       *Order
	calls       []string
	completeKey string
	completed   *CompleteRequest
	created     *CreateRequest
	who         Caller
}

func (b *fakeBackend) answer(op string) (*Checkout, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = append(b.calls, op)
	if b.err != nil {
		return nil, b.err
	}
	c := *b.next
	c.UCP = nil
	return &c, nil
}

func (b *fakeBackend) Create(_ context.Context, who Caller, in *CreateRequest) (*Checkout, error) {
	b.created, b.who = in, who
	return b.answer("create")
}
func (b *fakeBackend) Read(_ context.Context, _ Caller, _ string) (*Checkout, error) {
	return b.answer("read")
}
func (b *fakeBackend) Update(_ context.Context, _ Caller, _ string, _ *UpdateRequest) (*Checkout, error) {
	return b.answer("update")
}
func (b *fakeBackend) Complete(_ context.Context, _ Caller, _, key string, in *CompleteRequest) (*Checkout, error) {
	b.completeKey, b.completed = key, in
	return b.answer("complete")
}
func (b *fakeBackend) Cancel(_ context.Context, _ Caller, _ string) (*Checkout, error) {
	return b.answer("cancel")
}
func (b *fakeBackend) Order(_ context.Context, _ Caller, _ string) (*Order, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = append(b.calls, "order")
	if b.err != nil {
		return nil, b.err
	}
	o := *b.order
	return &o, nil
}

func (b *fakeBackend) count(op string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := 0
	for _, c := range b.calls {
		if c == op {
			n++
		}
	}
	return n
}

const goodKey = "550e8400-e29b-41d4-a716-446655440000"

type harness struct {
	t        *testing.T
	srv      *Server
	backend  *fakeBackend
	peers    *peers
	faults   []error
	identify func(*http.Request) (Caller, error)
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata/examples", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func fixtureCheckout(t *testing.T, name string) *Checkout {
	t.Helper()
	var c Checkout
	if err := json.Unmarshal(fixture(t, name), &c); err != nil {
		t.Fatal(err)
	}
	return &c
}

// newHarness builds a server whose profile advertises the given handlers.
func newHarness(t *testing.T, handlers map[string][]Handler, mutate ...func(*Config)) *harness {
	t.Helper()
	h := &harness{t: t, backend: &fakeBackend{}, peers: &peers{m: map[string]*Profile{platformURL: platform(Version, CapCheckout)}}}
	prof, err := NewProfile(ProfileConfig{Endpoint: "https://merchant.example/ucp/v1", Handlers: handlers, Order: true})
	if err != nil {
		t.Fatal(err)
	}
	h.identify = func(*http.Request) (Caller, error) { return Caller{Org: "acme", Subject: "buyer-1"}, nil }
	cfg := Config{
		Profile: prof, Backend: h.backend, Profiles: h.peers, Store: newMemStore(), MaxBody: 1 << 20,
		Identify:   func(r *http.Request) (Caller, error) { return h.identify(r) },
		Links:      []Link{{Type: "terms_of_service", URL: "https://business.example.com/terms"}},
		Storefront: "https://merchant.example/",
		Fault:      func(op string, err error) { h.faults = append(h.faults, err) },
	}
	for _, m := range mutate {
		m(&cfg)
	}
	h.srv, err = New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func shopPay() map[string][]Handler {
	return map[string][]Handler{"com.shopify.shop_pay": {{
		Entity:               Entity{ID: "shop_pay_1234", Version: Version, Config: json.RawMessage(`{"merchant_id":"shop_merchant_123"}`)},
		AvailableInstruments: []AvailableInstrument{{Type: "shop_pay"}},
	}}}
}

func gpay() map[string][]Handler {
	return map[string][]Handler{"com.google.pay": {{
		Entity: Entity{ID: "gpay_1234", Version: Version, Config: json.RawMessage(
			`{"allowed_payment_methods":[{"type":"CARD","parameters":{"allowed_card_networks":["VISA","MASTERCARD","AMEX"]}}]}`)},
	}}}
}

// memStore is an in-process Store.
type memStore struct {
	mu sync.Mutex
	m  map[string]*Seen
}

func newMemStore() *memStore { return &memStore{m: map[string]*Seen{}} }

func (s *memStore) Begin(_ context.Context, scope, key, digest string) (*Seen, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := scope + "\x00" + key
	if r, ok := s.m[id]; ok {
		seen := *r
		return &seen, nil
	}
	s.m[id] = &Seen{Digest: digest}
	return nil, nil
}

func (s *memStore) Finish(_ context.Context, scope, key, _ string, r Stored) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if seen, ok := s.m[scope+"\x00"+key]; ok {
		seen.Done, seen.Response = true, r
	}
	return nil
}

func (s *memStore) Release(_ context.Context, scope, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, scope+"\x00"+key)
	return nil
}

type call struct {
	method, path string
	body         []byte
	header       map[string]string
}

func (h *harness) do(c call) *httptest.ResponseRecorder {
	h.t.Helper()
	r := httptest.NewRequest(c.method, "https://merchant.example/ucp/v1"+c.path, bytes.NewReader(c.body))
	r.Header.Set("UCP-Agent", `profile="`+platformURL+`"`)
	if len(c.body) > 0 {
		r.Header.Set("Content-Type", "application/json")
	}
	if c.method != http.MethodGet {
		r.Header.Set("Idempotency-Key", goodKey)
	}
	for k, v := range c.header {
		if v == "" {
			r.Header.Del(k)
		} else {
			r.Header.Set(k, v)
		}
	}
	w := httptest.NewRecorder()
	h.srv.ServeHTTP(w, r)
	return w
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), v); err != nil {
		t.Fatalf("response is not JSON: %v\n%s", err, w.Body)
	}
}

func equalJSON(t *testing.T, got, want []byte, ignore ...string) {
	t.Helper()
	var g, w map[string]any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("got is not JSON: %v\n%s", err, got)
	}
	if err := json.Unmarshal(want, &w); err != nil {
		t.Fatal(err)
	}
	// Server-chosen values are named, never silently ignored.
	for _, k := range ignore {
		g[k], w[k] = "<server-chosen>", "<server-chosen>"
	}
	// ucp.status defaults to "success" in the schema: the examples state it in
	// some places and omit it in others, and the server always states it.
	for _, m := range []map[string]any{g, w} {
		if u, ok := m["ucp"].(map[string]any); ok && u["status"] == "success" {
			delete(u, "status")
		}
	}
	if !reflect.DeepEqual(g, w) {
		gb, _ := json.MarshalIndent(g, "", " ")
		wb, _ := json.MarshalIndent(w, "", " ")
		t.Fatalf("response differs from the specification example\n got %s\nwant %s", gb, wb)
	}
}

// The specification's REST examples drive the handlers: the example request is
// the HTTP request, and the response is compared with the example response.
// The checkout id is chosen by the business and the only member not compared.
func TestRESTExamplesAreAnsweredAsSpecified(t *testing.T) {
	cases := []struct {
		name     string
		handlers map[string][]Handler
		method   string
		path     string
		req      string // request fixture, "" for none
		res      string // response fixture
		status   int
	}{
		{"create", shopPay(), "POST", "/checkout-sessions", "shopping-checkout-rest-95.json", "shopping-checkout-rest-115.json", 201},
		{"update buyer", shopPay(), "PUT", "/checkout-sessions/chk_1234567890", "shopping-checkout-rest-249.json", "shopping-checkout-rest-275.json", 200},
		{"complete", gpay(), "POST", "/checkout-sessions/chk_123456789/complete", "shopping-checkout-rest-786.json", "shopping-checkout-rest-829.json", 200},
		{"get", shopPay(), "GET", "/checkout-sessions/chk_123456789", "", "shopping-checkout-rest-993.json", 200},
		{"cancel", gpay(), "POST", "/checkout-sessions/chk_123456789/cancel", "", "shopping-checkout-rest-1151.json", 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.handlers)
			want := fixture(t, tc.res)
			c := fixtureCheckout(t, tc.res)
			h.backend.next = c
			if tc.name == "complete" || tc.name == "cancel" {
				// the checkout the operation starts from
				pre := *c
				pre.Status, pre.Order = ReadyForComplete, nil
				h.backend.next = &pre
				h.backend.err = nil
			}
			var body []byte
			if tc.req != "" {
				body = fixture(t, tc.req)
			}
			if tc.name == "complete" || tc.name == "cancel" {
				// Read sees the ready checkout; the operation then answers the example.
				h.srv.cfg.Backend = &sequenceBackend{fakeBackend: h.backend, then: c}
			}
			w := h.do(call{method: tc.method, path: tc.path, body: body})
			if w.Code != tc.status {
				t.Fatalf("status = %d; want %d\n%s", w.Code, tc.status, w.Body)
			}
			equalJSON(t, w.Body.Bytes(), want, "id")
			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("Content-Type = %q", ct)
			}
		})
	}
}

// sequenceBackend answers reads from the starting checkout and the operation
// itself with the final one.
type sequenceBackend struct {
	*fakeBackend
	then *Checkout
}

func (s *sequenceBackend) Complete(_ context.Context, _ Caller, _, key string, in *CompleteRequest) (*Checkout, error) {
	s.completeKey, s.completed = key, in
	c := *s.then
	c.UCP = nil
	return &c, nil
}

func (s *sequenceBackend) Cancel(_ context.Context, _ Caller, _ string) (*Checkout, error) {
	c := *s.then
	c.UCP = nil
	return &c, nil
}

func TestCreateHandsTheBackendTheSpecifiedRequest(t *testing.T) {
	h := newHarness(t, shopPay())
	h.backend.next = fixtureCheckout(t, "shopping-checkout-rest-115.json")
	h.do(call{method: "POST", path: "/checkout-sessions", body: fixture(t, "shopping-checkout-rest-95.json")})
	if h.backend.created == nil || len(h.backend.created.LineItems) != 1 || h.backend.created.LineItems[0].Item.ID != "item_123" || h.backend.created.LineItems[0].Quantity != 2 {
		t.Fatalf("backend saw %+v", h.backend.created)
	}
}

func problemOf(t *testing.T, w *httptest.ResponseRecorder) Error {
	t.Helper()
	var e Error
	decodeBody(t, w, &e)
	e.Status = w.Code
	return e
}

func TestTransportErrorsFollowTheRegistry(t *testing.T) {
	create := fixture(t, "shopping-checkout-rest-95.json")
	post := call{method: "POST", path: "/checkout-sessions", body: create}
	with := func(c call, k, v string) call {
		h := map[string]string{}
		for a, b := range c.header {
			h[a] = b
		}
		h[k] = v
		c.header = h
		return c
	}
	for _, tc := range []struct {
		name   string
		setup  func(h *harness)
		call   call
		status int
		code   string
	}{
		{"no UCP-Agent", nil, with(post, "UCP-Agent", ""), 400, CodeInvalidProfileURL},
		{"UCP-Agent without profile", nil, with(post, "UCP-Agent", `other="x"`), 400, CodeInvalidProfileURL},
		{"UCP-Agent not HTTPS", nil, with(post, "UCP-Agent", `profile="http://platform.example/profile"`), 400, CodeInvalidProfileURL},
		{"UCP-Agent not a dictionary", nil, with(post, "UCP-Agent", `profile="a" b c`), 400, CodeInvalidProfileURL},
		{"profile unreachable", func(h *harness) { h.peers.err = fail(CodeProfileUnreachable, "connection timeout") }, post, 424, CodeProfileUnreachable},
		{"profile malformed", func(h *harness) { h.peers.err = fail(CodeProfileMalformed, "not JSON") }, post, 422, CodeProfileMalformed},
		{"version unsupported", func(h *harness) { h.peers.m[platformURL] = platform("2026-01-12", CapCheckout) }, post, 422, CodeVersionUnsupported},
		{"missing idempotency key", nil, with(post, "Idempotency-Key", ""), 400, "idempotency_key_missing"},
		{"short idempotency key", nil, with(post, "Idempotency-Key", "abc"), 400, "idempotency_key_invalid"},
		{"not JSON content type", nil, with(post, "Content-Type", "text/plain"), 400, "invalid_request"},
		{"unauthenticated", func(h *harness) {
			h.identify = func(*http.Request) (Caller, error) { return Caller{}, ErrUnauthorized }
		}, post, 401, "unauthorized"},
		{"forbidden", func(h *harness) { h.identify = func(*http.Request) (Caller, error) { return Caller{}, ErrForbidden } }, post, 403, "forbidden"},
		{"wrong method", nil, call{method: "DELETE", path: "/checkout-sessions/chk_1"}, 405, "method_not_allowed"},
		{"unknown path", nil, call{method: "GET", path: "/nothing"}, 404, "not_found"},
		{"empty create body", nil, call{method: "POST", path: "/checkout-sessions"}, 400, "invalid_request"},
		{"create without items", nil, call{method: "POST", path: "/checkout-sessions", body: []byte(`{"buyer":{"email":"a@b.c"}}`)}, 400, "invalid_request"},
		{"create with cart_id", nil, call{method: "POST", path: "/checkout-sessions", body: []byte(`{"cart_id":"c1"}`)}, 400, "invalid_request"},
		{"zero quantity", nil, call{method: "POST", path: "/checkout-sessions", body: []byte(`{"line_items":[{"item":{"id":"x"},"quantity":0}]}`)}, 400, "invalid_request"},
		{"fractional quantity", nil, call{method: "POST", path: "/checkout-sessions", body: []byte(`{"line_items":[{"item":{"id":"x"},"quantity":1.5}]}`)}, 400, "invalid_request"},
		{"quantity past 2^53", nil, call{method: "POST", path: "/checkout-sessions", body: []byte(`{"line_items":[{"item":{"id":"x"},"quantity":9007199254740992}]}`)}, 400, "invalid_request"},
		{"line without item id", nil, call{method: "POST", path: "/checkout-sessions", body: []byte(`{"line_items":[{"item":{},"quantity":1}]}`)}, 400, "invalid_request"},
		{"signal key not reverse domain", nil, call{method: "POST", path: "/checkout-sessions", body: []byte(`{"line_items":[{"item":{"id":"x"},"quantity":1}],"signals":{"buyer_ip":"1.2.3.4"}}`)}, 400, "invalid_request"},
		{"update without line_items", nil, call{method: "PUT", path: "/checkout-sessions/chk_1", body: []byte(`{"buyer":{}}`)}, 400, "invalid_request"},
		{"complete without payment", nil, call{method: "POST", path: "/checkout-sessions/chk_1/complete", body: []byte(`{}`)}, 400, "invalid_request"},
		{"body too large", func(h *harness) { h.srv.cfg.MaxBody = 8 }, post, 413, "request_too_large"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, shopPay())
			h.backend.next = fixtureCheckout(t, "shopping-checkout-rest-115.json")
			if tc.setup != nil {
				tc.setup(h)
			}
			w := h.do(tc.call)
			if w.Code != tc.status {
				t.Fatalf("status = %d; want %d\n%s", w.Code, tc.status, w.Body)
			}
			if e := problemOf(t, w); e.Code != tc.code || e.Content == "" {
				t.Fatalf("error = %+v; want code %s", e, tc.code)
			}
			if len(h.backend.calls) != 0 {
				t.Fatalf("the backend was called: %v", h.backend.calls)
			}
			if tc.status == 401 && w.Header().Get("WWW-Authenticate") != `Bearer realm="ucp"` {
				t.Fatalf("WWW-Authenticate = %q", w.Header().Get("WWW-Authenticate"))
			}
			if tc.code == CodeVersionUnsupported || tc.code == CodeProfileUnreachable {
				if e := problemOf(t, w); e.ContinueURL != "https://merchant.example/" {
					t.Fatalf("continue_url = %q", e.ContinueURL)
				}
			}
		})
	}
}

func TestNoCompatibleCapabilityIsABusinessOutcome(t *testing.T) {
	h := newHarness(t, shopPay())
	h.peers.m[platformURL] = platform(Version, "dev.ucp.shopping.cart")
	w := h.do(call{method: "POST", path: "/checkout-sessions", body: fixture(t, "shopping-checkout-rest-95.json")})
	if w.Code != 200 {
		t.Fatalf("status = %d; negotiation failure is a business outcome (200)", w.Code)
	}
	var r ErrorResponse
	decodeBody(t, w, &r)
	if r.UCP.Status != "error" || r.UCP.Version != Version || r.Messages[0].Code != CodeCapabilitiesIncompatible || r.Messages[0].Severity != Unrecoverable || r.ContinueURL == "" {
		t.Fatalf("response = %+v", r)
	}
	if len(h.backend.calls) != 0 {
		t.Fatal("the backend was called")
	}
}

func TestIdempotencyReplaysConflictsAndFailsClosed(t *testing.T) {
	h := newHarness(t, shopPay())
	h.backend.next = fixtureCheckout(t, "shopping-checkout-rest-115.json")
	create := call{method: "POST", path: "/checkout-sessions", body: fixture(t, "shopping-checkout-rest-95.json")}
	first := h.do(create)
	again := h.do(create)
	if first.Code != 201 || again.Code != 201 || !bytes.Equal(first.Body.Bytes(), again.Body.Bytes()) {
		t.Fatalf("a duplicate must return the original response: %d %d", first.Code, again.Code)
	}
	if n := h.backend.count("create"); n != 1 {
		t.Fatalf("backend created %d times; a duplicate runs nothing", n)
	}
	other := create
	other.body = []byte(`{"line_items":[{"item":{"id":"item_123"},"quantity":3}]}`)
	w := h.do(other)
	if w.Code != 409 || problemOf(t, w).Code != "idempotency_key_reused" {
		t.Fatalf("a reused key with another body: %d %s", w.Code, w.Body)
	}
	if n := h.backend.count("create"); n != 1 {
		t.Fatalf("a conflicting duplicate executed: %d", n)
	}
	// Another key is another operation.
	fresh := create
	fresh.header = map[string]string{"Idempotency-Key": "6f1c1d5e-0f0a-4c55-9b61-9a1f1f9c9d10"}
	if w := h.do(fresh); w.Code != 201 || h.backend.count("create") != 2 {
		t.Fatalf("a fresh key must run: %d", w.Code)
	}
	// The same key by another buyer is another operation too.
	h.identify = func(*http.Request) (Caller, error) { return Caller{Org: "acme", Subject: "buyer-2"}, nil }
	if w := h.do(create); w.Code != 201 || h.backend.count("create") != 3 {
		t.Fatalf("keys are scoped to the caller: %d", w.Code)
	}
}

type brokenStore struct{ Store }

func (brokenStore) Begin(context.Context, string, string, string) (*Seen, error) {
	return nil, errors.New("store down")
}

func TestStoreFailureFailsClosedWith503(t *testing.T) {
	h := newHarness(t, shopPay(), func(c *Config) { c.Store = brokenStore{} })
	h.backend.next = fixtureCheckout(t, "shopping-checkout-rest-115.json")
	w := h.do(call{method: "POST", path: "/checkout-sessions", body: fixture(t, "shopping-checkout-rest-95.json")})
	if w.Code != 503 {
		t.Fatalf("status %d", w.Code)
	}
	if len(h.backend.calls) != 0 {
		t.Fatal("a request that cannot be made repeatable must not run")
	}
}

func TestAServerFailureReleasesTheKeySoTheRequestCanBeRetried(t *testing.T) {
	h := newHarness(t, shopPay())
	h.backend.err = &Unavailable{RetryAfter: 7}
	create := call{method: "POST", path: "/checkout-sessions", body: fixture(t, "shopping-checkout-rest-95.json")}
	w := h.do(create)
	if w.Code != 503 || w.Header().Get("Retry-After") != "7" {
		t.Fatalf("status %d Retry-After %q", w.Code, w.Header().Get("Retry-After"))
	}
	h.backend.err = nil
	h.backend.next = fixtureCheckout(t, "shopping-checkout-rest-115.json")
	if w := h.do(create); w.Code != 201 {
		t.Fatalf("retry after a 503: %d %s", w.Code, w.Body)
	}
}

func TestBusinessOutcomesAreAnsweredWith200(t *testing.T) {
	h := newHarness(t, shopPay())
	// Nothing can be created: an error response, no resource.
	h.backend.err = &Refusal{Messages: []Message{Err("out_of_stock", Unrecoverable, "", "All requested items are currently out of stock")}, ContinueURL: "https://merchant.com/"}
	w := h.do(call{method: "POST", path: "/checkout-sessions", body: fixture(t, "shopping-checkout-rest-95.json")})
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var r ErrorResponse
	decodeBody(t, w, &r)
	if r.UCP.Status != "error" || r.Messages[0].Code != "out_of_stock" || r.ContinueURL != "https://merchant.com/" {
		t.Fatalf("response = %+v", r)
	}
	// An unknown checkout is the not_found business outcome.
	h.backend.err = ErrNotFound
	w = h.do(call{method: "GET", path: "/checkout-sessions/chk_nope"})
	decodeBody(t, w, &r)
	if w.Code != 200 || r.UCP.Status != "error" || r.Messages[0].Code != "not_found" {
		t.Fatalf("not found: %d %+v", w.Code, r)
	}
	// An internal failure is a 500 that says nothing of the cause.
	h.backend.err = errors.New("db: connection refused 10.0.0.9")
	w = h.do(call{method: "GET", path: "/checkout-sessions/chk_1"})
	if w.Code != 500 || strings.Contains(w.Body.String(), "10.0.0.9") || len(h.faults) != 1 {
		t.Fatalf("status %d body %s faults %v", w.Code, w.Body, h.faults)
	}
}

func lifecycle(t *testing.T, status string) *harness {
	h := newHarness(t, gpay())
	c := fixtureCheckout(t, "shopping-checkout-rest-829.json")
	c.Status = status
	if status != Completed {
		c.Order = nil
	}
	if status == RequiresEscalation {
		c.ContinueURL = "https://merchant.example/checkout/1"
		c.Messages = []Message{Err("schedule_delivery", RequiresBuyerInput, "", "Select delivery window")}
	}
	h.backend.next = c
	return h
}

func TestTheLifecycleGuardsUpdateCompleteAndCancel(t *testing.T) {
	update := []byte(`{"line_items":[{"id":"li_1","item":{"id":"item_123"},"quantity":2}]}`)
	complete := fixture(t, "shopping-checkout-rest-786.json")
	type want struct{ status, code, severity string }
	for _, tc := range []struct {
		name string
		from string
		call call
		want want
	}{
		{"update while completing", CompleteInProgress, call{method: "PUT", path: "/checkout-sessions/c", body: update}, want{CompleteInProgress, "in_progress", Recoverable}},
		{"update a completed checkout", Completed, call{method: "PUT", path: "/checkout-sessions/c", body: update}, want{Completed, "not_modifiable", Unrecoverable}},
		{"update a canceled checkout", Canceled, call{method: "PUT", path: "/checkout-sessions/c", body: update}, want{Canceled, "not_modifiable", Unrecoverable}},
		{"complete while completing", CompleteInProgress, call{method: "POST", path: "/checkout-sessions/c/complete", body: complete}, want{CompleteInProgress, "in_progress", Recoverable}},
		{"complete a completed checkout", Completed, call{method: "POST", path: "/checkout-sessions/c/complete", body: complete}, want{Completed, "not_modifiable", Unrecoverable}},
		{"complete a canceled checkout", Canceled, call{method: "POST", path: "/checkout-sessions/c/complete", body: complete}, want{Canceled, "not_modifiable", Unrecoverable}},
		{"complete an incomplete checkout", Incomplete, call{method: "POST", path: "/checkout-sessions/c/complete", body: complete}, want{Incomplete, "not_ready", Recoverable}},
		{"cancel a completed checkout", Completed, call{method: "POST", path: "/checkout-sessions/c/cancel"}, want{Completed, "not_cancelable", Unrecoverable}},
		{"cancel a canceled checkout", Canceled, call{method: "POST", path: "/checkout-sessions/c/cancel"}, want{Canceled, "not_cancelable", Unrecoverable}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := lifecycle(t, tc.from)
			w := h.do(tc.call)
			var c Checkout
			decodeBody(t, w, &c)
			if w.Code != 200 || c.Status != tc.want.status {
				t.Fatalf("status %d / %q; want 200 / %q\n%s", w.Code, c.Status, tc.want.status, w.Body)
			}
			last := c.Messages[len(c.Messages)-1]
			if last.Code != tc.want.code || last.Severity != tc.want.severity {
				t.Fatalf("message = %+v; want %s/%s", last, tc.want.code, tc.want.severity)
			}
			for _, op := range []string{"update", "complete", "cancel"} {
				if h.backend.count(op) != 0 {
					t.Fatalf("the backend was asked to %s", op)
				}
			}
		})
	}
	// Escalated checkouts hand off through continue_url without a new message.
	h := lifecycle(t, RequiresEscalation)
	w := h.do(call{method: "POST", path: "/checkout-sessions/c/complete", body: complete})
	var c Checkout
	decodeBody(t, w, &c)
	if c.Status != RequiresEscalation || c.ContinueURL == "" || h.backend.count("complete") != 0 {
		t.Fatalf("escalated checkout: %+v", c)
	}
}

func TestCompletePaymentIsCheckedAgainstTheAdvertisedHandlers(t *testing.T) {
	good := fixtureCheckoutRequest(t)
	for _, tc := range []struct {
		name   string
		mutate func(*CompleteRequest)
		path   string
	}{
		{"unknown handler", func(r *CompleteRequest) { r.Payment.Instruments[0].HandlerID = "other_handler" }, "$.payment.instruments[0].handler_id"},
		{"no instrument", func(r *CompleteRequest) { r.Payment.Instruments = nil }, "$.payment.instruments"},
		{"two instruments", func(r *CompleteRequest) {
			r.Payment.Instruments = append(r.Payment.Instruments, r.Payment.Instruments[0])
		}, "$.payment.instruments"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := lifecycle(t, ReadyForComplete)
			req := *good
			p := *good.Payment
			p.Instruments = append([]Instrument(nil), good.Payment.Instruments...)
			req.Payment = &p
			tc.mutate(&req)
			body, _ := json.Marshal(req)
			w := h.do(call{method: "POST", path: "/checkout-sessions/c/complete", body: body})
			var c Checkout
			decodeBody(t, w, &c)
			m := c.Messages[len(c.Messages)-1]
			if w.Code != 200 || c.Status != Incomplete || m.Code != "payment_failed" || m.Severity != Recoverable || m.Path != tc.path {
				t.Fatalf("status %d/%s message %+v\n%s", w.Code, c.Status, m, w.Body)
			}
			if h.backend.count("complete") != 0 {
				t.Fatal("the backend charged a payment that was refused")
			}
		})
	}
	// A valid completion reaches the backend with the key and the instrument.
	h := lifecycle(t, ReadyForComplete)
	body, _ := json.Marshal(good)
	w := h.do(call{method: "POST", path: "/checkout-sessions/c/complete", body: body})
	if w.Code != 200 || h.backend.completeKey != goodKey || h.backend.completed.Payment.Instruments[0].Credential.Token != "examplePaymentMethodToken" {
		t.Fatalf("complete: %d key %q", w.Code, h.backend.completeKey)
	}
}

func fixtureCheckoutRequest(t *testing.T) *CompleteRequest {
	t.Helper()
	var r CompleteRequest
	if err := json.Unmarshal(fixture(t, "shopping-checkout-rest-786.json"), &r); err != nil {
		t.Fatal(err)
	}
	return &r
}

// A backend that returns a checkout the schema forbids is a defect: nothing is sent.
func TestAnInvalidBackendCheckoutIsNeverSent(t *testing.T) {
	h := newHarness(t, shopPay())
	c := fixtureCheckout(t, "shopping-checkout-rest-115.json")
	c.Totals = c.Totals[:2] // no total entry
	h.backend.next = c
	w := h.do(call{method: "GET", path: "/checkout-sessions/chk_1"})
	if w.Code != 500 || len(h.faults) != 1 {
		t.Fatalf("status %d faults %v", w.Code, h.faults)
	}
}

func TestProfileIsServedAsTheSpecificationRequires(t *testing.T) {
	h := newHarness(t, shopPay())
	srv := httptest.NewServer(h.srv.Profile())
	defer srv.Close()
	resp, err := http.Get(srv.URL + WellKnown)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	cc := resp.Header.Get("Cache-Control")
	if resp.StatusCode != 200 || !strings.Contains(cc, "public") || !strings.Contains(cc, "max-age=60") || resp.Header.Get("ETag") == "" {
		t.Fatalf("status %d Cache-Control %q ETag %q", resp.StatusCode, cc, resp.Header.Get("ETag"))
	}
	for _, bad := range []string{"private", "no-store", "no-cache"} {
		if strings.Contains(cc, bad) {
			t.Fatalf("Cache-Control has %s", bad)
		}
	}
	var p Profile
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if err := p.Validate(Business); err != nil {
		t.Fatal(err)
	}
	if p.UCP.Version != Version || len(p.UCP.Capabilities) != 2 || p.UCP.Services[ServiceShopping][0].Endpoint != "https://merchant.example/ucp/v1" {
		t.Fatalf("profile = %+v", p.UCP)
	}
	// Revalidation answers 304, which is not a redirect.
	req, _ := http.NewRequest("GET", srv.URL+WellKnown, nil)
	req.Header.Set("If-None-Match", resp.Header.Get("ETag"))
	r2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r2.Body.Close()
	if r2.StatusCode != 304 {
		t.Fatalf("conditional GET = %d", r2.StatusCode)
	}
	// Only GET and HEAD.
	rp, _ := http.Post(srv.URL+WellKnown, "application/json", nil)
	rp.Body.Close()
	if rp.StatusCode != 405 {
		t.Fatalf("POST = %d", rp.StatusCode)
	}
}

func TestResponseCarriesOnlyTheRelevantNegotiatedCapabilities(t *testing.T) {
	h := newHarness(t, shopPay())
	h.peers.m[platformURL] = platform(Version, CapCheckout, CapOrder)
	h.backend.next = fixtureCheckout(t, "shopping-checkout-rest-115.json")
	w := h.do(call{method: "GET", path: "/checkout-sessions/chk_1"})
	var c Checkout
	decodeBody(t, w, &c)
	if _, ok := c.UCP.Capabilities[CapOrder]; ok || len(c.UCP.Capabilities[CapCheckout]) != 1 || c.UCP.Capabilities[CapCheckout][0].Version != Version {
		t.Fatalf("capabilities = %+v; a checkout response carries checkout and its extensions, not order", c.UCP.Capabilities)
	}
}

func TestRequestIDIsEchoed(t *testing.T) {
	h := newHarness(t, shopPay())
	h.backend.next = fixtureCheckout(t, "shopping-checkout-rest-115.json")
	w := h.do(call{method: "GET", path: "/checkout-sessions/chk_1", header: map[string]string{"Request-Id": "9a1f1f9c-0f0a-4c55-9b61-6f1c1d5e0f0a"}})
	if w.Header().Get("Request-Id") != "9a1f1f9c-0f0a-4c55-9b61-6f1c1d5e0f0a" {
		t.Fatalf("Request-Id = %q", w.Header().Get("Request-Id"))
	}
}

func TestNewRefusesAnIncompleteConfiguration(t *testing.T) {
	prof, _ := NewProfile(ProfileConfig{Endpoint: "https://merchant.example/ucp/v1"})
	ok := Config{Profile: prof, Backend: &fakeBackend{}, Profiles: &peers{}, Store: newMemStore(), MaxBody: 1, Identify: func(*http.Request) (Caller, error) { return Caller{}, nil }}
	if _, err := New(ok); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Config){
		"no backend":           func(c *Config) { c.Backend = nil },
		"no identity":          func(c *Config) { c.Identify = nil },
		"no profiles":          func(c *Config) { c.Profiles = nil },
		"require unsigned":     func(c *Config) { c.Require = true },
		"bad storefront":       func(c *Config) { c.Storefront = "http://x" },
		"no idempotency store": func(c *Config) { c.Store = nil },
		"no body limit":        func(c *Config) { c.MaxBody = 0 },
	} {
		c := ok
		mutate(&c)
		if _, err := New(c); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	withOrder, _ := NewProfile(ProfileConfig{Endpoint: "https://merchant.example/ucp/v1", Order: true})
	c := ok
	c.Profile = withOrder
	c.Backend = struct{ Backend }{&fakeBackend{}}
	if _, err := New(c); err == nil {
		t.Error("order advertised without a backend that serves orders")
	}
}

// With a Verifier the server authenticates the platform by its signature and,
// with a Signer, signs what it answers.
func TestServerEnforcesAndProducesSignatures(t *testing.T) {
	h := newHarness(t, shopPay())
	h.backend.next = fixtureCheckout(t, "shopping-checkout-rest-115.json")
	plat, platPub := newKeyPair(t, ES256)
	pp := platform(Version, CapCheckout)
	pp.Keys = []Key{platPub}
	h.peers.m[platformURL] = pp
	biz, bizPub := newKeyPair(t, ES384)
	biz.KeyID, bizPub.KID = "merchant-2026", "merchant-2026"
	h.srv.cfg.Verifier = &Verifier{Keys: AgentKeys{Profiles: h.peers}}
	h.srv.cfg.Require = true
	h.srv.cfg.Signer = biz

	payload := fixture(t, "shopping-checkout-rest-95.json")
	send := func(signer *Signer, mutate func(*http.Request), sent []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "https://merchant.example/ucp/v1/checkout-sessions", bytes.NewReader(payload))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("UCP-Agent", `profile="`+platformURL+`"`)
		r.Header.Set("Idempotency-Key", goodKey)
		if mutate != nil {
			mutate(r)
		}
		if signer != nil {
			if err := signer.SignRequest(r, payload, Options{}); err != nil {
				t.Fatal(err)
			}
		}
		if sent != nil {
			r.Body = nopCloser{bytes.NewReader(sent)}
		}
		w := httptest.NewRecorder()
		h.srv.ServeHTTP(w, r)
		return w
	}

	if w := send(nil, nil, nil); w.Code != 401 || problemOf(t, w).Code != CodeSignatureMissing {
		t.Fatalf("unsigned: %d %s", w.Code, w.Body)
	}
	w := send(plat, nil, nil)
	if w.Code != 201 {
		t.Fatalf("signed: %d %s", w.Code, w.Body)
	}
	// The response is signed with the business key and verifies as the spec describes.
	v := &Verifier{}
	if _, err := v.Response(w.Code, w.Header(), w.Body.Bytes(), []Key{bizPub}); err != nil {
		t.Fatalf("response signature: %v", err)
	}
	h.backend.calls = nil

	if w := send(plat, nil, []byte(`{"line_items":[{"item":{"id":"item_123"},"quantity":99}]}`)); w.Code != 400 || problemOf(t, w).Code != CodeDigestMismatch {
		t.Fatalf("tampered body: %d %s", w.Code, w.Body)
	}
	other, _ := newKeyPair(t, ES256)
	if w := send(other, nil, nil); w.Code != 401 || problemOf(t, w).Code != CodeSignatureInvalid {
		t.Fatalf("wrong key: %d %s", w.Code, w.Body)
	}
	stranger := &Signer{KeyID: "unknown", Key: plat.Key}
	if w := send(stranger, nil, nil); w.Code != 401 || problemOf(t, w).Code != CodeKeyNotFound {
		t.Fatalf("unknown key id: %d %s", w.Code, w.Body)
	}
	// A header the signature does not cover.
	r := httptest.NewRequest("POST", "https://merchant.example/ucp/v1/checkout-sessions", bytes.NewReader(payload))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("UCP-Agent", `profile="`+platformURL+`"`)
	if err := plat.SignRequest(r, payload, Options{}); err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Idempotency-Key", goodKey)
	rec := httptest.NewRecorder()
	h.srv.ServeHTTP(rec, r)
	if rec.Code != 401 || problemOf(t, rec).Code != CodeSignatureInvalid {
		t.Fatalf("uncovered Idempotency-Key: %d %s", rec.Code, rec.Body)
	}
	if h.backend.count("create") != 0 {
		t.Fatalf("a request that failed authentication reached the backend")
	}
	// Without Require, an unsigned request is the host's to authenticate; a bad signature is still refused.
	h.srv.cfg.Require = false
	if w := send(nil, nil, nil); w.Code != 201 {
		t.Fatalf("unsigned without Require: %d %s", w.Code, w.Body)
	}
	if w := send(other, nil, nil); w.Code != 401 {
		t.Fatalf("a signature that is present must verify: %d", w.Code)
	}
}

type nopCloser struct{ *bytes.Reader }

func (nopCloser) Close() error { return nil }

// Businesses may keep a registry of pre-approved platforms (overview,
// "Fetching"): a platform outside it is profile_not_trusted (403) and its
// profile is never fetched. The platform whose signature verified is named in
// the Caller the backend sees; an unsigned request names none.
func TestTrustedPlatformsAndTheVerifiedIdentity(t *testing.T) {
	h := newHarness(t, shopPay())
	h.backend.next = fixtureCheckout(t, "shopping-checkout-rest-115.json")
	create := call{method: "POST", path: "/checkout-sessions", body: fixture(t, "shopping-checkout-rest-95.json")}

	// Unsigned: the backend is told of no verified platform.
	if w := h.do(create); w.Code != 201 || h.backend.who.Platform != "" {
		t.Fatalf("unsigned: %d, platform %q", w.Code, h.backend.who.Platform)
	}

	plat, pub := newKeyPair(t, ES256)
	pp := platform(Version, CapCheckout)
	pp.Keys = []Key{pub}
	h.peers.m[platformURL] = pp
	h.srv.cfg.Verifier = &Verifier{Keys: AgentKeys{Profiles: h.peers}}
	h.srv.cfg.Trusted = func(p string) bool { return p == platformURL }
	signed := func(agent string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "https://merchant.example/ucp/v1/checkout-sessions", bytes.NewReader(create.body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("UCP-Agent", `profile="`+agent+`"`)
		r.Header.Set("Idempotency-Key", "6f1c1d5e-0f0a-4c55-9b61-9a1f1f9c9d10")
		if err := plat.SignRequest(r, create.body, Options{}); err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		h.srv.ServeHTTP(w, r)
		return w
	}
	if w := signed(platformURL); w.Code != 201 || h.backend.who.Platform != platformURL || h.backend.who.Subject != "buyer-1" {
		t.Fatalf("signed by a trusted platform: %d, caller %+v", w.Code, h.backend.who)
	}

	h.peers.calls, h.backend.calls = 0, nil
	w := signed("https://stranger.example/profile")
	if w.Code != 403 || problemOf(t, w).Code != CodeProfileNotTrusted {
		t.Fatalf("an unlisted platform: %d %s", w.Code, w.Body)
	}
	if h.peers.calls != 0 || len(h.backend.calls) != 0 {
		t.Fatalf("an untrusted platform's profile was fetched (%d) or the backend called (%v)", h.peers.calls, h.backend.calls)
	}
}
