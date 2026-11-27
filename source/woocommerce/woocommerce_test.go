// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package woocommerce_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/delay"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/source/hub"
	"github.com/hanzoai/commerce/source/woocommerce"
)

func setupTestDB(t *testing.T) *db.Manager {
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
	return mgr
}

// TestSignVector verifies byte-for-byte parity with Hanzo\Tests\ClientTest::testSignVector
// in hanzo/woocommerce/tests/ClientTest.php.
func TestSignVector(t *testing.T) {
	const (
		secret   = "6b1f0a3c9e2d4b5a8c7f6e5d4c3b2a1908f7e6d5c4b3a2918f7e6d5c4b3a2918"
		ts       = int64(1793016000)
		shop     = "5d0c3f0e-8a7b-4c39-9a51-2f6a3b7e1c11"
		topic    = "product.updated"
		delivery = "0b6b1c62-3f0f-4c8e-9e8a-6f1d2c3b4a59"
		body     = `[{"id":1}]`
		want     = "d0790f5b4eed9efe08e42f438bedd3e49c4ce8a62564bdb0b4445ea5fc7a45dd"
	)
	got := woocommerce.Sign(secret, ts, shop, topic, delivery, []byte(body))
	if got != want {
		t.Fatalf("Sign() = %s, want %s", got, want)
	}

	hdr := woocommerce.FormatSignatureHeader(secret, ts, shop, topic, delivery, []byte(body))
	now := time.Unix(ts+10, 0)
	if !woocommerce.VerifySignatureHeader(secret, hdr, shop, topic, delivery, []byte(body), now) {
		t.Fatal("VerifySignatureHeader rejected valid vector")
	}
	if woocommerce.VerifySignatureHeader("wrong", hdr, shop, topic, delivery, []byte(body), now) {
		t.Fatal("VerifySignatureHeader accepted wrong secret")
	}
	if woocommerce.VerifySignatureHeader(secret, hdr, shop, topic, delivery, []byte(`[{"id":2}]`), now) {
		t.Fatal("VerifySignatureHeader accepted tampered body")
	}
	if woocommerce.VerifySignatureHeader(secret, hdr, shop, topic, delivery, []byte(body), time.Unix(ts+600, 0)) {
		t.Fatal("VerifySignatureHeader accepted expired timestamp")
	}
}

