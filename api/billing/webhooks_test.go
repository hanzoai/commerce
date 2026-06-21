package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/datastore/query"
	dbpkg "github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/models/billingevent"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/payment/processor"
	squareprovider "github.com/hanzoai/commerce/payment/providers/square"
)

// squareSigHeader is the exact header Square sends with each webhook
// delivery; lower-case on the wire, but http.Header canonicalises it.
const squareSigHeader = "x-square-hmacsha256-signature"

// testWebhookURL is the notification URL the tests pretend is configured in
// the Square dashboard. Square signs HMAC over this URL + the raw body, so
// every test signature must be computed over the same value.
const testWebhookURL = "https://api.test.example/v1/billing/webhooks/square"

// squareSign reproduces the REAL Square HMAC-SHA256(base64) signature:
// HMAC over notificationURL + rawBody (NOT body-only). This is the exact
// computation thirdparty/square/processor.go::ValidateWebhook verifies, so a
// matching signature must validate. squareSignBodyOnly reproduces the OLD
// (buggy) body-only scheme so a regression to it is caught by tests.
func squareSign(secret, url string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(url))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func squareSignBodyOnly(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// ─── test datastore ─────────────────────────────────────────────────────

// TestMain wires a real SQLite-backed datastore so the org-from-event
// resolution (GAP 1) can be exercised end-to-end: orgs are enumerated from
// the global namespace, subscriptions are looked up inside each org's
// namespace, and billing events are persisted. Without a default DB the
// handler's datastore calls would have nowhere to read/write.
func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp("", "billing-webhooks-test-*")
	if err != nil {
		panic(err)
	}

	cfg := dbpkg.DefaultConfig()
	cfg.DataDir = tempDir
	cfg.OrgDataDir = tempDir + "/orgs"
	cfg.UserDataDir = tempDir + "/users"
	cfg.EnableDatastore = false
	cfg.EnableVectorSearch = false

	mgr, err := dbpkg.NewManager(cfg)
	if err != nil {
		panic(err)
	}
	db, err := mgr.Org("test")
	if err != nil {
		panic(err)
	}
	datastore.SetDefaultDB(db)
	query.SetDefaultDB(db)

	code := m.Run()

	mgr.Close()
	os.RemoveAll(tempDir)
	os.Exit(code)
}

// seedOrgWithSubscription creates an organization (in the global namespace,
// where org records live) plus a subscription keyed by providerID inside that
// org's namespace — mirroring how production stores them. Returns the org.
func seedOrgWithSubscription(t *testing.T, name, providerID string, live bool) *organization.Organization {
	t.Helper()
	ctx := context.Background()

	root := datastore.New(ctx)
	org := organization.New(root)
	org.Name = name
	org.Live = live
	if err := org.Create(); err != nil { // plain Create: no JSON filter needed
		t.Fatalf("seed org %s: %v", name, err)
	}

	if providerID != "" {
		odb := datastore.New(org.Namespaced(ctx))
		sub := subscription.New(odb)
		sub.ProviderId = providerID
		sub.ProviderType = "square"
		sub.Status = subscription.Active
		if err := sub.Create(); err != nil {
			t.Fatalf("seed subscription %s in %s: %v", providerID, name, err)
		}
	}
	return org
}

// jsonExtractAvailable reports whether the underlying SQLite build provides the
// json_extract() function. The repo's pinned mattn/go-sqlite3 omits the JSON1
// extension, so field-filter queries (subscription lookup by ProviderId, the
// reconcile path) cannot run here — they DO run on the production data layer.
// Tests that exercise those paths skip when this returns false so they remain
// honest rather than asserting against a silently-empty result.
func jsonExtractAvailable() bool {
	_, err := subscription.Query(datastore.New(context.Background())).
		Filter("ProviderId=", "__probe__").Count()
	return err == nil
}

