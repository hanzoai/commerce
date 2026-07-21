package catalog

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
	"github.com/hanzoai/commerce/models/catalogentry"
	"github.com/hanzoai/commerce/util/test/ae"
)

// callCatalog drives handler h through a real request whose GetContext resolves
// to the ae SQLite datastore, optionally carrying IAM claims. Returns the
// response status and body bytes.
func callCatalog(t *testing.T, claims *auth.IAMClaims, method, target string, body []byte, h zip.Handler) (int, []byte) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	seed := func(c *zip.Ctx) error {
		c.SetContext(context.Background())
		if claims != nil {
			c.Locals("iam_claims", claims)
		}
		return c.Next()
	}
	app.All("/*", seed, h)

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Fiber().Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func TestPublic_ReturnsProjection_NoAuth(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	// Seed the catalog in the system namespace via a global-admin seed call.
	if code, body := callCatalog(t, &auth.IAMClaims{Owner: "admin"}, http.MethodPost, "/v1/catalog/seed", nil, SeedCatalog); code != 200 {
		t.Fatalf("seed status = %d; body=%s", code, body)
	}

	// Public GET — no auth at all.
	code, body := callCatalog(t, nil, http.MethodGet, "/v1/commerce/catalog?brand=hanzo", nil, Public)
	if code != 200 {
		t.Fatalf("public catalog status = %d; body=%s", code, body)
	}
	var cat catalogentry.Catalog
	if err := json.Unmarshal(body, &cat); err != nil {
		t.Fatalf("projection not JSON: %s", body)
	}
	if cat.Brand != "hanzo" || len(cat.Categories) != 13 || len(cat.Products) == 0 {
		t.Fatalf("bad projection: brand=%s cats=%d products=%d", cat.Brand, len(cat.Categories), len(cat.Products))
	}
}

// TestAdminCatalog_GatesAndCarriesMargin proves the owner=="admin" boundary: an
// org-level admin is refused, the public projection NEVER carries cost/margin, and
// only the admin projection surfaces costCents + a derived marginPct.
func TestAdminCatalog_GatesAndCarriesMargin(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	admin := &auth.IAMClaims{Owner: "admin"}
	// A priced product with an upstream cost: public price $1.00, cost $0.40 →
	// derived margin 60%.
	body, _ := json.Marshal(map[string]any{
		"slug": "margined", "brand": "hanzo", "name": "Margined", "category": "AI",
		"iconKey": "Box", "priceCents": 100, "costCents": 40,
	})
	if code, b := callCatalog(t, admin, http.MethodPost, "/v1/catalog/entries", body, CreateEntry); code != 201 {
		t.Fatalf("create status = %d, want 201; body=%s", code, b)
	}

	// PUBLIC projection must NEVER leak cost or margin — check the raw bytes.
	_, pubBody := callCatalog(t, nil, http.MethodGet, "/v1/commerce/catalog?brand=hanzo", nil, Public)
	if bytes.Contains(pubBody, []byte("costCents")) || bytes.Contains(pubBody, []byte("marginPct")) {
		t.Fatalf("public projection leaked cost/margin: %s", pubBody)
	}

	// Admin catalog: org-level admin (NOT owner=="admin") is refused 403.
	if code, b := callCatalog(t, &auth.IAMClaims{Owner: "acme", IsAdmin: true}, http.MethodGet, "/v1/commerce/admin/catalog?brand=hanzo", nil, AdminCatalog); code != 403 {
		t.Fatalf("org-admin admin-catalog status = %d, want 403; body=%s", code, b)
	}

	// owner=="admin" gets the margin projection.
	code, adminBody := callCatalog(t, admin, http.MethodGet, "/v1/commerce/admin/catalog?brand=hanzo", nil, AdminCatalog)
	if code != 200 {
		t.Fatalf("admin catalog status = %d, want 200; body=%s", code, adminBody)
	}
	var ac catalogentry.AdminCatalog
	if err := json.Unmarshal(adminBody, &ac); err != nil {
		t.Fatalf("admin projection not JSON: %s", adminBody)
	}
	var found bool
	for _, p := range ac.Products {
		if p.Slug == "margined" {
			found = true
			if p.CostCents != 40 || p.MarginPct != 60 {
				t.Fatalf("admin item cost/margin = %d/%v, want 40/60", p.CostCents, p.MarginPct)
			}
		}
	}
	if !found {
		t.Fatal("admin projection missing the margined entry")
	}
}

func TestCreateEntry_RequiresSuperAdmin(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	body, _ := json.Marshal(map[string]any{
		"slug": "myproduct", "brand": "hanzo", "name": "My Product",
		"category": "AI", "iconKey": "Box",
	})

	// Org-level admin (NOT global) — must be rejected 403.
	if code, b := callCatalog(t, &auth.IAMClaims{Owner: "acme", IsAdmin: true}, http.MethodPost, "/v1/catalog/entries", body, CreateEntry); code != 403 {
		t.Fatalf("org-admin create status = %d, want 403 (platform-admin only); body=%s", code, b)
	}

	// Global admin — allowed 201.
	if code, b := callCatalog(t, &auth.IAMClaims{Owner: "admin"}, http.MethodPost, "/v1/catalog/entries", body, CreateEntry); code != 201 {
		t.Fatalf("global-admin create status = %d, want 201; body=%s", code, b)
	}

	// The created entry is now visible in the public projection.
	_, projBody := callCatalog(t, nil, http.MethodGet, "/v1/commerce/catalog?brand=hanzo", nil, Public)
	var cat catalogentry.Catalog
	_ = json.Unmarshal(projBody, &cat)
	found := false
	for _, p := range cat.Products {
		if p.Slug == "myproduct" {
			found = true
		}
	}
	if !found {
		t.Fatal("created entry did not appear in the public projection")
	}
}

func TestCreateEntry_DuplicateSlugRejected(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	body, _ := json.Marshal(map[string]any{
		"slug": "dup", "brand": "hanzo", "name": "Dup", "category": "AI", "iconKey": "Box",
	})
	admin := &auth.IAMClaims{Owner: "admin"}

	if code, _ := callCatalog(t, admin, http.MethodPost, "/v1/catalog/entries", body, CreateEntry); code != 201 {
		t.Fatalf("first create = %d, want 201", code)
	}

	if code, _ := callCatalog(t, admin, http.MethodPost, "/v1/catalog/entries", body, CreateEntry); code != 409 {
		t.Fatalf("duplicate create = %d, want 409", code)
	}
}