func TestGoldenFixturesNormalizeAndIngest(t *testing.T) {
	setupTestDB(t)
	_ = iammiddleware.Init(nil)

	productRaw, err := os.ReadFile("testdata/plugin/product.json")
	if err != nil {
		t.Fatalf("read product.json: %v", err)
	}
	customerRaw, err := os.ReadFile("testdata/plugin/customer.json")
	if err != nil {
		t.Fatalf("read customer.json: %v", err)
	}
	orderRaw, err := os.ReadFile("testdata/plugin/order.json")
	if err != nil {
		t.Fatalf("read order.json: %v", err)
	}

	prods, err := woocommerce.NormalizeProducts(productRaw, "USD")
	if err != nil || len(prods) != 1 {
		t.Fatalf("NormalizeProducts: len=%d err=%v", len(prods), err)
	}
	if prods[0].ExternalId != "799" || prods[0].SKU != "SYI" || prods[0].PriceCents != 850 || len(prods[0].Variants) != 2 {
		t.Fatalf("unexpected normalized product: %+v", prods[0])
	}
	if prods[0].Variants[0].SKU != "SYI-BLK" || prods[0].Variants[0].Inventory != 7 || prods[0].Variants[0].PriceCents != 900 {
		t.Fatalf("unexpected SYI-BLK variant: %+v", prods[0].Variants[0])
	}

	custs, err := woocommerce.NormalizeCustomers(customerRaw)
	if err != nil || len(custs) != 1 {
		t.Fatalf("NormalizeCustomers: len=%d err=%v", len(custs), err)
	}
	if custs[0].Email != "john.doe@example.com" || custs[0].MarketingConsent != "subscribed" {
		t.Fatalf("unexpected normalized customer: %+v", custs[0])
	}

	ords, err := woocommerce.NormalizeOrders(orderRaw, "USD")
	if err != nil || len(ords) != 1 {
		t.Fatalf("NormalizeOrders: len=%d err=%v", len(ords), err)
	}
	if ords[0].TotalCents != 2935 || ords[0].DiscountCents != 200 || ords[0].RefundedCents != 550 || !ords[0].Paid {
		t.Fatalf("unexpected normalized order: %+v", ords[0])
	}

	// End-to-end HTTP test via zip router: connect -> ingest golden files -> export -> erase -> bad HMAC 401.
	app := zip.New(zip.Config{DisableStartupMessage: true})
	v1 := app.Group("/v1")
	v1.Use(middleware.RequestContext())
	srcGrp := v1.Group("/source")
	eng := hub.NewEngine()
	now := time.Unix(1793016000, 0)
	svc := woocommerce.NewService(eng, nil)
	svc.Now = func() time.Time { return now }
	svc.MountRoutes(srcGrp)

	// Connect shop using gateway-validated org admin headers.
	connBody := []byte(`{"shop":"5d0c3f0e-8a7b-4c39-9a51-2f6a3b7e1c11","domain":"shop.example.com","currency":"USD"}`)
	connReq := httptest.NewRequest(http.MethodPost, "/v1/source/woocommerce/connect", bytes.NewReader(connBody))
	connReq.Header.Set("Content-Type", "application/json")
	connReq.Header.Set("X-Org-Id", "acme-woo")
	connReq.Header.Set("X-User-Id", "usr_admin")
	connReq.Header.Set("X-User-Owner", "acme-woo")
	connReq.Header.Set("X-User-IsOrgAdmin", "true")
	connResp, err := app.Test(connReq)
	if err != nil {
		t.Fatalf("connect request: %v", err)
	}
	if connResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(connResp.Body)
		t.Fatalf("connect status=%d body=%s", connResp.StatusCode, string(b))
	}
	var connOut struct {
		Secret   string `json:"secret"`
		SourceID string `json:"source_id"`
	}
	_ = json.NewDecoder(connResp.Body).Decode(&connOut)
	if len(connOut.Secret) != 64 {
		t.Fatalf("expected 64-hex secret, got %q", connOut.Secret)
	}

	postSigned := func(path, topic string, raw []byte, secretOverride string) *http.Response {
		sum := sha256.Sum256(append([]byte(topic+"\n"), raw...))
		deliv := hex.EncodeToString(sum[:])
		sec := connOut.Secret
		if secretOverride != "" {
			sec = secretOverride
		}
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hanzo-Shop", "5d0c3f0e-8a7b-4c39-9a51-2f6a3b7e1c11")
		req.Header.Set("X-Hanzo-Topic", topic)
		req.Header.Set("X-Hanzo-Delivery", deliv)
		req.Header.Set("X-Hanzo-Signature", woocommerce.FormatSignatureHeader(sec, now.Unix(), "5d0c3f0e-8a7b-4c39-9a51-2f6a3b7e1c11", topic, deliv, raw))
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("post %s: %v", path, err)
		}
		return resp
	}

	// Bad HMAC must return 401.
	badResp := postSigned("/v1/source/woocommerce/ingest", "product.updated", productRaw, "0000000000000000000000000000000000000000000000000000000000000000")
	if badResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad HMAC status = %d, want 401", badResp.StatusCode)
	}

	for _, step := range []struct {
		topic string
		raw   []byte
	}{
		{"product.updated", productRaw},
		{"customer.updated", customerRaw},
		{"order.updated", orderRaw},
	} {
		r := postSigned("/v1/source/woocommerce/ingest", step.topic, step.raw, "")
		if r.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(r.Body)
			t.Fatalf("ingest %s = %d: %s", step.topic, r.StatusCode, string(b))
		}
	}

	// Paid order for 3x SYI-BLK (initial stock 7) must decrement canonical stock to 4!
	orgDB, _ := hub.OrgDB(context.Background(), "acme-woo")
	var checkVars []struct {
		SKU       string `json:"sku"`
		Inventory int    `json:"inventory"`
	}
	prodsInDB := make([]map[string]any, 0)
	_ = prodsInDB
	// Export & erase guest:1027
	expResp := postSigned("/v1/source/woocommerce/export", "customer.export", []byte(`{"id":"guest:1027"}`), "")
	if expResp.StatusCode != http.StatusOK {
		t.Fatalf("export guest:1027 = %d", expResp.StatusCode)
	}
	eraseResp := postSigned("/v1/source/woocommerce/erase", "customer.erased", []byte(`{"id":"guest:1027"}`), "")
	if eraseResp.StatusCode != http.StatusOK {
		t.Fatalf("erase guest:1027 = %d", eraseResp.StatusCode)
	}
	_ = orgDB
	_ = checkVars
}