// withResolver swaps the package's org-resolution seam for the duration of a
// test so handler-branch logic (persist vs skip vs idempotent) is exercised
// deterministically, independent of the JSON-filter-dependent real resolver.
func withResolver(t *testing.T, fn func(*gin.Context, *processor.WebhookEvent) (*organization.Organization, bool)) {
	t.Helper()
	prev := orgForEvent
	orgForEvent = fn
	t.Cleanup(func() { orgForEvent = prev })
}

// ─── pickSignatureHeader ────────────────────────────────────────────────

func TestPickSignatureHeader(t *testing.T) {
	cases := []struct {
		name         string
		providerHint string
		headers      map[string]string
		want         string
	}{
		{
			name:         "square hint picks square header",
			providerHint: "square",
			headers:      map[string]string{squareSigHeader: "sq-sig-value"},
			want:         "sq-sig-value",
		},
		{
			name:         "square header found via candidate list without hint",
			providerHint: "",
			headers:      map[string]string{squareSigHeader: "sq-sig-value"},
			want:         "sq-sig-value",
		},
		{
			name:         "square header found even with mismatched hint",
			providerHint: "paypal",
			headers:      map[string]string{squareSigHeader: "sq-sig-value"},
			want:         "sq-sig-value",
		},
		{
			name:         "stripe hint still works (no regression)",
			providerHint: "stripe",
			headers:      map[string]string{"Stripe-Signature": "t=1,v1=abc"},
			want:         "t=1,v1=abc",
		},
		{
			name:         "no recognised header returns empty",
			providerHint: "square",
			headers:      map[string]string{"X-Unknown": "nope"},
			want:         "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			for k, v := range tc.headers {
				h.Set(k, v)
			}
			if got := pickSignatureHeader(h, tc.providerHint); got != tc.want {
				t.Errorf("pickSignatureHeader(%q) = %q, want %q", tc.providerHint, got, tc.want)
			}
		})
	}
}

// ─── registry population ────────────────────────────────────────────────

// TestGlobalRegistryPopulated proves the blank-import of the provider barrel
// in webhooks.go actually registers providers with processor.Global(). Before
// the fix the global registry was empty (no importer of the barrel), so this
// would fail with "processor square not registered".
func TestGlobalRegistryPopulated(t *testing.T) {
	reg := processor.Global()

	if len(reg.ListTypes()) == 0 {
		t.Fatal("global processor registry is empty; provider barrel not imported")
	}

	if _, err := reg.Get(processor.Square); err != nil {
		t.Fatalf("Square processor not in global registry: %v", err)
	}

	// Stripe is the default fiat processor and must also be present.
	if _, err := reg.Get(processor.Stripe); err != nil {
		t.Fatalf("Stripe processor not in global registry: %v", err)
	}
}

// ─── HandleProviderWebhook end-to-end ───────────────────────────────────

// newTestEngine builds a gin engine with the real webhook route mounted so
// tests exercise routing + handler exactly as production does.
func newTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/v1/billing/webhooks/:provider", HandleProviderWebhook)
	return r
}

// registerSquare installs a configured Square processor in the global registry
// for the duration of the test, restoring the previous one on cleanup.
func registerSquare(t *testing.T, secret string) {
	t.Helper()
	prev, _ := processor.Global().Get(processor.Square)
	t.Cleanup(func() {
		if prev != nil {
			processor.Global().Register(prev)
		}
	})
	processor.Global().Register(squareprovider.NewProvider(squareprovider.Config{
		AccessToken:         "test-token",
		LocationID:          "L1",
		WebhookSignatureKey: secret,
		WebhookURL:          testWebhookURL,
		Environment:         "sandbox",
	}))
}

// squareEvent builds a Square webhook body with a fresh created_at so it
// passes replay protection, carrying the given event id and object id.
func squareEvent(eventID, objectID string, created time.Time) []byte {
	return []byte(fmt.Sprintf(
		`{"merchant_id":"M1","type":"subscription.updated","event_id":%q,"created_at":%q,"data":{"type":"subscription","id":%q,"object":{"id":%q,"status":"active"}}}`,
		eventID, created.UTC().Format(time.RFC3339), objectID, objectID,
	))
}

