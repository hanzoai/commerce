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
	read("payment-config", "/v1/billing/settings", GetPaymentConfig, `"provider":"square"`)

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

// TestSubscribeWithCard_NoOrgInContext_NoPanic is the same class on the WRITE side, and it
// is the one that cost money. SubscribeWithCard — the paid front door — opened with the
// unguarded `middleware.GetOrganization(c)` while every read beside it had been fixed.
//
// IAMTokenRequired deliberately FALLS THROUGH without setting the local when the gateway
// named no principal (`ownerID == "" || userID == ""`), so legacy auth still gets its turn.
// Measured in production 2026-08-06: the panic recovered as a bare 500 with no body, a $99
// "Subscription Max — first period" settled at Square with NO subscription row behind it,
// and `zero subscriptions exist platform-wide` was the visible symptom. That in turn
// starves the serving plane, because ai's funding gate requires a confirmed paying
// subscriber for any prepaid SKU and fails closed.
//
// A door that takes a card must refuse cleanly when it cannot name the payer. Money never
// bills a guess, and a missing org is the emptiest guess there is.
func TestMoneyWrites_NoOrgInContext_NoPanic(t *testing.T) {
	noOrg := func(c *zip.Ctx) {} // the fall-through path: nothing set the "organization" local.

	// Every co-resident money WRITE, each of which opened by dereferencing the org.
	// SubscribeWithCard is the one that cost money — a $99 first period settled at
	// Square and the panic ate the subscription — but they all sit on the same chain,
	// and IAMTokenRequired falls through without setting the local whenever the
	// gateway named no principal (`ownerID == "" || userID == ""`), so legacy auth
	// still gets its turn.
	//
	// A door that moves money must refuse cleanly when it cannot name the payer.
	// Money never bills a guess, and a missing org is the emptiest guess there is.
	write := func(name, method, path, body string, h zip.Handler) {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(method, path, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := driveSeeded(noOrg, path, req, h)
			if w.StatusCode != 401 {
				b, _ := io.ReadAll(w.Body)
				t.Fatalf("%s with no org: status = %d, body = %q; want 401 — "+
					"a panic here 500s with no body, and on a charging door it does so "+
					"AFTER the card has moved", name, w.StatusCode, string(b))
			}
		})
	}

	write("subscribe-card", http.MethodPost, "/v1/billing/subscribe/card",
		`{"planId":"max","sourceId":"cnon:card-nonce-ok"}`, SubscribeWithCard)
	write("topup", http.MethodPost, "/v1/billing/topup",
		`{"amountCents":500}`, Topup)
	write("topup-token", http.MethodPost, "/v1/billing/topup/token",
		`{"amountCents":500,"sourceId":"cnon:card-nonce-ok"}`, TopupWithToken)
	write("payment-method-create", http.MethodPost, "/v1/billing/payment-methods",
		`{"sourceId":"cnon:card-nonce-ok"}`, CreatePaymentMethod)
	write("payment-method-detach", http.MethodDelete, "/v1/billing/payment-methods/pm_1",
		``, DetachPaymentMethod)
	write("subscription-cancel", http.MethodPost, "/v1/billing/subscriptions/sub_1/cancel",
		``, CancelBillingSubscription)
	write("subscription-reactivate", http.MethodPost, "/v1/billing/subscriptions/sub_1/reactivate",
		``, ReactivateBillingSubscription)
}
