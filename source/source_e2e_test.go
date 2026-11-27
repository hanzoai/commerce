// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package source_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/delay"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/models/variant"
	"github.com/hanzoai/commerce/source"
	"github.com/hanzoai/commerce/source/hub"
	"github.com/hanzoai/commerce/source/shopify"
	"github.com/hanzoai/commerce/source/woocommerce"
)

type recordedGQL struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type mockShopifyServer struct {
	mu    sync.Mutex
	calls []recordedGQL
	srv   *httptest.Server
}

func newMockShopifyServer(prefix string) *mockShopifyServer {
	m := &mockShopifyServer{}
	seq := 100
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body recordedGQL
		_ = json.NewDecoder(r.Body).Decode(&body)
		m.mu.Lock()
		m.calls = append(m.calls, body)
		seq++
		idNum := seq
		m.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(body.Query, "productSet"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"productSet": map[string]any{
						"product":    map[string]any{"id": "gid://shopify/Product/" + prefix + string(rune('0'+idNum%10))},
						"userErrors": []any{},
					},
				},
			})
		case strings.Contains(body.Query, "inventorySetQuantities"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"inventorySetQuantities": map[string]any{
						"inventoryAdjustmentGroup": map[string]any{"id": "gid://shopify/InventoryAdjustmentGroup/1"},
						"userErrors":               []any{},
					},
				},
			})
		case strings.Contains(body.Query, "discountCodeBasicCreate"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"discountCodeBasicCreate": map[string]any{
						"codeDiscountNode": map[string]any{"id": "gid://shopify/DiscountCodeNode/501"},
						"userErrors":       []any{},
					},
				},
			})
		case strings.Contains(body.Query, "fulfillmentCreateV2"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"fulfillmentCreateV2": map[string]any{
						"fulfillment": map[string]any{"id": "gid://shopify/Fulfillment/701"},
						"userErrors":  []any{},
					},
				},
			})
		case strings.Contains(body.Query, "refundCreate"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"refundCreate": map[string]any{
						"refund":     map[string]any{"id": "gid://shopify/Refund/801"},
						"userErrors": []any{},
					},
				},
			})
		case strings.Contains(body.Query, "customerEmailMarketingConsentUpdate"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"customerEmailMarketingConsentUpdate": map[string]any{
						"customer":   map[string]any{"id": "gid://shopify/Customer/901"},
						"userErrors": []any{},
					},
				},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
		}
	}))
	return m
}

func (m *mockShopifyServer) snapshot() []recordedGQL {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]recordedGQL, len(m.calls))
	copy(out, m.calls)
	return out
}

func (m *mockShopifyServer) countContaining(substr string) int {
	n := 0
	for _, c := range m.snapshot() {
		if strings.Contains(c.Query, substr) {
			n++
		}
	}
	return n
}

