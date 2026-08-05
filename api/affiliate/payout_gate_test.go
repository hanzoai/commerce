package affiliate

// payout_gate_test.go proves the [MEDIUM] C1 miss is closed: the OSS-contributor
// treasury payout machinery (executePayouts and friends) is service-token /
// platform-global-admin ONLY — an org-level admin can no longer drive treasury
// disbursement (CreditGrants / Stripe transfers / on-chain HUSD).

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// mountAffiliate mounts the real affiliate Route() behind a seed and returns the app
// plus the concrete POST path whose registration ends with the given suffix. Fiber
// exposes no handler names (gin's ri.Handler is gone), so routes are matched by their
// path tail instead.
func mountAffiliate(t *testing.T, seed func(*zip.Ctx), pathSuffix string) (*zip.App, string) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	if seed != nil {
		app.Use(zip.H(func(c *zip.Ctx) error { seed(c); return c.Next() }))
	}
	Route(app.Group("/v1"))

	for _, ri := range app.Fiber().GetRoutes() {
		if ri.Method == http.MethodPost && strings.HasSuffix(ri.Path, pathSuffix) {
			return app, ri.Path
		}
	}
	t.Fatalf("route with path suffix %q not found", pathSuffix)
	return nil, ""
}

// TestPayoutExecute_OrgAdminDenied is the acceptance test: an org admin POSTing
// the payout executor is 403 — it can no longer disburse the treasury.
func TestPayoutExecute_OrgAdminDenied(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")
	app, path := mountAffiliate(t, func(c *zip.Ctx) {
		c.Locals("iam_authenticated", true)
		c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
		c.Locals("iam_claims", &auth.IAMClaims{Owner: "acme", IsAdmin: true})
	}, "/payouts/execute")

	req := httptest.NewRequest(http.MethodPost, path+"?execute=true", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("org-admin %s: status=%d body=%s, want 403", path, resp.StatusCode, body)
	}
}

// TestPayoutCalculate_OrgAdminDenied proves the sibling payout-family write is
// gated too (RED grouped calculate/sbom with execute).
func TestPayoutCalculate_OrgAdminDenied(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "")
	app, path := mountAffiliate(t, func(c *zip.Ctx) {
		c.Locals("iam_authenticated", true)
		c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
		c.Locals("iam_claims", &auth.IAMClaims{Owner: "acme", IsAdmin: true})
	}, "/payouts/calculate")

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{"totalRevenueCents":100000}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("org-admin %s: status=%d body=%s, want 403", path, resp.StatusCode, body)
	}
}

// TestPayoutExecute_ServiceTokenReaches proves the legitimate CronJob / cloud-api
// path is preserved: the internal service token passes the gate (NOT 403). The
// dry-run executor runs behind it.
func TestPayoutExecute_ServiceTokenReaches(t *testing.T) {
	const tok = "svc-payout"
	t.Setenv("COMMERCE_SERVICE_TOKEN", tok)
	ctx := ae.NewContext()
	defer ctx.Close()

	app, path := mountAffiliate(t, func(c *zip.Ctx) { c.SetContext(ctx) }, "/payouts/execute")

	// dry-run (no ?execute=true) — the gate must ADMIT the service token.
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Org-Id", "payoutorg")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	if resp.StatusCode == http.StatusForbidden {
		t.Fatalf("service-token %s was 403 — the gate wrongly denied the internal service path", path)
	}
}