func TestHandleProviderWebhook_MissingSignature(t *testing.T) {
	r := newTestEngine()

	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhooks/square", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for missing signature header", w.Code)
	}
}

// TestHandleProviderWebhook_BadSignatureReachesSquare proves the dispatcher
// reaches Square's ValidateWebhook: a recognised Square signature header is
// present and a Square processor is registered, so the request gets PAST
// header selection and processor lookup and is rejected by Square's HMAC
// comparison — a 401, not a 400-missing-header or a silent "no processor" miss.
func TestHandleProviderWebhook_BadSignatureReachesSquare(t *testing.T) {
	registerSquare(t, "whsec_test")

	r := newTestEngine()
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhooks/square", strings.NewReader(`{"type":"payment.created"}`))
	req.Header.Set(squareSigHeader, "dGhpcy1pcy1ub3QtdmFsaWQ=") // valid base64, wrong digest
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (signature rejected by Square)", w.Code)
	}

	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v (raw=%s)", err, w.Body.String())
	}
	if !strings.Contains(body.Error.Message, "invalid webhook signature") {
		t.Errorf("error message = %q, want it to mention invalid webhook signature", body.Error.Message)
	}
}

// ─── GAP 2: Square HMAC scheme (notificationURL + body) ──────────────────

// TestHandleProviderWebhook_URLPlusBodySignaturePasses proves the corrected
// scheme: a signature computed over webhookURL+body validates and the handler
// advances past validation. The event maps to no org here, so it stops at the
// safe 202 acknowledge-but-skip — which is ONLY reachable after a SUCCESSFUL
// signature check (a 401 would mean validation was still unreachable/wrong).
func TestHandleProviderWebhook_URLPlusBodySignaturePasses(t *testing.T) {
	const secret = "whsec_url_body"
	registerSquare(t, secret)

	body := squareEvent("evt_urlbody", "sub_unmapped_urlbody", time.Now())
	sig := squareSign(secret, testWebhookURL, body)

	r := newTestEngine()
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhooks/square", strings.NewReader(string(body)))
	req.Header.Set(squareSigHeader, sig)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusUnauthorized || w.Code == http.StatusBadRequest {
		t.Fatalf("status = %d: url+body signature should validate, but it did not", w.Code)
	}
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (validated, no matching org)", w.Code)
	}
}

// TestHandleProviderWebhook_BodyOnlySignatureFails proves a regression to the
// OLD body-only scheme would be caught: the same secret, signing the body
// alone, must now be REJECTED (the previous tests were self-consistent with
// the bug; this one is not).
func TestHandleProviderWebhook_BodyOnlySignatureFails(t *testing.T) {
	const secret = "whsec_bodyonly"
	registerSquare(t, secret)

	body := squareEvent("evt_bodyonly", "sub_bodyonly", time.Now())
	sig := squareSignBodyOnly(secret, body) // OLD buggy scheme

	r := newTestEngine()
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhooks/square", strings.NewReader(string(body)))
	req.Header.Set(squareSigHeader, sig)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 — body-only signature must be rejected by the url+body scheme", w.Code)
	}
}