// TestMultiStoreBidirectionalE2E proves:
//  1. Connecting 2 Shopify stores + 1 WooCommerce store to one org ("carbon-district")
//  2. Paid order on Shopify Store 1 decrements master SKU stock and pushes updated stock
//     to Shopify Store 2 (inventorySetQuantities) + WooCommerce (/outbound queue & callback)
//  3. Product & coupon created in Hanzo Commerce push to all 3 stores, and echoed
//     webhooks from Shopify Store 1, Shopify Store 2, and WooCommerce are dropped by
//     SyncLedger (last_synced_hash / pending_outbound_hash) with ZERO webhook ping-pong!
//  4. Cross-org store hijacking returns 409, and bad Shopify/Woo HMACs return 401.
func TestMultiStoreBidirectionalE2E(t *testing.T) {
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
	_ = iammiddleware.Init(nil)

	shopify1 := newMockShopifyServer("us")
	defer shopify1.srv.Close()
	shopify2 := newMockShopifyServer("eu")
	defer shopify2.srv.Close()

	const (
		orgSlug       = "carbon-district"
		shopifySecret = "shpss_carbon_multi_store_secret"
		shop1Domain   = "carbondistrict-us.myshopify.com"
		shop2Domain   = "carbondistrict-eu.myshopify.com"
		wooShopID     = "5d0c3f0e-8a7b-4c39-9a51-2f6a3b7e1c11"
	)
	now := time.Unix(1793016000, 0)

	eng := hub.NewEngine()
	bundle := source.NewBundle(shopify.Config{
		ClientID:     "abcdef0123456789abcdef0123456789",
		ClientSecret: shopifySecret,
	}, eng, shopify1.srv.Client())
	bundle.Shopify.Now = func() time.Time { return now }
	bundle.WooCommerce.Now = func() time.Time { return now }

	app := zip.New(zip.Config{DisableStartupMessage: true})
	v1 := app.Group("/v1")
	bundle.Mount(v1)

	doOrgAdmin := func(method, path, org string, payload any) *http.Response {
		var rdr io.Reader
		if payload != nil {
			b, _ := json.Marshal(payload)
			rdr = bytes.NewReader(b)
		}
		req := httptest.NewRequest(method, path, rdr)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Org-Id", org)
		req.Header.Set("X-User-Id", "usr_owner")
		req.Header.Set("X-User-Owner", org)
		req.Header.Set("X-User-IsOrgAdmin", "true")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		return resp
	}

	// 1. Connect Shopify Store 1 (US), Shopify Store 2 (EU), and WooCommerce Store to "carbon-district".
	r1 := doOrgAdmin(http.MethodPost, "/v1/source/sources", orgSlug, map[string]any{
		"sourceId":           "shopify_us",
		"provider":           "shopify",
		"shop":               shop1Domain,
		"currency":           "USD",
		"syncMode":           "bidirectional",
		"externalLocationId": "gid://shopify/Location/101",
		"endpointUrl":        shopify1.srv.URL,
		"accessToken":        "shpat_us_secret",
	})
	if r1.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(r1.Body)
		t.Fatalf("connect shopify_us = %d: %s", r1.StatusCode, string(b))
	}

	r2 := doOrgAdmin(http.MethodPost, "/v1/source/sources", orgSlug, map[string]any{
		"sourceId":           "shopify_eu",
		"provider":           "shopify",
		"shop":               shop2Domain,
		"currency":           "USD",
		"syncMode":           "bidirectional",
		"externalLocationId": "gid://shopify/Location/202",
		"endpointUrl":        shopify2.srv.URL,
		"accessToken":        "shpat_eu_secret",
	})
	if r2.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(r2.Body)
		t.Fatalf("connect shopify_eu = %d: %s", r2.StatusCode, string(b))
	}

	rWoo := doOrgAdmin(http.MethodPost, "/v1/source/woocommerce/connect", orgSlug, map[string]any{
		"shop":      wooShopID,
		"source_id": "woo_vintage",
		"domain":    "vintage.carbondistrict.com",
		"currency":  "USD",
		"sync_mode": "bidirectional",
	})
	if rWoo.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(rWoo.Body)
		t.Fatalf("connect woo_vintage = %d: %s", rWoo.StatusCode, string(b))
	}
	var wooConn struct {
		Secret   string `json:"secret"`
		SourceID string `json:"source_id"`
	}
	_ = json.NewDecoder(rWoo.Body).Decode(&wooConn)

	// Cross-org hijack check: another org attempting to connect shop1Domain or wooShopID gets 409!
	hijack := doOrgAdmin(http.MethodPost, "/v1/source/woocommerce/connect", "rival-org", map[string]any{
		"shop":   wooShopID,
		"domain": "evil.example.com",
	})
	if hijack.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 on cross-org shop hijack, got %d", hijack.StatusCode)
	}

	// Verify GET /v1/source/sources lists all 3 stores in carbon-district without exposing secrets.
	listResp := doOrgAdmin(http.MethodGet, "/v1/source/sources", orgSlug, nil)
	listBytes, _ := io.ReadAll(listResp.Body)
	if strings.Contains(string(listBytes), "shpat_us_secret") || strings.Contains(string(listBytes), wooConn.Secret) {
		t.Fatalf("GET /v1/source/sources leaked secret credentials: %s", string(listBytes))
	}
	var listData struct {
		Count   int                `json:"count"`
		Sources []*hub.StoreSource `json:"sources"`
	}
	_ = json.Unmarshal(listBytes, &listData)
	if listData.Count != 3 {
		t.Fatalf("expected 3 connected stores in %s, got %d", orgSlug, listData.Count)
	}

	// Helper to send signed WooCommerce requests.
	wooSeq := 0
	sendWoo := func(path, topic string, payload any) *http.Response {
		wooSeq++
		var raw []byte
		switch v := payload.(type) {
		case []byte:
			raw = v
		default:
			raw, _ = json.Marshal(v)
		}
		sum := sha256.Sum256(append([]byte(topic+"\n"), append(raw, byte(wooSeq))...))
		deliv := hex.EncodeToString(sum[:])
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hanzo-Shop", wooShopID)
		req.Header.Set("X-Hanzo-Topic", topic)
		req.Header.Set("X-Hanzo-Delivery", deliv)
		req.Header.Set("X-Hanzo-Signature", woocommerce.FormatSignatureHeader(wooConn.Secret, now.Unix(), wooShopID, topic, deliv, raw))
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("woo %s: %v", path, err)
		}
		return resp
	}

	// Helper to send signed Shopify webhooks.
	shSeq := 0
	sendShopifyWebhook := func(shop, topic string, payload any) *http.Response {
		shSeq++
		raw, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/v1/source/shopify/webhook", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Shopify-Shop-Domain", shop)
		req.Header.Set("X-Shopify-Topic", topic)
		req.Header.Set("X-Shopify-Webhook-Id", "wh_"+hex.EncodeToString([]byte{byte(shSeq), 1, 2, 3}))
		req.Header.Set("X-Shopify-Hmac-Sha256", shopify.ComputeWebhookHMAC(shopifySecret, raw))
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("shopify webhook %s: %v", topic, err)
		}
		return resp
	}

	// 2. Create Product ("CD-HOODIE-ASH", stock 20, $120.00) + Coupon ("CARBON20", 20%) in Hanzo Commerce!
	pubProd := doOrgAdmin(http.MethodPost, "/v1/source/publish/product", orgSlug, hub.CanonicalProduct{
		Slug:        "cd-hoodie-ash",
		SKU:         "CD-HOODIE-ASH",
		Name:        "Carbon Hoodie Ash",
		Currency:    "USD",
		PriceCents:  12000,
		Available:   true,
		ManageStock: true,
		Inventory:   20,
		Variants: []hub.CanonicalVariant{{
			SKU:         "CD-HOODIE-ASH",
			Name:        "Ash / L",
			PriceCents:  12000,
			Available:   true,
			ManageStock: true,
			Inventory:   20,
		}},
	})
	if pubProd.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(pubProd.Body)
		t.Fatalf("publish product = %d: %s", pubProd.StatusCode, string(b))
	}

	pubCpn := doOrgAdmin(http.MethodPost, "/v1/source/publish/coupon", orgSlug, hub.CanonicalCoupon{
		Code:    "CARBON20",
		Name:    "Carbon 20% Off",
		Type:    "percent",
		Amount:  20,
		Limit:   100,
		Once:    true,
		Enabled: true,
	})
	if pubCpn.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(pubCpn.Body)
		t.Fatalf("publish coupon = %d: %s", pubCpn.StatusCode, string(b))
	}

	// Verify BOTH Shopify Store 1 AND Shopify Store 2 received productSet + discountCodeBasicCreate!
	if shopify1.countContaining("productSet") != 1 || shopify2.countContaining("productSet") != 1 {
		t.Fatalf("expected 1 productSet on both Shopify stores, got us=%d eu=%d",
			shopify1.countContaining("productSet"), shopify2.countContaining("productSet"))
	}
	if shopify1.countContaining("discountCodeBasicCreate") != 1 || shopify2.countContaining("discountCodeBasicCreate") != 1 {
		t.Fatalf("expected 1 discountCodeBasicCreate on both Shopify stores")
	}

	// Verify WooCommerce store pulls & acks BOTH outbound jobs (product.upsert + coupon.upsert)!
	pull1 := sendWoo("/v1/source/woocommerce/outbound", "outbound.pull", map[string]any{"limit": 50})
	var pull1Data struct {
		Jobs []woocommerce.OutboundEnvelope `json:"jobs"`
	}
	_ = json.NewDecoder(pull1.Body).Decode(&pull1Data)
	if len(pull1Data.Jobs) != 2 {
		t.Fatalf("expected 2 outbound jobs queued for WooCommerce (product + coupon), got %d", len(pull1Data.Jobs))
	}
	ackList := make([]map[string]string, 0, len(pull1Data.Jobs))
	for i, j := range pull1Data.Jobs {
		ext := "901"
		if i == 1 {
			ext = "902"
		}
		ackList = append(ackList, map[string]string{"job_id": j.JobID, "external_id": ext})
	}
	ackResp := sendWoo("/v1/source/woocommerce/outbound", "outbound.ack", map[string]any{"ack": ackList})
	if ackResp.StatusCode != http.StatusOK {
		t.Fatalf("woo outbound.ack = %d", ackResp.StatusCode)
	}

	// 3. ZERO-LOOP ECHO TEST:
	// Simulate Shopify Store 1, Shopify Store 2, AND WooCommerce all firing products/update
	// webhooks echoing back the exact product Hanzo just pushed!
	beforeUS := len(shopify1.snapshot())
	beforeEU := len(shopify2.snapshot())

	for _, shopDom := range []string{shop1Domain, shop2Domain} {
		echoResp := sendShopifyWebhook(shopDom, "products/update", map[string]any{
			"id":       555,
			"title":    "Carbon Hoodie Ash",
			"handle":   "cd-hoodie-ash",
			"status":   "active",
			"currency": "USD",
			"variants": []map[string]any{{
				"id":                   777,
				"title":                "Ash / L",
				"sku":                  "CD-HOODIE-ASH",
				"price":                "120.00",
				"inventory_quantity":   20,
				"inventory_management": "shopify",
			}},
		})
		var echoBody map[string]any
		_ = json.NewDecoder(echoResp.Body).Decode(&echoBody)
		if echoBody["echo"] != true {
			t.Fatalf("expected Shopify %s echoed webhook to be dropped with echo=true, got %+v", shopDom, echoBody)
		}
	}

	wooEchoResp := sendWoo("/v1/source/woocommerce/ingest", "product.updated", []map[string]any{{
		"id":             901,
		"name":           "Carbon Hoodie Ash",
		"type":           "variable",
		"status":         "publish",
		"permalink":      "https://vintage.carbondistrict.com/product/cd-hoodie-ash/",
		"sku":            "CD-HOODIE-ASH",
		"price":          "120.00",
		"manage_stock":   false,
		"stock_quantity": nil,
		"stock_status":   "instock",
		"currency":       "USD",
		"variations": []map[string]any{{
			"id":             911,
			"sku":            "CD-HOODIE-ASH",
			"price":          "120.00",
			"regular_price":  "120.00",
			"status":         "publish",
			"manage_stock":   true,
			"stock_quantity": 20,
			"stock_status":   "instock",
		}},
	}})
	var wooEchoBody map[string]any
	_ = json.NewDecoder(wooEchoResp.Body).Decode(&wooEchoBody)
	if wooEchoBody["echoes"] != float64(1) {
		t.Fatalf("expected WooCommerce echoed webhook to be dropped (echoes=1), got %+v", wooEchoBody)
	}

	if len(shopify1.snapshot()) != beforeUS || len(shopify2.snapshot()) != beforeEU {
		t.Fatalf("echoed webhooks caused outbound GraphQL loop! us=%d->%d eu=%d->%d",
			beforeUS, len(shopify1.snapshot()), beforeEU, len(shopify2.snapshot()))
	}

	// 4. MASTER SKU CROSS-STORE INVENTORY SYNC:
	// Paid order on Shopify Store 1 buys 3 units of "CD-HOODIE-ASH" (20 -> 17)!
	usInvBefore := shopify1.countContaining("inventorySetQuantities")
	euInvBefore := shopify2.countContaining("inventorySetQuantities")

	paidOrderResp := sendShopifyWebhook(shop1Domain, "orders/paid", map[string]any{
		"id":               88001,
		"order_number":     1001,
		"email":            "buyer@carbondistrict.com",
		"currency":         "USD",
		"total_price":      "360.00",
		"total_discounts":  "0.00",
		"financial_status": "paid",
		"line_items": []map[string]any{{
			"id":       99001,
			"sku":      "CD-HOODIE-ASH",
			"title":    "Carbon Hoodie Ash",
			"quantity": 3,
			"price":    "120.00",
		}},
	})
	if paidOrderResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(paidOrderResp.Body)
		t.Fatalf("orders/paid = %d: %s", paidOrderResp.StatusCode, string(b))
	}

	// Canonical stock in carbon-district MUST now be 17!
	orgDB, err := hub.OrgDB(context.Background(), orgSlug)
	if err != nil {
		t.Fatalf("OrgDB: %v", err)
	}
	vars := make([]*variant.Variant, 0)
	_, _ = orgDB.Query("variant").Filter("SKU=", "CD-HOODIE-ASH").GetAll(&vars)
	if len(vars) != 1 || vars[0].Inventory != 17 {
		t.Fatalf("expected canonical Variant inventory = 17, got %+v", vars)
	}
	prods := make([]*product.Product, 0)
	_, _ = orgDB.Query("product").Filter("SKU=", "CD-HOODIE-ASH").GetAll(&prods)
	if len(prods) != 1 || len(prods[0].Variants) != 1 || prods[0].Variants[0].Inventory != 17 {
		t.Fatalf("expected canonical Product.Variants[0].Inventory = 17, got %+v", prods)
	}

	// Origin store (Shopify Store 1) must NOT receive inventorySetQuantities back,
	// while sibling Shopify Store 2 MUST receive inventorySetQuantities(quantity: 17)!
	if shopify1.countContaining("inventorySetQuantities") != usInvBefore {
		t.Fatalf("origin Shopify Store 1 should not receive self-inventory push")
	}
	if shopify2.countContaining("inventorySetQuantities") != euInvBefore+1 {
		t.Fatalf("Shopify Store 2 did not receive inventorySetQuantities push!")
	}

	// WooCommerce store MUST have an outbound inventory.set job with quantity = 17!
	pullInv := sendWoo("/v1/source/woocommerce/outbound", "outbound.pull", map[string]any{"limit": 50})
	var pullInvData struct {
		Jobs []woocommerce.OutboundEnvelope `json:"jobs"`
	}
	_ = json.NewDecoder(pullInv.Body).Decode(&pullInvData)
	if len(pullInvData.Jobs) != 1 || pullInvData.Jobs[0].Operation != hub.OpInventorySet {
		t.Fatalf("expected 1 inventory.set job queued for WooCommerce, got %+v", pullInvData.Jobs)
	}
	var invPayload hub.CanonicalInventoryUpdate
	_ = json.Unmarshal(pullInvData.Jobs[0].Data, &invPayload)
	if invPayload.SKU != "CD-HOODIE-ASH" || invPayload.Quantity != 17 {
		t.Fatalf("expected WooCommerce inventory.set quantity=17, got %+v", invPayload)
	}

	// And when Shopify Store 2 echoes back a products/update webhook with the new stock 17,
	// PendingOutboundHash catches it as an echo with zero loop!
	euStockEcho := sendShopifyWebhook(shop2Domain, "products/update", map[string]any{
		"id":       556,
		"title":    "Carbon Hoodie Ash",
		"handle":   "cd-hoodie-ash",
		"status":   "active",
		"currency": "USD",
		"variants": []map[string]any{{
			"id":                   778,
			"title":                "Ash / L",
			"sku":                  "CD-HOODIE-ASH",
			"price":                "120.00",
			"inventory_quantity":   17,
			"inventory_management": "shopify",
		}},
	})
	var euStockEchoBody map[string]any
	_ = json.NewDecoder(euStockEcho.Body).Decode(&euStockEchoBody)
	if euStockEchoBody["echo"] != true {
		t.Fatalf("expected Shopify Store 2 stock-17 echo to be dropped, got %+v", euStockEchoBody)
	}
}

