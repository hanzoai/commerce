package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/auth"
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

// userCtx is capCtx as a VALIDATED IAM user with the given subject — the identity
// ownership derives from (never ?user=). subject "" means an unauthenticated
// caller (no validated claim).
func userCtx(w http.ResponseWriter, org *organization.Organization, subject string) *gin.Context {
	c := capCtx(w, org)
	claims := &auth.IAMClaims{}
	claims.Subject = subject // promoted from the embedded StandardClaims.
	c.Set("iam_claims", claims)
	return c
}

// createCap drives CreateSpendAlert as IAM user "owner" (so the row has a
// deterministic owner) and returns the status.
func createCap(t *testing.T, org *organization.Organization, body string) int {
	t.Helper()
	w := httptest.NewRecorder()
	c := userCtx(w, org, "owner")
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

	// pv=1 so the project cap hard-enforces (isolation under HARD enforcement).
	// A/P is over cap → denied.
	if v := authorize(t, orgA, "user=iso-a&project=P&amount=1&pv=1"); v.Allow {
		t.Fatalf("A/P authorize ALLOWED, want deny: %+v", v)
	}
	// A/Q (different project, no cap covering it) → allowed.
	if v := authorize(t, orgA, "user=iso-a&project=Q&amount=1&pv=1"); !v.Allow {
		t.Fatalf("A/Q authorize DENIED, want allow (project P's cap must not gate Q): %+v", v)
	}
	// B/P (different org, no rows at all) → allowed. A's cap must never gate B.
	if v := authorize(t, orgB, "user=iso-b&project=P&amount=1&pv=1"); !v.Allow {
		t.Fatalf("B/P authorize DENIED, want allow (org A's cap must not gate org B): %+v", v)
	}
}

// createCapAs drives CreateSpendAlert as IAM user `subject` and returns the row id.
func createCapAs(t *testing.T, org *organization.Organization, subject, body string) string {
	t.Helper()
	w := httptest.NewRecorder()
	c := userCtx(w, org, subject)
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

// patchCapAs / deleteCapAs drive the mutating verbs as IAM user `caller`, with an
// attacker-chosen ?user= query that MUST be ignored for ownership.
func patchCapAs(t *testing.T, org *organization.Organization, id, caller, userQuery, body string) int {
	t.Helper()
	w := httptest.NewRecorder()
	c := userCtx(w, org, caller)
	c.Params = gin.Params{{Key: "id", Value: id}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/v1/billing/spend-alerts/"+id+"?user="+userQuery, bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	UpdateSpendAlert(c)
	return w.Code
}

func deleteCapAs(t *testing.T, org *organization.Organization, id, caller, userQuery string) int {
	t.Helper()
	w := httptest.NewRecorder()
	c := userCtx(w, org, caller)
	c.Params = gin.Params{{Key: "id", Value: id}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/v1/billing/spend-alerts/"+id+"?user="+userQuery, nil)
	DeleteSpendAlert(c)
	return w.Code
}

// Per-row ownership (IDOR, HIGH-1): ownership derives ONLY from the validated
// claim subject, NEVER from ?user=. Bob cannot mutate Alice's budget — not by
// ?user=bob (mismatch) and CRUCIALLY not by forging ?user=alice (the live repro).
func TestSpendCap_MutateOwnership_IDOR(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	gin.SetMode(gin.TestMode)

	org := &organization.Organization{}
	org.Name = "idor-org"

	id := createCapAs(t, org, "alice", `{"title":"alice budget","threshold":100,"enforce":true}`)

	// The LIVE repro: bob forges ?user=alice to match alice's row. MUST be 404 —
	// ?user= is ignored; ownership is bob's validated subject.
	if code := patchCapAs(t, org, id, "bob", "alice", `{"threshold":1}`); code != 404 {
		t.Fatalf("bob PATCH alice's row via ?user=alice = %d, want 404 (the IDOR)", code)
	}
	if code := deleteCapAs(t, org, id, "bob", "alice"); code != 404 {
		t.Fatalf("bob DELETE alice's row via ?user=alice = %d, want 404 (the IDOR)", code)
	}
	// Also the naive ?user=bob mismatch.
	if code := patchCapAs(t, org, id, "bob", "bob", `{"threshold":1}`); code != 404 {
		t.Fatalf("bob PATCH alice's row via ?user=bob = %d, want 404", code)
	}

	// Alice (the validated owner) succeeds even with a bogus ?user=bob in the URL.
	if code := patchCapAs(t, org, id, "alice", "bob", `{"threshold":200}`); code != 200 {
		t.Fatalf("alice PATCH own row = %d, want 200", code)
	}
	if code := deleteCapAs(t, org, id, "alice", ""); code != 204 {
		t.Fatalf("alice DELETE own row = %d, want 204", code)
	}
}

// (HIGH-2) Project-scope hard caps DEGRADE to soft when the project axis is not
// validated (pv=0): still records + warns, but does NOT 402 — a forgeable
// X-Project-Id can neither hard-stop nor be evaded. With pv=1 (a future validated
// project claim) the SAME cap hard-denies. Org/service axes stay hard regardless.
func TestSpendCap_ProjectDegradesToSoftWhenUnvalidated(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	gin.SetMode(gin.TestMode)

	org := &organization.Organization{}
	org.Name = "pv-org"

	// $1 HARD cap on project P; spend $1 on P.
	createCap(t, org, `{"title":"P cap","threshold":100,"enforce":true,"project":"P"}`)
	if code := recordUsage(t, org, `{"user":"pv-org","amount":100,"requestId":"pv1","project":"P"}`); code != 201 {
		t.Fatalf("RecordUsage status = %d", code)
	}

	// pv=0 (today): project axis unvalidated → DEGRADE to soft → allow + warn.
	soft := authorize(t, org, "user=pv-org&project=P&amount=1&pv=0")
	if !soft.Allow {
		t.Fatalf("pv=0 authorize = %+v, want allow (project cap degrades to soft)", soft)
	}
	if soft.WarnPct < 100 {
		t.Fatalf("pv=0 warnPct = %d, want >=100 (still warns at the ceiling)", soft.WarnPct)
	}

	// pv=1 (validated project claim): the SAME cap hard-denies.
	hard := authorize(t, org, "user=pv-org&project=P&amount=1&pv=1")
	if hard.Allow || hard.Reason != "spend_cap" {
		t.Fatalf("pv=1 authorize = %+v, want deny spend_cap (validated project hardens)", hard)
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
