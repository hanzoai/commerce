package plan

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/util/test/ae"
)

// callPlan drives the REAL AdminRoute wiring (so :slug binds) through a request
// whose context resolves to the ae SQLite datastore, optionally carrying IAM
// claims. The claims-injecting middleware is passed as AdminRoute's arg, so it
// runs ahead of every handler exactly as the bundle middleware does in prod.
// Returns the response status and body.
func callPlan(t *testing.T, claims *auth.IAMClaims, method, target string, body []byte) (int, []byte) {
	t.Helper()
	return callPlanSeed(t, claims, method, target, body, nil)
}

func callPlanSeed(t *testing.T, claims *auth.IAMClaims, method, target string, body []byte, src SeedSource) (int, []byte) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	inject := func(c *zip.Ctx) error {
		c.SetContext(context.Background())
		if claims != nil {
			c.Locals("iam_claims", claims)
		}
		return c.Next()
	}
	AdminRoute(app.Group("/v1"), src, inject)

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

var admin = &auth.IAMClaims{Owner: "admin"}

// TestCRUD_RequiresSuperAdmin: a non-platform-admin (org owner) and an anonymous
// caller are BOTH refused every mutating verb — the plan authority is cross-tenant
// pricing, so an org admin must not edit it.
func TestCRUD_RequiresSuperAdmin(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	orgAdmin := &auth.IAMClaims{Owner: "acme", IsAdmin: true} // org-level admin, NOT platform
	body := []byte(`{"slug":"x","name":"X","price":100}`)

	for _, tt := range []struct {
		name   string
		claims *auth.IAMClaims
		method string
		target string
	}{
		{"org-admin create", orgAdmin, http.MethodPost, "/v1/plans/entries"},
		{"anon create", nil, http.MethodPost, "/v1/plans/entries"},
		{"org-admin update", orgAdmin, http.MethodPut, "/v1/plans/entries/x"},
		{"org-admin delete", orgAdmin, http.MethodDelete, "/v1/plans/entries/x"},
		{"org-admin list", orgAdmin, http.MethodGet, "/v1/plans/entries"},
	} {
		code, _ := callPlan(t, tt.claims, tt.method, tt.target, body)
		if code != 403 {
			t.Errorf("%s: status = %d, want 403 (super-admin only)", tt.name, code)
		}
	}
}

// TestCreate_And_RejectDup: a platform admin creates a plan; a duplicate slug is
// rejected 409.
func TestCreate_And_RejectDup(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	body := []byte(`{"slug":"pro-x","name":"Pro X","category":"personal","price":2000,"priceAnnual":1600,"currency":"usd"}`)
	if code, b := callPlan(t, admin, http.MethodPost, "/v1/plans/entries", body); code != 201 {
		t.Fatalf("create status = %d; body=%s", code, b)
	}
	if code, _ := callPlan(t, admin, http.MethodPost, "/v1/plans/entries", body); code != 409 {
		t.Fatalf("duplicate create status = %d, want 409", code)
	}

	// Missing slug → 400.
	if code, _ := callPlan(t, admin, http.MethodPost, "/v1/plans/entries", []byte(`{"name":"No Slug"}`)); code != 400 {
		t.Fatalf("no-slug create status = %d, want 400", code)
	}
}

