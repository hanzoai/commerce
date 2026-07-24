// Copyright © 2026 Hanzo AI. MIT License.

package invite

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	invitemodel "github.com/hanzoai/commerce/billing/invite"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/util/test/ae"
	"github.com/hanzoai/commerce/util/test/zipclient"
)

// TestRedeem_AdmitsOrgAdmin proves the paywall invite-redeem surface admits a
// gateway-verified ORG admin claiming a code for its OWN org — the way an org
// acquires access during onboarding. It is gated by the bundle's no-mask
// tokenRequired (any authenticated principal) behind IAMTokenRequired, so the org
// owner reaches Redeem and the code binds to its namespace (X-Org-Id), not a 403.
func TestRedeem_AdmitsOrgAdmin(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	// Platform mints the code out of band (Mint is superadmin-only).
	if _, err := invitemodel.Mint(invitemodel.SystemDB(ctx), "WELCOME", ""); err != nil {
		t.Fatalf("mint invite code: %v", err)
	}

	cl := zipclient.New(ctx)
	cl.Use(iammiddleware.IAMTokenRequired())
	Route(cl.Router, middleware.TokenRequired())

	req := httptest.NewRequest(http.MethodPost, "/commerce/invite/redeem", strings.NewReader(`{"code":"WELCOME"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(iammiddleware.HeaderUserIsAdmin, "true")
	req.Header.Set("X-Org-Id", "acme")
	req.Header.Set("X-User-Id", "user-acme-owner")

	w := cl.Do(req)
	if w.Code != http.StatusCreated {
		t.Fatalf("org admin redeem invite: got %d, want 201\nbody: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"org":"acme"`) {
		t.Fatalf("invite not bound to caller org: %s", w.Body.String())
	}
}
