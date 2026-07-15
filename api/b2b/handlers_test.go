// Copyright © 2026 Hanzo AI. MIT License.

package b2b

import (
	"net/http"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
)

// TestB2B_MoneyRoutes_NonAdmin_403 proves the HIGH-4 fix for the B2B custom
// subroutes. util/rest.Route wired the auth middleware only onto base CRUD, NOT
// custom subroutes, and middleware.TokenRequired(Admin) no-ops on the IAM path —
// so accept/reject quote and approve/reject approval (money-moving B2B actions)
// were reachable by any authenticated non-admin. Each now enforces admin
// first-thing via middleware.RequireAdmin; an authenticated non-admin gets 403
// before any state change.
func TestB2B_MoneyRoutes_NonAdmin_403(t *testing.T) {
	cases := []struct {
		name    string
		handler func(*zip.Ctx) error
	}{
		{"acceptQuote", AcceptQuote},
		{"rejectQuote", RejectQuote},
		{"approveApproval", ApproveApproval},
		{"rejectApproval", RejectApproval},
	}
	app := zip.New(zip.Config{DisableStartupMessage: true})
	for _, tcase := range cases {
		t.Run(tcase.name, func(t *testing.T) {
			c := app.TestCtx(http.MethodPost, "/v1/commerce/b2b/"+tcase.name)
			// Authenticated via IAM, but NOT an admin — the exact caller HIGH-4
			// let through.
			c.Locals("iam_authenticated", true)
			c.Locals("iam_claims", &auth.IAMClaims{Owner: "acme", IsAdmin: false})
			_ = tcase.handler(c)
			if st := c.Fiber().Response().StatusCode(); st != http.StatusForbidden {
				t.Fatalf("%s as non-admin = %d, want 403 (money subroute must gate admin)", tcase.name, st)
			}
		})
	}
}
