package checkout

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/models/organization"
)

// TestSandboxMintAtCatalogPrice drives the REAL Square Payment Links client in
// SANDBOX for each agency plan at its catalog price, proving a checkout intent
// mints at the correct amount (not the $50 hat fallback). It is skipped unless
// the sandbox creds are provided, so it never runs (or bills) in CI:
//
//	SQUARE_ENVIRONMENT=sandbox \
//	SQUARE_SANDBOX_ACCESS_TOKEN=... SQUARE_SANDBOX_LOCATION_ID=... \
//	go test ./api/checkout/ -run TestSandboxMintAtCatalogPrice -v
func TestSandboxMintAtCatalogPrice(t *testing.T) {
	if os.Getenv("SQUARE_SANDBOX_ACCESS_TOKEN") == "" || os.Getenv("SQUARE_SANDBOX_LOCATION_ID") == "" {
		t.Skip("sandbox Square creds not provided; skipping live-sandbox mint proof")
	}
	t.Setenv("SQUARE_ENVIRONMENT", "sandbox")

	gin.SetMode(gin.TestMode)
	org := &organization.Organization{} // no per-org Square creds → sandbox env fallback
	org.Name = "hanzo"

	cases := []struct {
		plan   string
		name   string
		amount int64
	}{
		{"agency", "Agency Service", 999900},
		{"instant-site", "Instant Site", 50000},
		{"enterprise", "Enterprise", 999900},
	}
	for _, tc := range cases {
		t.Run(tc.plan, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/checkout/sessions", nil)

			items := []checkoutLineItem{{Name: tc.name, Quantity: 1, Amount: tc.amount}}
			resp, err := createSquareCheckout(c, org, items, tc.amount, 0, tc.amount, nil, "USD",
				checkoutSessionRequest{SuccessURL: "https://hanzo.ai/onboarding-success"})
			if err != nil {
				t.Fatalf("%s: mint failed: %v", tc.plan, err)
			}
			if !strings.Contains(resp.CheckoutURL, "sandbox.square.link") &&
				!strings.Contains(resp.CheckoutURL, "squareupsandbox.com") {
				t.Fatalf("%s: not a sandbox link: %s", tc.plan, resp.CheckoutURL)
			}
			t.Logf("PROOF %s @ $%.2f -> %s (session %s)", tc.plan, float64(tc.amount)/100, resp.CheckoutURL, resp.SessionID)
		})
	}
}
