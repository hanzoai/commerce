// Copyright © 2026 Hanzo AI. MIT License.

package iammiddleware

import (
	"net/http"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
)

// TestOrgAdminGrant_CoversOnboardingCatalog pins the self-serve onboarding
// contract: an org owner's grant satisfies the create gate of every catalog
// primitive step-1 onboarding needs — store (rest "store"/create =
// Admin|WriteStore), product, collection, variant — via the SAME intersection
// (bit.Field.Has) rest.CheckPermissions uses.
func TestOrgAdminGrant_CoversOnboardingCatalog(t *testing.T) {
	need := []struct {
		name string
		mask bit.Mask
	}{
		{"WriteStore", permission.WriteStore},
		{"ReadStore", permission.ReadStore},
		{"Store(list)", permission.Store},
		{"WriteProduct", permission.WriteProduct},
		{"WriteCollection", permission.WriteCollection},
		{"WriteVariant", permission.WriteVariant},
	}
	for _, n := range need {
		if !orgAdminGrant.Has(n.mask) {
			t.Errorf("orgAdminGrant lacks %s — an org owner cannot manage its own catalog", n.name)
		}
	}
}

// TestOrgAdminGrant_ExcludesMoneyAuthority is the security invariant: the org
// grant must NEVER carry permission.Admin (the money/platform authority credit-
// mint + card-charge + cross-org billing gates key on). Widening it here would
// let an org owner mint balance or charge cards platform-wide.
func TestOrgAdminGrant_ExcludesMoneyAuthority(t *testing.T) {
	if orgAdminGrant.Has(permission.Admin) {
		t.Fatal("orgAdminGrant includes permission.Admin — org owners must not gain money/platform authority")
	}
	for _, m := range []struct {
		name string
		mask bit.Mask
	}{
		{"Secret", permission.Secret},
		{"Authorize", permission.Authorize},
		{"Capture", permission.Capture},
		{"Payment", permission.Payment},
	} {
		if orgAdminGrant.Has(m.mask) {
			t.Errorf("orgAdminGrant includes %s — org owners must not gain payment/secret authority", m.name)
		}
	}
}

// TestIsOrgAdmin pins the trusted-header contract: only X-User-IsAdmin=="true"
// (case-insensitive) marks an org admin; absent or any other value fails closed.
func TestIsOrgAdmin(t *testing.T) {
	cases := []struct {
		val  string
		want bool
	}{
		{"true", true},
		{"TRUE", true},
		{"", false},
		{"false", false},
		{"1", false},
	}
	app := zip.New(zip.Config{DisableStartupMessage: true})
	for _, tc := range cases {
		c := app.TestCtx(http.MethodPost, "/v1/store")
		if tc.val != "" {
			c.Fiber().Request().Header.Set(HeaderUserIsAdmin, tc.val)
		}
		if got := isOrgAdmin(c); got != tc.want {
			t.Errorf("isOrgAdmin(%q)=%v, want %v", tc.val, got, tc.want)
		}
	}
}