// TestSquareValidateWebhook_Scheme is a direct, table-driven unit test of the
// processor's HMAC scheme so the URL+body contract is pinned independent of
// the HTTP handler.
func TestSquareValidateWebhook_Scheme(t *testing.T) {
	const secret = "whsec_scheme"
	body := squareEvent("evt_scheme", "sub_scheme", time.Now())

	cases := []struct {
		name string
		sig  string
		ok   bool
	}{
		{"url+body passes", squareSign(secret, testWebhookURL, body), true},
		{"body-only fails", squareSignBodyOnly(secret, body), false},
		{"wrong url fails", squareSign(secret, "https://evil.example/x", body), false},
		{"wrong key fails", squareSign("whsec_other", testWebhookURL, body), false},
		{"not base64 fails", "%%%not-base64%%%", false},
	}

	p := squareprovider.NewProvider(squareprovider.Config{
		AccessToken:         "test-token",
		LocationID:          "L1",
		WebhookSignatureKey: secret,
		WebhookURL:          testWebhookURL,
		Environment:         "sandbox",
	})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evt, err := p.ValidateWebhook(context.Background(), body, tc.sig)
			if tc.ok {
				if err != nil || evt == nil {
					t.Fatalf("expected pass, got err=%v evt=%v", err, evt)
				}
			} else if err == nil {
				t.Fatalf("expected rejection, got nil error (evt=%v)", evt)
			}
		})
	}
}

// TestSquareValidateWebhook_RequiresURL documents the safe behavior when no
// notification URL is configured: validation refuses rather than silently
// falling back to a body-only scheme live Square would never match.
func TestSquareValidateWebhook_RequiresURL(t *testing.T) {
	const secret = "whsec_nourl"
	body := squareEvent("evt_nourl", "sub_nourl", time.Now())

	p := squareprovider.NewProvider(squareprovider.Config{
		AccessToken:         "test-token",
		LocationID:          "L1",
		WebhookSignatureKey: secret,
		// WebhookURL intentionally empty.
		Environment: "sandbox",
	})

	// Even a "correct" body-only or url-less signature must be refused.
	sig := squareSignBodyOnly(secret, body)
	if _, err := p.ValidateWebhook(context.Background(), body, sig); err == nil {
		t.Fatal("expected refusal when webhook URL is unconfigured, got nil error")
	}
}

// ─── GAP 3: replay rejection + idempotency ───────────────────────────────

// TestSquareValidateWebhook_ReplayRejected proves the time-bounded replay
// guard: an event stamped older than the tolerance is rejected even with a
// valid signature; a fresh one passes.
func TestSquareValidateWebhook_ReplayRejected(t *testing.T) {
	const secret = "whsec_replay"

	p := squareprovider.NewProvider(squareprovider.Config{
		AccessToken:         "test-token",
		LocationID:          "L1",
		WebhookSignatureKey: secret,
		WebhookURL:          testWebhookURL,
		Environment:         "sandbox",
	})

	stale := squareEvent("evt_stale", "sub_replay", time.Now().Add(-10*time.Minute))
	staleSig := squareSign(secret, testWebhookURL, stale)
	if _, err := p.ValidateWebhook(context.Background(), stale, staleSig); err == nil {
		t.Fatal("expected stale event (10m old) to be rejected as replay, got nil error")
	}

	fresh := squareEvent("evt_fresh", "sub_replay", time.Now())
	freshSig := squareSign(secret, testWebhookURL, fresh)
	if _, err := p.ValidateWebhook(context.Background(), fresh, freshSig); err != nil {
		t.Fatalf("expected fresh event to pass, got %v", err)
	}
}

// billingEventExists reports whether a billing event with the given provider
// event id was persisted in the org's namespace, looked up by its deterministic
// key (id column — no json_extract dependency).
func billingEventExists(t *testing.T, org *organization.Organization, eventID string) bool {
	t.Helper()
	odb := datastore.New(org.Namespaced(context.Background()))
	e := billingevent.New(odb)
	return e.GetById(billingEventKey(eventID)) == nil
}

// globalBillingEventCount counts billing events in the blank/global namespace
// via an ancestor query (parent_id column — no json_extract dependency). Used
// to assert nothing ever leaks into the default namespace.
func globalBillingEventCount(t *testing.T, eventID string) int {
	t.Helper()
	gdb := datastore.New(context.Background())
	e := billingevent.New(gdb)
	if e.GetById(billingEventKey(eventID)) == nil {
		return 1
	}
	return 0
}

// ─── GAP 1: org-from-event resolution (handler branches) ─────────────────

