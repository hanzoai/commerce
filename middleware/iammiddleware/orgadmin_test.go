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

// TestIsOrgAdmin_ReadsBothSpellingsTheEdgesMint — the two identity edges mint
// DIFFERENT headers for the same authority: the gateway sets X-User-IsAdmin,
// while cloud's in-process boundary reserves that for a SuperAdmin and sets
// X-User-IsOrgAdmin for a merchant admin. A predicate that reads only one of
// them refuses a real org admin depending on where the request entered.
func TestIsOrgAdmin_ReadsBothSpellingsTheEdgesMint(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	for _, header := range []string{HeaderUserIsAdmin, HeaderUserIsOrgAdmin} {
		c := app.TestCtx(http.MethodPost, "/v1/billing/risk/controls")
		c.Fiber().Request().Header.Set(header, "true")
		if !isOrgAdmin(c) {
			t.Fatalf("%s: true did not mark an org admin", header)
		}
	}
}

// TestIsOrgAdmin_IsTheRecordedAnswerAndFailsClosed — the exported predicate
// reads the answer IAMTokenRequired already computed WITH the home==effective
// binding, rather than re-deriving it from headers. A second derivation is a
// second chance to forget the binding, and the one that forgets it hands a
// merchant admin authority over a foreign tenant.
func TestIsOrgAdmin_IsTheRecordedAnswerAndFailsClosed(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})

	// Absent: no IAM middleware ran, or the caller is not one.
	c := app.TestCtx(http.MethodGet, "/v1/billing/risk/controls")
	if IsOrgAdmin(c) {
		t.Fatal("an unmarked request reported an org admin")
	}
	// The headers ALONE are not enough — the recorded answer is what counts,
	// because only that carries the home==effective check.
	c.Fiber().Request().Header.Set(HeaderUserIsOrgAdmin, "true")
	if IsOrgAdmin(c) {
		t.Fatal("a header alone granted org-admin authority without the home==effective binding")
	}
	c.Locals(LocalOrgAdmin, true)
	if !IsOrgAdmin(c) {
		t.Fatal("the recorded answer was not read back")
	}
	c.Locals(LocalOrgAdmin, "true") // a non-bool must not be read as one
	if IsOrgAdmin(c) {
		t.Fatal("a non-bool marker was treated as an org admin")
	}
	if IsOrgAdmin(nil) {
		t.Fatal("a nil request reported an org admin")
	}
}

// TestOrgAdminHomeMatches_RefusesAForeignTenant — the binding itself.
func TestOrgAdminHomeMatches_RefusesAForeignTenant(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	c := app.TestCtx(http.MethodPost, "/v1/billing/risk/controls")
	c.Fiber().Request().Header.Set(HeaderUserOwner, "acme")
	if !orgAdminHomeMatches(c, "acme") {
		t.Fatal("a caller acting in its own org did not match")
	}
	if orgAdminHomeMatches(c, "victim") {
		t.Fatal("an org-switched caller inherited authority over a foreign org")
	}
	blank := app.TestCtx(http.MethodPost, "/v1/billing/risk/controls")
	if orgAdminHomeMatches(blank, "acme") {
		t.Fatal("a caller with no home org matched one")
	}
}
