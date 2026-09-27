// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package iammiddleware

import (
	"net/http"
	"testing"

	"github.com/zap-proto/zip"
)

// TestGetIAMClaims_HomeIsWhatTheBoundaryAttests: SuperAdmin is read from the home
// org the identity boundary attests (X-User-Owner) and from nothing else. A
// request the boundary validated nobody on carries only the caller's own
// X-Org-Id, and naming the reserved org there grants nothing.
func TestGetIAMClaims_HomeIsWhatTheBoundaryAttests(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	cases := []struct {
		name    string
		headers map[string]string
		want    bool
	}{
		{"nobody validated, X-Org-Id admin", map[string]string{"X-Org-Id": "admin"}, false},
		{"nobody validated, X-Org-Id Admin", map[string]string{"X-Org-Id": "Admin"}, false},
		{"home admin", map[string]string{"X-User-Owner": "admin", "X-Org-Id": "admin"}, true},
		{"home admin, switched into a tenant", map[string]string{"X-User-Owner": "admin", "X-Org-Id": "acme"}, true},
		{"home brand org, X-Org-Id admin", map[string]string{"X-User-Owner": "hanzo", "X-Org-Id": "admin"}, false},
		{"org admin of its own org", map[string]string{"X-User-Owner": "acme", "X-Org-Id": "acme", HeaderUserIsAdmin: "true"}, false},
	}
	for _, tc := range cases {
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