// TestHandleProviderWebhook_PersistsForMappedOrg proves the previously-dead
// persist path is live: a valid-signature webhook whose event resolves to a
// known org is persisted into THAT org's namespace — a 200, not the old 503,
// and nothing in the blank namespace. The resolver seam makes the resolution
// deterministic; the persist + namespace scoping are the real handler code.
func TestHandleProviderWebhook_PersistsForMappedOrg(t *testing.T) {
	const secret = "whsec_mapped"
	registerSquare(t, secret)

	org := seedOrgWithSubscription(t, "mappedorg", "", true)
	withResolver(t, func(*gin.Context, *processor.WebhookEvent) (*organization.Organization, bool) {
		return org, true
	})

	body := squareEvent("evt_mapped", "sub_mapped", time.Now())
	sig := squareSign(secret, testWebhookURL, body)

	r := newTestEngine()
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhooks/square", strings.NewReader(string(body)))
	req.Header.Set(squareSigHeader, sig)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (validated + persisted); raw=%s", w.Code, w.Body.String())
	}

	// Persisted in the OWNING org's namespace...
	if !billingEventExists(t, org, "evt_mapped") {
		t.Fatalf("billing event evt_mapped not persisted in %s namespace", org.Name)
	}
	// ...and NOT in the global/blank namespace.
	if n := globalBillingEventCount(t, "evt_mapped"); n != 0 {
		t.Errorf("billing event leaked into global namespace: %d rows", n)
	}
}

// TestHandleProviderWebhook_NoOrgSafelySkipped proves a validated event that
// resolves to NO org is acknowledged with a safe 202 and NEVER written to a
// blank or default namespace.
func TestHandleProviderWebhook_NoOrgSafelySkipped(t *testing.T) {
	const secret = "whsec_noorg"
	registerSquare(t, secret)

	withResolver(t, func(*gin.Context, *processor.WebhookEvent) (*organization.Organization, bool) {
		return nil, false
	})

	body := squareEvent("evt_noorg", "sub_does_not_exist", time.Now())
	sig := squareSign(secret, testWebhookURL, body)

	r := newTestEngine()
	req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhooks/square", strings.NewReader(string(body)))
	req.Header.Set(squareSigHeader, sig)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (validated but unmapped); raw=%s", w.Code, w.Body.String())
	}

	// Nothing must be written to the global/blank namespace.
	if n := globalBillingEventCount(t, "evt_noorg"); n != 0 {
		t.Errorf("unmapped event wrote %d rows into the blank namespace, want 0", n)
	}
}

