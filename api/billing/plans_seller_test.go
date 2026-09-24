package billing

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/util/test/ae"
)

// TestReadPlansSellsOnlyOnTheSellersHosts: GET /v1/billing/plans answers the
// catalog of the brand the host resolves to. Every brand's pay host reads it,
// and each listed Hanzo's ladder under its own name — pay.lux.cloud and
// pay.zoo.cloud offered Pro and Max as if Lux and Zoo sold them.
func TestReadPlansSellsOnlyOnTheSellersHosts(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	if _, _, err := SeedPlans(c); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Hanzo's hosts, and a caller that names no host (the deployment default).
	for _, host := range []string{"pay.hanzo.ai", "api.hanzo.ai", "PAY.HANZO.AI:443", ""} {
		rows, err := ReadPlans(c, host, "", nil)
		if err != nil {
			t.Fatalf("%q: %v", host, err)
		}
		if len(rows) != len(catalog) {
			t.Errorf("%q lists %d plans, the catalog has %d", host, len(rows), len(catalog))
		}
	}

	// Every other brand's pay host sells none — an empty list, never Hanzo's, and
	// never nil, which a JSON reader would take for a failure.
	for _, host := range []string{"pay.lux.cloud", "pay.lux.tel", "pay.zoo.cloud", "Pay.Zoo.Cloud:443", "pay.pars.network"} {
		rows, err := ReadPlans(c, host, "", nil)
		if err != nil {
			t.Fatalf("%q: %v", host, err)
		}
		if rows == nil || len(rows) != 0 {
			t.Errorf("%q lists %v, want an empty list", host, rows)
		}
		if dns, _ := ReadPlans(c, host, "dns", nil); len(dns) != 0 {
			t.Errorf("%q lists %d dns plans, want none", host, len(dns))
		}
	}
}

// TestSellsFollowsTheOrgEndpointsTable: the catalog's seller is decided by the
// same host→brand table the org endpoint wears, so a spoof suffix is not Hanzo.
func TestSellsFollowsTheOrgEndpointsTable(t *testing.T) {
	for host, want := range map[string]bool{
		"pay.hanzo.ai":           true,
		"hanzo.ai":               true,
		"pay.lux.cloud":          false,
		"lux.id":                 false,
		"zoo.ngo":                false,
		"pay.hanzo.ai.lux.tel":   false,
		"pay.lux.cloud.hanzo.ai": true,
	} {
		if got := Sells(host); got != want {
			t.Errorf("Sells(%q) = %v, want %v", host, got, want)
		}
	}
}

// The purchase doors ask the same question as the list. A plan bought on a
// brand's host that sells no catalog is a plan that is not there — refused with
// the same 404 as a slug that never existed, before anything is charged.
func TestPurchaseDoorsSellOnlyOnTheSellersHosts(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := moneyOrg("acme")

	at := func(host, method, path, pattern, body string, h zip.Handler) int {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Host = host
		req.Header.Set("Content-Type", "application/json")
		return driveSeeded(func(c *zip.Ctx) {
			c.Locals("organization", org)
			c.SetContext(ctx)
			c1MintPrincipal(c)
		}, pattern, req, h).StatusCode
	}

	for _, host := range []string{"pay.lux.cloud", "pay.lux.tel", "pay.zoo.cloud", "pay.pars.network"} {
		if got := at(host, http.MethodPost, "/v1/billing/subscribe/card", "/v1/billing/subscribe/card",
			`{"sourceId":"cnon:card-nonce-ok","planId":"pro"}`, SubscribeWithCard); got != http.StatusNotFound {
			t.Errorf("subscribe/card on %s = %d, want 404", host, got)
		}
		if got := at(host, http.MethodPost, "/v1/billing/subscriptions", "/v1/billing/subscriptions",
			`{"userId":"acme","planId":"free"}`, CreateBillingSubscription); got != http.StatusNotFound {
			t.Errorf("create subscription on %s = %d, want 404", host, got)
		}
	}
	// The seller's own host is not refused by this gate.
	if got := at("pay.hanzo.ai", http.MethodPost, "/v1/billing/subscriptions", "/v1/billing/subscriptions",
		`{"userId":"acme","planId":"free"}`, CreateBillingSubscription); got == http.StatusNotFound {
		t.Errorf("create subscription on pay.hanzo.ai = 404, the seller's own host")
	}
}
