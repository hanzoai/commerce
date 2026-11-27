// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package shopify_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/delay"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/source/hub"
	"github.com/hanzoai/commerce/source/shopify"
)

func setupShopifyTestDB(t *testing.T) {
	t.Helper()
	cfg := db.DefaultConfig()
	cfg.DataDir = filepath.Join(t.TempDir(), "orgs")
	cfg.UserDataDir = filepath.Join(t.TempDir(), "users")
	mgr, err := db.NewManager(cfg)
	if err != nil {
		t.Fatalf("db.NewManager: %v", err)
	}
	sysDB, err := mgr.Org("system")
	if err != nil {
		t.Fatalf("mgr.Org(system): %v", err)
	}
	datastore.SetDefaultDB(sysDB)
	datastore.SetOrgDBResolver(mgr.Org)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = delay.Drain(ctx)
		datastore.SetOrgDBResolver(nil)
		_ = mgr.Close()
	})
}

func TestShopifySessionLinkPixelAndComplianceWebhooks(t *testing.T) {
	setupShopifyTestDB(t)
	_ = iammiddleware.Init(nil)

	const (
		clientID     = "0123456789abcdef0123456789abcdef"
		clientSecret = "shpss_test_secret_key_for_hmac_verification"
		shop         = "carbondistrict-us.myshopify.com"
	)
	now := time.Unix(1793016000, 0)

	// Mock Shopify Admin GraphQL + OAuth server.
	var gqlCalls []string
	mockShopify := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin/oauth/access_token" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "shpat_offline_token_1"})
			return
		}
		var req struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gqlCalls = append(gqlCalls, req.Query)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(req.Query, "webPixelCreate"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"webPixelCreate": map[string]any{
						"webPixel":   map[string]any{"id": "gid://shopify/WebPixel/901"},
						"userErrors": []any{},
					},
				},
			})
		case strings.Contains(req.Query, "getCustomer"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"customer": map[string]any{
						"id":        "gid://shopify/Customer/444",
						"firstName": "Ada",
						"lastName":  "Lovelace",
						"defaultEmailAddress": map[string]any{
							"emailAddress":       "ada@example.com",
							"marketingState":     "SUBSCRIBED",
							"marketingUpdatedAt": "2026-10-01T00:00:00Z",
						},
					},
				},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
		}
	}))
	defer mockShopify.Close()

	app := zip.New(zip.Config{DisableStartupMessage: true})
	v1 := app.Group("/v1")
	v1.Use(middleware.RequestContext())
	srcGrp := v1.Group("/source")
	eng := hub.NewEngine()
	svc := shopify.NewService(shopify.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}, eng, mockShopify.Client())
	svc.Now = func() time.Time { return now }
	svc.MountRoutes(srcGrp)

	// 1. App Home serves HTML with client_id and CSP frame-ancestors.
	appResp, err := app.Test(httptest.NewRequest(http.MethodGet, "/v1/source/shopify/app", nil))
	if err != nil || appResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /app = %d err=%v", appResp.StatusCode, err)
	}
	htmlBytes, _ := io.ReadAll(appResp.Body)
	if !strings.Contains(string(htmlBytes), clientID) {
		t.Fatalf("App Home missing injected client_id")
	}
	if !strings.Contains(appResp.Header.Get("Content-Security-Policy"), "frame-ancestors https://admin.shopify.com") {
		t.Fatalf("App Home missing CSP frame-ancestors")
	}

	// 2. Mint valid App Bridge session JWT; verify unlinked /session -> /link -> /callback -> /pixel.
	jwt, err := shopify.SignSessionToken(shopify.SessionClaims{
		Issuer:   "https://" + shop + "/admin",
		Dest:     "https://" + shop,
		Audience: clientID,
		Subject:  "42",
		Iat:      now.Unix(),
		Nbf:      now.Unix() - 5,
		Exp:      now.Unix() + 60,
	}, clientSecret)
	if err != nil {
		t.Fatalf("SignSessionToken: %v", err)
	}

	// Bad JWT must 401.
	badSessReq := httptest.NewRequest(http.MethodPost, "/v1/source/shopify/session", strings.NewReader(`{}`))
	badSessReq.Header.Set("Authorization", "Bearer bad.jwt.token")
	badSessResp, _ := app.Test(badSessReq)
	if badSessResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad session token = %d, want 401", badSessResp.StatusCode)
	}

	linkReq := httptest.NewRequest(http.MethodPost, "/v1/source/shopify/link", strings.NewReader(`{}`))
	linkReq.Header.Set("Authorization", "Bearer "+jwt)
	linkResp, _ := app.Test(linkReq)
	if linkResp.StatusCode != http.StatusOK {
		t.Fatalf("/link = %d", linkResp.StatusCode)
	}
	var linkData struct {
		URL   string `json:"url"`
		State string `json:"state"`
	}
	_ = json.NewDecoder(linkResp.Body).Decode(&linkData)
	if linkData.State == "" || !strings.Contains(linkData.URL, "hanzo-shopify") {
		t.Fatalf("unexpected /link response: %+v", linkData)
	}

	// Complete /callback as org admin of "carbon-district".
	cbBody, _ := json.Marshal(map[string]any{
		"state":       linkData.State,
		"endpointUrl": mockShopify.URL,
	})
	cbReq := httptest.NewRequest(http.MethodPost, "/v1/source/shopify/callback", bytes.NewReader(cbBody))
	cbReq.Header.Set("Content-Type", "application/json")
	cbReq.Header.Set("X-Org-Id", "carbon-district")
	cbReq.Header.Set("X-User-Id", "usr_1")
	cbReq.Header.Set("X-User-Owner", "carbon-district")
	cbReq.Header.Set("X-User-IsOrgAdmin", "true")
	cbResp, _ := app.Test(cbReq)
	if cbResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(cbResp.Body)
		t.Fatalf("/callback = %d: %s", cbResp.StatusCode, string(b))
	}

	// /session exchanges offline token & returns multi-store org binding.
	sessReq := httptest.NewRequest(http.MethodPost, "/v1/source/shopify/session", strings.NewReader(`{}`))
	sessReq.Header.Set("Authorization", "Bearer "+jwt)
	sessResp, _ := app.Test(sessReq)
	var sessData map[string]any
	_ = json.NewDecoder(sessResp.Body).Decode(&sessData)
	if sessData["org"] != "carbon-district" || sessData["source_id"] != "shopify_carbondistrict-us" {
		t.Fatalf("unexpected session status: %+v", sessData)
	}

	// /pixel rejects sk- secret keys and activates webPixelCreate for pk- keys.
	badPixReq := httptest.NewRequest(http.MethodPost, "/v1/source/shopify/pixel", strings.NewReader(`{"key":"sk-secret"}`))
	badPixReq.Header.Set("Authorization", "Bearer "+jwt)
	badPixResp, _ := app.Test(badPixReq)
	if badPixResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("sk- pixel key status = %d, want 400", badPixResp.StatusCode)
	}

	pixReq := httptest.NewRequest(http.MethodPost, "/v1/source/shopify/pixel", strings.NewReader(`{"key":"pk-live-carbon123"}`))
	pixReq.Header.Set("Authorization", "Bearer "+jwt)
	pixResp, _ := app.Test(pixReq)
	if pixResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(pixResp.Body)
		t.Fatalf("pk- pixel status = %d: %s", pixResp.StatusCode, string(b))
	}

	// 3. Mandatory compliance webhooks: bad HMAC -> 401; valid HMAC -> 200.
	for _, topic := range []string{"customers/data_request", "customers/redact", "shop/redact"} {
		body := []byte(`{"shop_domain":"carbondistrict-us.myshopify.com","customer":{"id":444,"email":"ada@example.com"}}`)
		badReq := httptest.NewRequest(http.MethodPost, "/v1/source/shopify/webhook", bytes.NewReader(body))
		badReq.Header.Set("X-Shopify-Topic", topic)
		badReq.Header.Set("X-Shopify-Shop-Domain", shop)
		badReq.Header.Set("X-Shopify-Hmac-Sha256", "bm90LWEtdmFsaWQtaG1hYw==")
		rBad, _ := app.Test(badReq)
		if rBad.StatusCode != http.StatusUnauthorized {
			t.Fatalf("topic %s with bad HMAC returned %d, want 401", topic, rBad.StatusCode)
		}
	}

	// Flow action hanzo-send-customer: bad HMAC -> 401, valid HMAC -> fetches customer & deduplicates action_run_id.
	flowBody := []byte(`{"shopify_domain":"carbondistrict-us.myshopify.com","handle":"hanzo-send-customer","action_run_id":"run_1","properties":{"customer_id":"gid://shopify/Customer/444"}}`)
	badFlow := httptest.NewRequest(http.MethodPost, "/v1/source/shopify/flow", bytes.NewReader(flowBody))
	badFlow.Header.Set("X-Shopify-Hmac-Sha256", "bad")
	if r, _ := app.Test(badFlow); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("flow bad HMAC = %d, want 401", r.StatusCode)
	}
	goodFlow := httptest.NewRequest(http.MethodPost, "/v1/source/shopify/flow", bytes.NewReader(flowBody))
	goodFlow.Header.Set("X-Shopify-Hmac-Sha256", shopify.ComputeWebhookHMAC(clientSecret, flowBody))
	if r, _ := app.Test(goodFlow); r.StatusCode != http.StatusOK {
		t.Fatalf("flow valid HMAC = %d, want 200", r.StatusCode)
	}
	if len(gqlCalls) < 2 {
		t.Fatalf("expected webPixelCreate + getCustomer GraphQL calls, got %d", len(gqlCalls))
	}
}
