// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package iammiddleware

import (
	"net/http"
	"testing"

	"github.com/zap-proto/zip"
)

// SuperAdmin is the identity edge's verdict and nothing else. The edge — cloud's
// SanitizeIdentity, or authz/edge — strips X-User-IsAdmin from every request and
// writes it back from authz.Claims.Sudo alone: a PERSON whose own org is "admin",
// compared verbatim. X-User-Owner is not that verdict. The boundary writes it for
// an admin-org application and for an admin-org person's sk- key as well as for a
// person, and it carries whatever spelling the org has, so reading it as
// authority made a machine and an "Admin"-cased org SuperAdmin here while they were
// nobody anywhere else in the same process.
func TestGetIAMClaims_SuperAdminIsTheEdgesVerdict(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	for _, tc := range []struct {
		name    string
		headers map[string]string
		want    bool
	}{
		{"the edge's verdict", map[string]string{HeaderUserIsAdmin: "true", "X-User-Owner": "admin", "X-User-Id": "z", "X-Org-Id": "admin"}, true},
		{"the edge's verdict, acting in a tenant", map[string]string{HeaderUserIsAdmin: "true", "X-User-Owner": "admin", "X-User-Id": "z", "X-Org-Id": "acme"}, true},
		{"an admin-org application or key: owner admin, no verdict", map[string]string{"X-User-Owner": "admin", "X-User-Id": "app_platform", "X-Org-Id": "admin"}, false},
		{"an org spelled Admin", map[string]string{"X-User-Owner": "Admin", "X-User-Id": "mallory", "X-Org-Id": "Admin"}, false},
		{"an org spelled ADMIN", map[string]string{"X-User-Owner": "ADMIN", "X-User-Id": "mallory", "X-Org-Id": "ADMIN"}, false},
		{"an owner padded with space", map[string]string{"X-User-Owner": " admin ", "X-User-Id": "mallory", "X-Org-Id": "acme"}, false},
		{"an org admin of its own org", map[string]string{HeaderUserIsOrgAdmin: "true", "X-User-Owner": "acme", "X-User-Id": "wile", "X-Org-Id": "acme"}, false},
		{"nobody validated, X-Org-Id admin", map[string]string{"X-Org-Id": "admin"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := app.TestCtx(http.MethodPut, "/v1/commerce/plans/entries/dev")
			for k, v := range tc.headers {
				c.Fiber().Request().Header.Set(k, v)
			}
			if got := GetIAMClaims(c).IsSuperAdmin(); got != tc.want {
				t.Fatalf("IsSuperAdmin()=%v want %v", got, tc.want)
			}
		})
	}
}
