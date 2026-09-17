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

// A route beneath Mint must run its handler EXACTLY ONCE.
//
// The gate is PlatformOnly, a fiber handler that continues by calling c.Next()
// and then returns nil. Wrapped as
//
//	if err := gate(c); err != nil { return err }
//	return next(c)
//
// that is two continuations for one request: c.Next() inside the gate, and
// next(c) after it. On the 403 path the error short-circuits and it does not
// matter. On the AUTHORIZED path both reach the handler, and a money route
// executes twice per request — a double deposit, a double refund.
//
// So this asserts the COUNT. "Reached" cannot tell one run from two, and on a
// ledger path that is the whole difference.
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

// And the gate must still refuse an unauthorized caller, with the handler not
// running at all. A double-execution fix that also opened the gate would trade
// one defect for a worse one.
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
