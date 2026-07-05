package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// capCtx builds a gin context scoped to org `org` (name == namespace) — the exact
// plumbing the gateway/middleware injects, so the handlers resolve the ae SQLite
// datastore in that org's namespace.
func capCtx(w http.ResponseWriter, org *organization.Organization) *gin.Context {
	c, _ := gin.CreateTestContext(w)
	c.Set("organization", org)
	c.Set("context", nscontext.WithNamespace(context.Background(), org.Name))
	return c
}

// createCap drives CreateSpendAlert with a JSON body and returns the status.
func createCap(t *testing.T, org *organization.Organization, body string) int {
	t.Helper()
	w := httptest.NewRecorder()
	c := capCtx(w, org)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/billing/spend-alerts", bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	CreateSpendAlert(c)
	if w.Code != 201 {
		t.Fatalf("CreateSpendAlert status = %d, body=%s", w.Code, w.Body.String())
	}
	return w.Code
}

// recordUsage drives RecordUsage with a JSON body and returns the status.
func recordUsage(t *testing.T, org *organization.Organization, body string) int {
	t.Helper()
	w := httptest.NewRecorder()
	c := capCtx(w, org)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/billing/usage", bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	RecordUsage(c)
	return w.Code
}

// authorize drives AuthorizeSpendCap and returns the parsed verdict.
func authorize(t *testing.T, org *organization.Organization, query string) authorizeResult {
	t.Helper()
	w := httptest.NewRecorder()
	c := capCtx(w, org)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/billing/spend-alerts/authorize?"+query, nil)
	AuthorizeSpendCap(c)
	if w.Code != 200 {
		t.Fatalf("AuthorizeSpendCap status = %d, body=%s", w.Code, w.Body.String())
	}
	var res authorizeResult
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode authorize: %v (body=%s)", err, w.Body.String())
	}
	return res
}

// (i) HARD spend cap: a $1 enforced cap allows spend up to the cap, then DENIES
// the next request with reason spend_cap once period spend reaches the ceiling.
func TestSpendCap_HardEnforce_DeniesAtCap(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	gin.SetMode(gin.TestMode)

	org := &organization.Organization{}
	org.Name = "cap-hard"

	// $1.00 hard cap on the org-wide default scope.
	createCap(t, org, `{"title":"org cap","threshold":100,"enforce":true}`)

	// Before any spend, a $0.50 request is authorized.
	if v := authorize(t, org, "user=cap-hard&amount=50"); !v.Allow {
		t.Fatalf("pre-spend authorize denied: %+v", v)
	}

	// Spend the full $1.00.
	if code := recordUsage(t, org, `{"user":"cap-hard","amount":100,"requestId":"r1"}`); code != 201 {
		t.Fatalf("RecordUsage status = %d", code)
	}

	// The next request (any positive amount) must be DENIED — funded or not, the
	// tenant's own cap is reached.
	v := authorize(t, org, "user=cap-hard&amount=1")
	if v.Allow {
		t.Fatalf("post-cap authorize ALLOWED, want deny: %+v", v)
	}
	if v.Reason != "spend_cap" {
		t.Fatalf("reason = %q, want spend_cap", v.Reason)
	}
	if v.CapCents != 100 || v.SpentCents != 100 {
		t.Fatalf("cap/spent = %d/%d, want 100/100", v.CapCents, v.SpentCents)
	}
}

// (ii) SOFT warn: a request under the cap but at/over the soft threshold reports a
// warnPct (drives the X-Spend-Warn header at the edge) without denying.
func TestSpendCap_SoftWarn_ReportsWarnPct(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	gin.SetMode(gin.TestMode)

	org := &organization.Organization{}
	org.Name = "cap-warn"

	// $1.00 cap, soft threshold 50%. Enforce=true so it also warns before the cap.
	createCap(t, org, `{"title":"warn cap","threshold":100,"enforce":true,"softPct":50}`)

	// Spend $0.60 — over the 50% soft threshold, under the cap.
	if code := recordUsage(t, org, `{"user":"cap-warn","amount":60,"requestId":"w1"}`); code != 201 {
		t.Fatalf("RecordUsage status = %d", code)
	}

	v := authorize(t, org, "user=cap-warn&amount=1")
	if !v.Allow {
		t.Fatalf("authorize denied under cap: %+v", v)
	}
	if v.WarnPct < 60 {
		t.Fatalf("warnPct = %d, want >= 60 (0.60/1.00 utilization over the 50%% soft threshold)", v.WarnPct)
	}
}