// TestUpdate_SlugImmutable: a body slug that differs from the path slug is
// rejected (no rename); a matching/absent slug updates the priced fields and
// preserves the identity.
func TestUpdate_SlugImmutable(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	create := []byte(`{"slug":"pro-x","name":"Pro X","category":"personal","price":2000,"currency":"usd"}`)
	if code, b := callPlan(t, admin, http.MethodPost, "/v1/plans/entries", create); code != 201 {
		t.Fatalf("create: %d %s", code, b)
	}

	// Rename attempt → 400.
	rename := []byte(`{"slug":"pro-y","price":9900}`)
	if code, _ := callPlan(t, admin, http.MethodPut, "/v1/plans/entries/pro-x", rename); code != 400 {
		t.Fatalf("rename status = %d, want 400 (slug immutable)", code)
	}

	// Legit edit (matching slug) → 200, price updated, slug preserved.
	edit := []byte(`{"slug":"pro-x","price":9900}`)
	if code, b := callPlan(t, admin, http.MethodPut, "/v1/plans/entries/pro-x", edit); code != 200 {
		t.Fatalf("edit status = %d; body=%s", code, b)
	}
	got := plan.New(plan.AuthorityDB(context.Background()))
	if ok, _ := got.Query().Filter("Slug=", "pro-x").Get(); !ok {
		t.Fatal("pro-x missing after edit")
	}
	if got.Price != 9900 {
		t.Fatalf("edited price = %d, want 9900", got.Price)
	}
	// The renamed slug must never have been created.
	orphan := plan.New(plan.AuthorityDB(context.Background()))
	if ok, _ := orphan.Query().Filter("Slug=", "pro-y").Get(); ok {
		t.Fatal("rename created an orphan pro-y row")
	}
}

// TestPreserves0VsNull: a free ($0) plan and a custom (null → contactSales) plan
// both store Price=0 but are DISTINGUISHED by ContactSales — the CRUD never
// coerces null→0 into a chargeable $0.
func TestPreserves0VsNull(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	free := []byte(`{"slug":"free-x","name":"Free","category":"personal","price":0,"currency":"usd"}`)
	custom := []byte(`{"slug":"custom-x","name":"Custom","category":"enterprise","price":0,"contactSales":true,"currency":"usd"}`)
	for _, b := range [][]byte{free, custom} {
		if code, resp := callPlan(t, admin, http.MethodPost, "/v1/plans/entries", b); code != 201 {
			t.Fatalf("create %s: %d %s", b, code, resp)
		}
	}

	db := plan.AuthorityDB(context.Background())
	fp := plan.New(db)
	if ok, _ := fp.Query().Filter("Slug=", "free-x").Get(); !ok {
		t.Fatal("free-x missing")
	}
	if fp.Price != 0 || fp.ContactSales {
		t.Fatalf("free-x = price %d contactSales %v, want 0/false (a real $0 charge)", fp.Price, fp.ContactSales)
	}
	cp := plan.New(db)
	if ok, _ := cp.Query().Filter("Slug=", "custom-x").Get(); !ok {
		t.Fatal("custom-x missing")
	}
	if cp.Price != 0 || !cp.ContactSales {
		t.Fatalf("custom-x = price %d contactSales %v, want 0/true (null, NOT a $0 charge)", cp.Price, cp.ContactSales)
	}
}

// TestSeedEndpoint: POST /plans/seed upserts the injected source (idempotent,
// non-destructive) and is super-admin gated.
func TestSeedEndpoint(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	src := SeedSource(func() []*plan.Plan {
		return []*plan.Plan{
			{Slug: "seed-a", Category: "personal", Name: "A", Price: 100},
			{Slug: "seed-b", Category: "dns", Name: "B", Price: 200},
		}
	})

	// Non-admin refused.
	if code, _ := callPlanSeed(t, &auth.IAMClaims{Owner: "acme"}, http.MethodPost, "/v1/plans/seed", nil, src); code != 403 {
		t.Fatalf("non-admin seed = %d, want 403", code)
	}

	// Admin seeds two.
	code, body := callPlanSeed(t, admin, http.MethodPost, "/v1/plans/seed", nil, src)
	if code != 200 {
		t.Fatalf("seed status = %d; body=%s", code, body)
	}
	var res struct {
		Created int `json:"created"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("seed response not JSON: %s", body)
	}
	if res.Created != 2 {
		t.Fatalf("seed created %d, want 2", res.Created)
	}

	// Re-seed is idempotent.
	if _, b := callPlanSeed(t, admin, http.MethodPost, "/v1/plans/seed", nil, src); true {
		_ = json.Unmarshal(b, &res)
		if res.Created != 0 {
			t.Fatalf("re-seed created %d, want 0 (idempotent)", res.Created)
		}
	}
}