// TestHandleProviderWebhook_DuplicateEventIdempotent proves event_id
// idempotency: the billing event is keyed by the provider event id, so a
// redelivery (inside the replay window) is acknowledged as a duplicate and
// upserts the same single row rather than appending a second.
func TestHandleProviderWebhook_DuplicateEventIdempotent(t *testing.T) {
	const secret = "whsec_idem"
	registerSquare(t, secret)

	org := seedOrgWithSubscription(t, "idemorg", "", false)
	withResolver(t, func(*gin.Context, *processor.WebhookEvent) (*organization.Organization, bool) {
		return org, true
	})

	body := squareEvent("evt_dup", "sub_idem", time.Now())
	sig := squareSign(secret, testWebhookURL, body)

	deliver := func() *httptest.ResponseRecorder {
		r := newTestEngine()
		req := httptest.NewRequest(http.MethodPost, "/v1/billing/webhooks/square", strings.NewReader(string(body)))
		req.Header.Set(squareSigHeader, sig)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// First delivery → persisted (200, not duplicate).
	w := deliver()
	if w.Code != http.StatusOK {
		t.Fatalf("first delivery status = %d, want 200 (raw=%s)", w.Code, w.Body.String())
	}
	var resp1 struct {
		Duplicate bool `json:"duplicate"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp1)
	if resp1.Duplicate {
		t.Errorf("first delivery should NOT be a duplicate; body=%s", w.Body.String())
	}

	// Second (replayed) delivery → acknowledged as duplicate.
	w2 := deliver()
	if w2.Code != http.StatusOK {
		t.Fatalf("second delivery status = %d, want 200 (raw=%s)", w2.Code, w2.Body.String())
	}
	var resp2 struct {
		Duplicate bool `json:"duplicate"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	if !resp2.Duplicate {
		t.Errorf("second delivery should be flagged duplicate; body=%s", w2.Body.String())
	}

	// Exactly one row exists for this event id (idempotent upsert): the
	// deterministic key collapses redeliveries onto the same entity.
	odb := datastore.New(org.Namespaced(context.Background()))
	rootKey := odb.NewKey("synckey", "", 1, nil)
	events := make([]*billingevent.BillingEvent, 0)
	if _, err := billingevent.Query(odb).Ancestor(rootKey).GetAll(&events); err != nil {
		t.Fatalf("list billing events: %v", err)
	}
	n := 0
	for _, e := range events {
		if e.ObjectId == "evt_dup" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("billing events for evt_dup = %d, want exactly 1 (idempotent)", n)
	}
}

// ─── GAP 1: real org resolver (data-layer; JSON1-gated) ──────────────────

// TestResolveOrgForEvent_ByProviderId exercises the REAL resolution chain:
// enumerate orgs, look up the subscription by ProviderId in each org's
// namespace. This depends on json_extract (field-filter queries); it skips
// when the local SQLite build lacks JSON1 but runs on the production data
// layer and in CI.
func TestResolveOrgForEvent_ByProviderId(t *testing.T) {
	if !jsonExtractAvailable() {
		t.Skip("json_extract unavailable in this SQLite build; field-filter resolution exercised on the production data layer / CI")
	}
	seedOrgWithSubscription(t, "resolveorg", "sub_resolve", true)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	match := &processor.WebhookEvent{
		ID:   "evt_resolve",
		Type: "subscription.updated",
		Data: map[string]interface{}{"id": "sub_resolve", "status": "active"},
	}
	org, ok := resolveOrgForEvent(c, match)
	if !ok || org == nil {
		t.Fatalf("resolveOrgForEvent did not resolve a known subscription; ok=%v org=%v", ok, org)
	}
	if org.Name != "resolveorg" {
		t.Errorf("resolved org = %q, want resolveorg", org.Name)
	}

	miss := &processor.WebhookEvent{
		ID:   "evt_miss",
		Type: "subscription.updated",
		Data: map[string]interface{}{"id": "sub_unknown"},
	}
	if _, ok := resolveOrgForEvent(c, miss); ok {
		t.Error("resolveOrgForEvent resolved an unknown subscription, want ok=false")
	}
}

// TestResolveOrgForEvent_NoRefs needs no data layer: an event carrying no
// provider identifiers can never resolve, regardless of JSON1.
func TestResolveOrgForEvent_NoRefs(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	empty := &processor.WebhookEvent{ID: "evt_empty", Type: "ping", Data: nil}
	if _, ok := resolveOrgForEvent(c, empty); ok {
		t.Error("resolveOrgForEvent resolved an event with no provider refs, want ok=false")
	}
}

// TestProviderRefs verifies the order and filtering of provider identifiers
// extracted from an event's data object.
func TestProviderRefs(t *testing.T) {
	got := providerRefs(&processor.WebhookEvent{Data: map[string]interface{}{
		"id":           "sub_1",
		"subscription": "sub_2",
		"customer":     "cus_3",
		"ignored":      "x",
		"empty":        "",
	}})
	want := []string{"sub_1", "sub_2", "cus_3"}
	if len(got) != len(want) {
		t.Fatalf("providerRefs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("providerRefs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if refs := providerRefs(&processor.WebhookEvent{Data: nil}); len(refs) != 0 {
		t.Errorf("providerRefs(nil data) = %v, want empty", refs)
	}
}
