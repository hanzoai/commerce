// Copyright © 2026 Hanzo AI. MIT License.

package iammiddleware

import (
	"io"
	"net/http"
	"net/http/httptest"
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

// TestIAMTokenRequired_NeedsValidatedPrincipal pins the WRITE side of the same
// contract TestIsIAMAuthenticated pins the read side of.
//
// v1.46.5 hardened the reader — IsIAMAuthenticated stopped falling back to raw
// X-Org-Id header presence. But the WRITER kept setting that local from an org
// header alone, so the forged principal just arrived one step earlier. Proven
// live against api.hanzo.ai before this fix: GET /v1/store/current answered 401
// with no headers and 200 with `X-Org-Id: hanzo` and no token, returning another
// org's store record and entitlement status.
//
// The gate is X-User-Id: the one header a client cannot supply, because every
// authority header is stripped on ingress and X-User-Id is re-injected ONLY from
// a verified credential. Same predicate as cloud's clients/principal.Validated
// (`c.User() != ""`).
//
// Driven through a real app rather than a bare TestCtx: these cases take the
// fall-through path, and Next() needs a chain to fall through TO. None of them
// reaches a datastore — the admitted path (both headers) resolves an org and is
// covered by the live suite.
func TestIAMTokenRequired_NeedsValidatedPrincipal(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Use(IAMTokenRequired())
	app.Get("/probe", func(c *zip.Ctx) error {
		if IsIAMAuthenticated(c) {
			return c.String(http.StatusOK, "authenticated")
		}
		return c.String(http.StatusOK, "anonymous")
	})

	for _, tc := range []struct {
		name string
		org  string
		user string
		want string
	}{
		{"org selector alone is NOT a principal", "victim-org", "", "anonymous"},
		{"user with no org", "", "user-123", "anonymous"},
		{"neither", "", "", "anonymous"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/probe", nil)
			if tc.org != "" {
				req.Header.Set(pkgAuth.HeaderOrgID, tc.org)
			}
			if tc.user != "" {
				req.Header.Set(pkgAuth.HeaderUserID, tc.user)
			}
			resp, err := app.Fiber().Test(req)
			if err != nil {
				t.Fatalf("Test: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			b, _ := io.ReadAll(resp.Body)
			if got := string(b); got != tc.want {
				t.Fatalf("probe = %q, want %q — a credential-less caller was treated as an IAM principal", got, tc.want)
			}
		})
	}
}