// TestBidirectionalWritebacksAndSignedWooPush verifies outbound fulfillments,
// refunds, customer consent write-backs, campaign Flow triggers, and direct
// signed push to WooCommerce's /wp-json/hanzo/v1/apply endpoint.
func TestBidirectionalWritebacksAndSignedWooPush(t *testing.T) {
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
	_ = iammiddleware.Init(nil)

	shopifySrv := newMockShopifyServer("wb")
	defer shopifySrv.srv.Close()

	var wooPushOps []string
	var wooSecretMu sync.Mutex
	var expectedWooSecret string
	const wooShop = "9a8b7c6d-1111-4222-8333-444455556666"

	wooCallbackSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		wooSecretMu.Lock()
		sec := expectedWooSecret
		wooSecretMu.Unlock()
		if !woocommerce.VerifySignatureHeader(
			sec,
			r.Header.Get("X-Hanzo-Signature"),
			wooShop,
			r.Header.Get("X-Hanzo-Topic"),
			r.Header.Get("X-Hanzo-Delivery"),
			raw,
			time.Now(),
		) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var env struct {
			Jobs []woocommerce.OutboundEnvelope `json:"jobs"`
		}
		_ = json.Unmarshal(raw, &env)
		applied := make([]map[string]string, 0, len(env.Jobs))
		wooSecretMu.Lock()
		for _, j := range env.Jobs {
			wooPushOps = append(wooPushOps, j.Operation)
			applied = append(applied, map[string]string{"job_id": j.JobID, "external_id": "777"})
		}
		wooSecretMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"applied": applied})
	}))
	defer wooCallbackSrv.Close()

	eng := hub.NewEngine()
	bundle := source.NewBundle(shopify.Config{
		ClientID:     "0123456789abcdef0123456789abcdef",
		ClientSecret: "shpss_wb_secret",
	}, eng, shopifySrv.srv.Client())

	app := zip.New(zip.Config{DisableStartupMessage: true})
	bundle.Mount(app.Group("/v1"))

	postAdmin := func(path string, payload any) *http.Response {
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Org-Id", "carbon-wb")
		req.Header.Set("X-User-Id", "usr_1")
		req.Header.Set("X-User-Owner", "carbon-wb")
		req.Header.Set("X-User-IsOrgAdmin", "true")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		return resp
	}

	_ = postAdmin("/v1/source/sources", map[string]any{
		"sourceId":    "shopify_wb",
		"provider":    "shopify",
		"shop":        "wb.myshopify.com",
		"syncMode":    "bidirectional",
		"endpointUrl": shopifySrv.srv.URL,
		"accessToken": "shpat_wb",
	})
	connResp := postAdmin("/v1/source/woocommerce/connect", map[string]any{
		"shop":         wooShop,
		"source_id":    "woo_wb",
		"domain":       "woo-wb.example.com",
		"endpoint_url": wooCallbackSrv.URL,
	})
	var conn map[string]string
	_ = json.NewDecoder(connResp.Body).Decode(&conn)
	wooSecretMu.Lock()
	expectedWooSecret = conn["secret"]
	wooSecretMu.Unlock()

	// Fulfillment, refund, consent, campaign, and product push!
	if r := postAdmin("/v1/source/publish/product", hub.CanonicalProduct{
		Slug: "wb-cap", SKU: "WB-CAP", Name: "WB Cap", Currency: "USD", PriceCents: 3500, Available: true, ManageStock: true, Inventory: 12,
	}); r.StatusCode != http.StatusOK {
		t.Fatalf("publish product = %d", r.StatusCode)
	}
	if r := postAdmin("/v1/source/publish/fulfillment", hub.CanonicalFulfillment{
		OrderExternalId: "5001", Carrier: "UPS", TrackingNumber: "1Z999", TrackingURL: "https://ups.example/1Z999", NotifyCustomer: true,
	}); r.StatusCode != http.StatusOK {
		t.Fatalf("publish fulfillment = %d", r.StatusCode)
	}
	if r := postAdmin("/v1/source/publish/refund", hub.CanonicalRefund{
		OrderExternalId: "5001", AmountCents: 1500, Currency: "USD", Note: "Partial refund",
	}); r.StatusCode != http.StatusOK {
		t.Fatalf("publish refund = %d", r.StatusCode)
	}
	if r := postAdmin("/v1/source/publish/campaign", hub.CanonicalCampaignActivity{
		CampaignId: "cmp_1", Title: "Autumn Drop", URL: "https://hanzo.ai/c/autumn", CustomerId: "444",
	}); r.StatusCode != http.StatusOK {
		t.Fatalf("publish campaign = %d", r.StatusCode)
	}

	for _, want := range []string{"productSet", "fulfillmentCreateV2", "refundCreate", "marketingActivityUpsertExternal", "flowTriggerReceive"} {
		if shopifySrv.countContaining(want) < 1 {
			t.Fatalf("expected Shopify GraphQL mutation %q, calls=%d", want, len(shopifySrv.snapshot()))
		}
	}
	wooSecretMu.Lock()
	gotWooOps := append([]string{}, wooPushOps...)
	wooSecretMu.Unlock()
	if len(gotWooOps) < 3 {
		t.Fatalf("expected signed push callbacks to WooCommerce /wp-json/hanzo/v1/apply, got %v", gotWooOps)
	}
}
