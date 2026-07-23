package billing

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zap-proto/zip"
)

// TestSpendAlerts_NoOrgInContext_NoPanic reproduces #146 (the console Budgets 502).
//
// On the co-resident cloud embed path a browser read of GET /v1/billing/spend-alerts
// runs the chain RequestContext → IAMTokenRequired → PinBillingSubject → ListSpendAlerts.
// IAMTokenRequired no-ops when there is no gateway-injected X-Org-Id (the embed path has
// no gateway in front of cloud), so it never sets the "organization" local — yet
// PinBillingSubject still admits the request (it validates the principal separately).
// ListSpendAlerts' first line was `middleware.GetOrganization(c)`, an UNCHECKED type
// assertion that PANICS on a nil interface when the org local is absent. cloud installs
// no global Recover, so fasthttp reset the connection and the edge returned 502.
//
// The seed here sets NO "organization" local (exactly the embed path), so before the fix
// every spend-alert handler panics. After the fix each resolves the org SAFELY and returns
// an honest status (never a 502): the list is an empty array, the writes are a clean 401.
func TestSpendAlerts_NoOrgInContext_NoPanic(t *testing.T) {
	noOrg := func(c *zip.Ctx) {} // the embed path: nothing set the "organization" local.

	t.Run("list is honest-empty, never a panic→502", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/billing/spend-alerts", nil)
		w := driveSeeded(noOrg, "/v1/billing/spend-alerts", req, ListSpendAlerts)
		if w.StatusCode != 200 {
			t.Fatalf("ListSpendAlerts with no org: status = %d, want 200 (honest empty)", w.StatusCode)
		}
		b, _ := io.ReadAll(w.Body)
		if got := strings.TrimSpace(string(b)); got != "[]" {
			t.Fatalf("ListSpendAlerts with no org: body = %q, want []", got)
		}
	})

	t.Run("create is a clean 401, never a panic→502", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/billing/spend-alerts",
			strings.NewReader(`{"threshold":100}`))
		req.Header.Set("Content-Type", "application/json")
		w := driveSeeded(noOrg, "/v1/billing/spend-alerts", req, CreateSpendAlert)
		if w.StatusCode != 401 {
			t.Fatalf("CreateSpendAlert with no org: status = %d, want 401", w.StatusCode)
		}
	})

	t.Run("update is a clean 401, never a panic→502", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/v1/billing/spend-alerts/abc",
			strings.NewReader(`{"title":"x"}`))
		req.Header.Set("Content-Type", "application/json")
		w := driveSeeded(noOrg, "/v1/billing/spend-alerts/:id", req, UpdateSpendAlert)
		if w.StatusCode != 401 {
			t.Fatalf("UpdateSpendAlert with no org: status = %d, want 401", w.StatusCode)
		}
	})

	t.Run("delete is a clean 401, never a panic→502", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/billing/spend-alerts/abc", nil)
		w := driveSeeded(noOrg, "/v1/billing/spend-alerts/:id", req, DeleteSpendAlert)
		if w.StatusCode != 401 {
			t.Fatalf("DeleteSpendAlert with no org: status = %d, want 401", w.StatusCode)
		}
	})
}
