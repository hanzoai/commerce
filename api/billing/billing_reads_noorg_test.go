package billing

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zap-proto/zip"
)

// TestBillingReads_NoOrgInContext_NoPanic closes the #146 sibling 502 class: the co-resident
// billing READ handlers registered alongside ListSpendAlerts (cloud apps/commerce.go billingRead)
// all had the SAME unguarded `middleware.GetOrganization(c)` first line. On the co-resident cloud
// embed path IAMTokenRequired no-ops (no gateway-injected X-Org-Id), so a validated caller can
// reach them with no "organization" local — the unchecked type assertion panicked on a nil
// interface, and (cloud installs no Recover) fasthttp reset the connection → the edge returned 502.
//
// Each handler must now resolve the org SAFELY (GetOrganizationOK) and return an honest status —
// the same empty shape it returns for an org with zero rows, or a clean 401 — never a panic. The
// seed sets NO "organization" local, exactly the embed path.
func TestBillingReads_NoOrgInContext_NoPanic(t *testing.T) {
	noOrg := func(c *zip.Ctx) {} // embed path: nothing set the "organization" local.

	// read is a co-resident billing GET that must be honest-empty (200) with no org.
	read := func(name, path string, h zip.Handler, wantSubstr string) {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := driveSeeded(noOrg, path, req, h)
			if w.StatusCode != 200 {
				t.Fatalf("%s with no org: status = %d, want 200 (honest empty, never a panic→502)", name, w.StatusCode)
			}
			b, _ := io.ReadAll(w.Body)
			if !strings.Contains(string(b), wantSubstr) {
				t.Fatalf("%s with no org: body = %q, want it to contain %q", name, string(b), wantSubstr)
			}
		})
	}

	read("invoices", "/v1/billing/invoices", ListInvoices, `"count":0`)
	read("subscriptions", "/v1/billing/subscriptions", ListBillingSubscriptions, `"count":0`)
	read("payouts", "/v1/billing/payouts", ListPayouts, `[]`)
	read("payment-config", "/v1/billing/payment-config", GetPaymentConfig, `"provider":"square"`)

	// The per-invoice PDF download is a binary GET in the SAME block; with no validated org it
	// must be a clean 401 (no namespace to resolve the invoice id in), never a panic.
	t.Run("invoice-pdf", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/billing/invoices/abc/pdf", nil)
		w := driveSeeded(noOrg, "/v1/billing/invoices/:id/pdf", req, DownloadInvoicePDF)
		if w.StatusCode != 401 {
			t.Fatalf("DownloadInvoicePDF with no org: status = %d, want 401", w.StatusCode)
		}
	})
}
