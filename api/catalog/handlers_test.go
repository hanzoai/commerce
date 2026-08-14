package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// callPut drives an update handler mounted at its REAL path pattern, so the
// :slug param resolves. callCatalog's wildcard mount cannot express one.
func callPut(t *testing.T, claims *auth.IAMClaims, pattern, target string, body []byte, h zip.Handler) (int, []byte) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	seed := func(c *zip.Ctx) error {
		c.SetContext(context.Background())
		if claims != nil {
			c.Locals("iam_claims", claims)
		}
		return c.Next()
	}
	app.Put(pattern, seed, h)

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(http.MethodPut, target, reader)
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
	if cat.Brand != "hanzo" || len(cat.Categories) != 10 || len(cat.Products) == 0 {
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

// TestModelCatalog_OneListForEveryCaller is the structural proof against the
// 440-vs-106 split: there is ONE catalog and ONE endpoint, so an anonymous
// visitor and an authenticated customer necessarily see the SAME rows. A model
// a caller cannot use still appears, carrying its price and the honest reason —
// entitlement decides USE, never sight.
func TestModelCatalog_OneListForEveryCaller(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	admin := &auth.IAMClaims{Owner: "admin"}
	rows := []map[string]any{
		{
			"slug": "enso", "name": "Enso", "category": catalogentry.CategoryEnso,
			"spec": map[string]any{
				"vendor": "Hanzo", "family": catalogentry.FamilyEnso, "serves": "enso",
				"modality": "chat", "contextWindow": 1000000, "minTier": "paid", "enabled": true,
			},
			"rates": []map[string]any{
				{"key": "in", "unit": "mtok", "cost": "6.00", "price": "20.00"},
			},
			"published": true,
		},
		{
			"slug": "llama-4-maverick", "name": "Llama 4 Maverick", "category": catalogentry.CategoryModel,
			"spec": map[string]any{
				"vendor": "Meta", "family": catalogentry.FamilyThirdParty, "serves": "openrouter",
				"modality": "chat", "enabled": false, "unavailable": "upstream withdrawn",
			},
			"rates":     []map[string]any{{"key": "in", "unit": "mtok", "cost": "0.15"}},
			"published": true,
		},
	}
	for _, r := range rows {
		b, _ := json.Marshal(r)
		if code, body := callCatalog(t, admin, http.MethodPost, "/v1/catalog/entries", b, CreateEntry); code != 201 {
			t.Fatalf("create %v = %d; body=%s", r["slug"], code, body)
		}
	}

	read := func(claims *auth.IAMClaims) catalogentry.Catalog {
		t.Helper()
		code, body := callCatalog(t, claims, http.MethodGet, "/v1/commerce/catalog?brand=models", nil, Public)
		if code != 200 {
			t.Fatalf("models catalog status = %d; body=%s", code, body)
		}
		var cat catalogentry.Catalog
		if err := json.Unmarshal(body, &cat); err != nil {
			t.Fatalf("not JSON: %s", body)
		}
		return cat
	}

	anon := read(nil)
	authed := read(&auth.IAMClaims{Owner: "acme"})
	if len(anon.Products) != 2 || len(authed.Products) != len(anon.Products) {
		t.Fatalf("anonymous saw %d models, authenticated saw %d — the catalog must not fork on auth",
			len(anon.Products), len(authed.Products))
	}
	for i := range anon.Products {
		if anon.Products[i].Slug != authed.Products[i].Slug {
			t.Fatalf("row %d differs by caller: %s vs %s", i, anon.Products[i].Slug, authed.Products[i].Slug)
		}
	}

	byID := map[string]catalogentry.Item{}
	for _, p := range anon.Products {
		byID[p.Slug] = p
	}
	enso := byID["enso"]
	if enso.Spec == nil || enso.Spec.MinTier != "paid" || enso.Spec.Vendor != "Hanzo" {
		t.Fatalf("enso spec = %+v, want the entitlement + vendor projected publicly", enso.Spec)
	}
	if len(enso.Rates) != 1 || enso.Rates[0].Price != "20.00" {
		t.Fatalf("enso rates = %+v, want the retail price", enso.Rates)
	}
	llama := byID["llama-4-maverick"]
	if llama.Spec.Enabled || llama.Spec.Unavailable == "" {
		t.Fatalf("an unusable model must still be LISTED with an honest reason: %+v", llama.Spec)
	}
	if llama.Spec.Vendor != "Meta" {
		t.Fatalf("vendor = %q, want the plain vendor name %q", llama.Spec.Vendor, "Meta")
	}
	// Derived retail: cost 0.15 x the default 1.20 markup.
	if len(llama.Rates) != 1 || llama.Rates[0].Price != "0.18" {
		t.Fatalf("derived retail = %+v, want 0.18", llama.Rates)
	}
	// Cost and margin never reach a public reader, model rows included.
	blob, _ := json.Marshal(anon)
	if bytes.Contains(blob, []byte(`"cost"`)) || bytes.Contains(blob, []byte(`"marginPct"`)) {
		t.Fatalf("public model catalog leaked cost/margin: %s", blob)
	}
}

// TestModelPrice_EditableFromAdmin proves admin.hanzo.ai can set a model's
// retail price and markup through the SAME CRUD every other catalog entry uses,
// and that the change shows up as a margin on the admin projection.
func TestModelPrice_EditableFromAdmin(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	admin := &auth.IAMClaims{Owner: "admin"}
	create, _ := json.Marshal(map[string]any{
		"slug": "zen5", "name": "Zen 5", "category": catalogentry.CategoryZen,
		"spec":      map[string]any{"vendor": "Hanzo", "family": catalogentry.FamilyZen, "enabled": true},
		"rates":     []map[string]any{{"key": "in", "unit": "mtok", "cost": "1.00"}},
		"published": true,
	})
	if code, b := callCatalog(t, admin, http.MethodPost, "/v1/catalog/entries", create, CreateEntry); code != 201 {
		t.Fatalf("create = %d; %s", code, b)
	}

	edit, _ := json.Marshal(map[string]any{
		"name":      "Zen 5",
		"category":  catalogentry.CategoryZen,
		"published": true,
		"markup":    "2.5",
		"rates":     []map[string]any{{"key": "in", "unit": "mtok", "cost": "1.00", "price": "4.00"}},
	})
	if code, b := callPut(t, admin, "/v1/catalog/entries/*", "/v1/catalog/entries/zen5", edit, UpdateEntry); code != 200 {
		t.Fatalf("update = %d; %s", code, b)
	}

	code, body := callCatalog(t, admin, http.MethodGet, "/v1/commerce/admin/catalog?brand=models", nil, AdminCatalog)
	if code != 200 {
		t.Fatalf("admin models catalog = %d; %s", code, body)
	}
	var ac catalogentry.AdminCatalog
	if err := json.Unmarshal(body, &ac); err != nil {
		t.Fatalf("not JSON: %s", body)
	}
	if len(ac.Products) != 1 {
		t.Fatalf("admin models = %d, want 1", len(ac.Products))
	}
	p := ac.Products[0]
	if p.Markup != "2.5" {
		t.Fatalf("markup = %q, want 2.5", p.Markup)
	}
	if len(p.AdminRates) != 1 || p.AdminRates[0].Cost != "1.00" || p.AdminRates[0].Price != "4.00" {
		t.Fatalf("admin rate = %+v, want cost 1.00 / price 4.00", p.AdminRates)
	}
	if p.AdminRates[0].MarginPct == nil || *p.AdminRates[0].MarginPct != 75 {
		t.Fatalf("margin = %v, want 75", p.AdminRates[0].MarginPct)
	}
}

// TestSyncModels_GatedAndCostOnly proves the write seam: platform-admin only,
// and a syncer's attempt to state retail is refused at the boundary — it may
// only move cost.
func TestSyncModels_GatedAndCostOnly(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	rows, _ := json.Marshal([]map[string]any{{
		"slug": "gpt-5.6", "name": "GPT-5.6",
		"spec":  map[string]any{"vendor": "OpenAI", "family": catalogentry.FamilyThirdParty, "serves": "openrouter", "enabled": true},
		"costs": []map[string]any{{"key": "in", "unit": "mtok", "cost": "1.25", "price": "0.01"}},
	}})

	// An org-level admin may not sync the platform catalog.
	if code, b := callCatalog(t, &auth.IAMClaims{Owner: "acme", IsAdmin: true}, http.MethodPost, "/v1/catalog/models", rows, SyncModels); code != 403 {
		t.Fatalf("org-admin sync = %d, want 403; %s", code, b)
	}

	admin := &auth.IAMClaims{Owner: "admin"}
	code, body := callCatalog(t, admin, http.MethodPost, "/v1/catalog/models", rows, SyncModels)
	if code != 200 {
		t.Fatalf("sync = %d; %s", code, body)
	}
	var stats catalogentry.SyncStats
	if err := json.Unmarshal(body, &stats); err != nil || stats.Created != 1 {
		t.Fatalf("stats = %+v err=%v, want created 1", stats, err)
	}

	_, adminBody := callCatalog(t, admin, http.MethodGet, "/v1/commerce/admin/catalog?brand=models", nil, AdminCatalog)
	var ac catalogentry.AdminCatalog
	_ = json.Unmarshal(adminBody, &ac)
	if len(ac.Products) != 1 {
		t.Fatalf("synced models = %d, want 1", len(ac.Products))
	}
	r := ac.Products[0].AdminRates[0]
	if r.Cost != "1.25" {
		t.Fatalf("cost = %q, want 1.25", r.Cost)
	}
	// The syncer's "price" was dropped; retail is cost x the default markup.
	if r.Price != "1.5" {
		t.Fatalf("price = %q, want 1.5 (cost x default markup) — a sync must never set retail", r.Price)
	}
}

// A product's API ADDRESS travels with a deploy, so this door refuses to store
// one — and refuses out loud.
//
// It used to take the whole entity and answer 200, which meant an admin could set
// apiPath to anything, watch it render back, and lose it at the next restart when
// catalogentry.Correct read the snapshot over the row. A write surface that
// reports durable success for a value it cannot keep is worse than one that says
// no: nothing was wrong until a pod cycled, and then nothing said why.
//
// The read-modify-write the console actually performs still has to work, so the
// refusal is for a CHANGE, not for the field being present.
func TestUpdateEntry_RefusesToMoveTheAddress(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	admin := &auth.IAMClaims{Owner: "admin"}
	create, _ := json.Marshal(map[string]any{
		"slug": "gateway", "name": "Gateway", "category": "Network",
		"apiPath": "/v1/gateway", "apiRoute": "api.hanzo.ai/v1/gateway", "published": true,
	})
	if code, b := callCatalog(t, admin, http.MethodPost, "/v1/catalog/entries", create, CreateEntry); code != 201 {
		t.Fatalf("create = %d; %s", code, b)
	}

	move, _ := json.Marshal(map[string]any{"name": "Gateway", "category": "Network", "apiPath": "/v1/made-up"})
	code, b := callPut(t, admin, "/v1/catalog/entries/*", "/v1/catalog/entries/gateway", move, UpdateEntry)
	if code != 409 {
		t.Fatalf("moving apiPath = %d, want 409; %s", code, b)
	}
	if !strings.Contains(string(b), "hanzo-catalog.json") {
		t.Errorf("the refusal does not say where the address IS edited: %s", b)
	}

	// The whole-entity round-trip the console performs: same address back, plus
	// the defaulted kind the projection always states. Both must be accepted.
	echo, _ := json.Marshal(map[string]any{
		"name": "Gateway, renamed", "category": "Network",
		"apiPath": "/v1/gateway", "apiRoute": "api.hanzo.ai/v1/gateway",
		"kind": catalogentry.KindService, "published": true,
	})
	if code, b := callPut(t, admin, "/v1/catalog/entries/*", "/v1/catalog/entries/gateway", echo, UpdateEntry); code != 200 {
		t.Fatalf("echoing the address back = %d, want 200 — the console reads a row and PUTs it whole; %s", code, b)
	}
}