// (iv) Tenant isolation: a cap on org A / project P gates ONLY A/P. A different
// project Q in the same org, and a different org B, are NOT gated by it.
func TestSpendCap_TenantIsolation(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	gin.SetMode(gin.TestMode)

	orgA := &organization.Organization{}
	orgA.Name = "iso-a"
	orgB := &organization.Organization{}
	orgB.Name = "iso-b"

	// A has a $1 hard cap on project "P"; A/P has already spent $1.
	createCap(t, orgA, `{"title":"A/P cap","threshold":100,"enforce":true,"project":"P"}`)
	if code := recordUsage(t, orgA, `{"user":"iso-a","amount":100,"requestId":"a1","project":"P"}`); code != 201 {
		t.Fatalf("RecordUsage A/P status = %d", code)
	}

	// A/P is over cap → denied.
	if v := authorize(t, orgA, "user=iso-a&project=P&amount=1"); v.Allow {
		t.Fatalf("A/P authorize ALLOWED, want deny: %+v", v)
	}
	// A/Q (different project, no cap covering it) → allowed.
	if v := authorize(t, orgA, "user=iso-a&project=Q&amount=1"); !v.Allow {
		t.Fatalf("A/Q authorize DENIED, want allow (project P's cap must not gate Q): %+v", v)
	}
	// B/P (different org, no rows at all) → allowed. A's cap must never gate B.
	if v := authorize(t, orgB, "user=iso-b&project=P&amount=1"); !v.Allow {
		t.Fatalf("B/P authorize DENIED, want allow (org A's cap must not gate org B): %+v", v)
	}
}

// createCapID drives CreateSpendAlert and returns the new row id.
func createCapID(t *testing.T, org *organization.Organization, body string) string {
	t.Helper()
	w := httptest.NewRecorder()
	c := capCtx(w, org)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/billing/spend-alerts", bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	CreateSpendAlert(c)
	if w.Code != 201 {
		t.Fatalf("CreateSpendAlert status = %d, body=%s", w.Code, w.Body.String())
	}
	var r struct {
		Id string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil || r.Id == "" {
		t.Fatalf("create id decode: %v (body=%s)", err, w.Body.String())
	}
	return r.Id
}

func patchCap(t *testing.T, org *organization.Organization, id, user, body string) int {
	t.Helper()
	w := httptest.NewRecorder()
	c := capCtx(w, org)
	c.Params = gin.Params{{Key: "id", Value: id}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/v1/billing/spend-alerts/"+id+"?user="+user, bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	UpdateSpendAlert(c)
	return w.Code
}

func deleteCap(t *testing.T, org *organization.Organization, id, user string) int {
	t.Helper()
	w := httptest.NewRecorder()
	c := capCtx(w, org)
	c.Params = gin.Params{{Key: "id", Value: id}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/v1/billing/spend-alerts/"+id+"?user="+user, nil)
	DeleteSpendAlert(c)
	return w.Code
}

// Per-row ownership (IDOR): a caller may PATCH/DELETE only a row they OWN. Bob
// cannot mutate Alice's budget by guessing its id (404, no existence leak); Alice
// can.
func TestSpendCap_MutateOwnership_IDOR(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	gin.SetMode(gin.TestMode)

	org := &organization.Organization{}
	org.Name = "idor-org"

	id := createCapID(t, org, `{"userId":"alice","title":"alice budget","threshold":100,"enforce":true}`)

	// Bob (not the owner) is refused on both mutating verbs — 404, no leak.
	if code := patchCap(t, org, id, "bob", `{"threshold":1}`); code != 404 {
		t.Fatalf("bob PATCH alice's row = %d, want 404 (IDOR must be refused)", code)
	}
	if code := deleteCap(t, org, id, "bob"); code != 404 {
		t.Fatalf("bob DELETE alice's row = %d, want 404 (IDOR must be refused)", code)
	}

	// Alice (the owner) succeeds.
	if code := patchCap(t, org, id, "alice", `{"threshold":200}`); code != 200 {
		t.Fatalf("alice PATCH own row = %d, want 200", code)
	}
	if code := deleteCap(t, org, id, "alice"); code != 204 {
		t.Fatalf("alice DELETE own row = %d, want 204", code)
	}
}

// (v) Idempotency: recording the SAME requestId twice creates AT MOST ONE debit,
// so period spend (and thus the cap verdict) counts it once — no double count.
func TestSpendCap_Idempotent_NoDoubleCount(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	gin.SetMode(gin.TestMode)

	org := &organization.Organization{}
	org.Name = "cap-idem"

	body := `{"user":"cap-idem","amount":100,"requestId":"dup-1"}`
	if code := recordUsage(t, org, body); code != 201 {
		t.Fatalf("RecordUsage #1 status = %d", code)
	}
	// A retry of the same requestId must replay, not double-debit.
	if code := recordUsage(t, org, body); code != 200 && code != 201 {
		t.Fatalf("RecordUsage #2 (replay) status = %d", code)
	}

	db := datastore.New(nscontext.WithNamespace(context.Background(), org.Name))
	spent, err := scopeSpentCents(db, org.TestMode(), "", "")
	if err != nil {
		t.Fatalf("scopeSpentCents: %v", err)
	}
	if spent != 100 {
		t.Fatalf("period spent = %d, want 100 (a duplicate requestId must not double-count)", spent)
	}
}
