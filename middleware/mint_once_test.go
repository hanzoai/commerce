package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// An authorized money route runs its handler once.
func TestMintRunsTheHandlerOnce(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	runs := 0
	platform := &auth.IAMClaims{Owner: "admin"}
	platform.Subject = "app_platform"
	identity := iamIdentity(bit.Field(permission.Admin|permission.Live), platform)
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Use(zip.H(func(c *zip.Ctx) error { c.SetContext(ctx); identity(c); return c.Next() }))
	app.Use(TokenRequired(permission.Admin))

	Mint(app.Group(""), "").Raw(http.MethodPost, "/x", func(c *zip.Ctx) error {
		runs++
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("X-Org-Id", "svc-org")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want 200 — the authorized mint path must still reach the handler", resp.StatusCode)
	}
	if runs != 1 {
		t.Fatalf("handler ran %d times, want exactly 1 — a mint route must not double-execute", runs)
	}
}

// An unauthorized caller never reaches the handler.
func TestMintRefusesAnUnauthorizedCaller(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	runs := 0
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Use(zip.H(func(c *zip.Ctx) error { c.SetContext(ctx); return c.Next() }))
	app.Use(TokenRequired(permission.Admin))

	Mint(app.Group(""), "").Raw(http.MethodPost, "/x", func(c *zip.Ctx) error {
		runs++
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("Authorization", "Bearer not-the-service-token")
	req.Header.Set("X-Org-Id", "svc-org")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode == http.StatusOK {
		t.Fatalf("status=200 for a non-platform caller — the mint gate did not hold")
	}
	if runs != 0 {
		t.Fatalf("handler ran %d times for an unauthorized caller, want 0", runs)
	}
}
