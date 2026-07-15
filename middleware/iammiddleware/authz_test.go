// Copyright © 2026 Hanzo AI. MIT License.

package iammiddleware

import (
	"net/http"
	"testing"

	"github.com/zap-proto/zip"

	pkgAuth "github.com/hanzoai/commerce/pkg/auth"
)

// TestIsIAMAuthenticated pins the hardened contract: authentication is proven
// ONLY by the validated iam_authenticated local (set by IAMTokenRequired after
// it resolves the org from the trusted, post-strip X-Org-Id). A bare client
// X-Org-Id header must NOT count — trusting header presence let an unvalidated
// opaque bearer + client X-Org-Id forge an IAM principal for any org (the
// checkout money-surface bypass).
func TestIsIAMAuthenticated(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*zip.Ctx)
		want  bool
	}{
		{"validated local true", func(c *zip.Ctx) {
			c.Locals("iam_authenticated", true)
		}, true},
		{"bare X-Org-Id header is NOT authentication", func(c *zip.Ctx) {
			c.Fiber().Request().Header.Set(pkgAuth.HeaderOrgID, "hanzo")
		}, false},
		{"nothing set", func(c *zip.Ctx) {}, false},
		{"explicit false", func(c *zip.Ctx) {
			c.Locals("iam_authenticated", false)
		}, false},
	}

	app := zip.New(zip.Config{DisableStartupMessage: true})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := app.TestCtx(http.MethodPost, "/v1/checkout/sessions")
			tc.setup(c)
			if got := IsIAMAuthenticated(c); got != tc.want {
				t.Fatalf("IsIAMAuthenticated=%v, want %v", got, tc.want)
			}
		})
	}
}
