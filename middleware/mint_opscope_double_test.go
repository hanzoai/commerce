package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A typed op registered through a Mint router must run its handler EXACTLY ONCE.
//
// mintRouter.OpScope carries the mint gate down to typed ops, which it has to:
// without it, deposit / refund / credit-grants / payouts would register with no
// authorization at all. But the gate is PlatformOnly, a fiber handler that
// continues by calling c.Next() and then returns nil — so a middleware shaped as
//
//	if err := m.gate(c); err != nil { return err }
//	return next(c)
//
// has two continuations for one request: c.Next() inside the gate, and next(c)
// after it. On the 403 path that is harmless — the error short-circuits. On the
// AUTHORIZED path, if both reach the handler, a money-mint op executes twice per
// request: a double deposit, a double refund.
//
// This asserts the COUNT, because "reached" cannot tell one run from two, and on
// a ledger path that difference is the whole point.
func TestMintOpScope_AuthorizedOpRunsHandlerOnce(t *testing.T) {
	const tok = "svc-secret-xyz"
	t.Setenv("COMMERCE_SERVICE_TOKEN", tok)
	ctx := ae.NewContext()
	defer ctx.Close()

	runs := 0
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Use(zip.H(func(c *zip.Ctx) error { c.SetContext(ctx); return c.Next() }))
	app.Use(TokenRequired(permission.Admin))

	// Register the way a TYPED OP does: take the gated router's OpScope and let
	// it wrap the handler, rather than going through Post() — that path prepends
	// the gate as an ordinary fiber handler and is already covered by mint_test.go.
	scope := Mint(app.Group(""), "").OpScope()
	h := zip.Handler(func(c *zip.Ctx) error { runs++; return c.NoContent(http.StatusOK) })
	if scope.Middleware != nil {
		h = scope.Middleware(h)
	}
	app.Post("/x", h)

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Org-Id", "svc-org")
	resp, err := app.Fiber().Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	t.Logf("status=%d handler runs=%d", resp.StatusCode, runs)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want 200 — the authorized mint path must still reach the handler", resp.StatusCode)
	}
	if runs != 1 {
		t.Fatalf("handler ran %d times, want exactly 1 — a mint op must not double-execute", runs)
	}
}

// The gate must still REFUSE an unauthorized caller on the typed-op path, and the
// handler must not run at all. A double-execution fix that also opened the gate
// would trade one defect for a worse one.
func TestMintOpScope_UnauthorizedOpNeverReachesHandler(t *testing.T) {
	t.Setenv("COMMERCE_SERVICE_TOKEN", "svc-secret-xyz")
	ctx := ae.NewContext()
	defer ctx.Close()

	runs := 0
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Use(zip.H(func(c *zip.Ctx) error { c.SetContext(ctx); return c.Next() }))
	app.Use(TokenRequired(permission.Admin))

	scope := Mint(app.Group(""), "").OpScope()
	h := zip.Handler(func(c *zip.Ctx) error { runs++; return c.NoContent(http.StatusOK) })
	if scope.Middleware != nil {
		h = scope.Middleware(h)
	}
	app.Post("/x", h)

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("Authorization", "Bearer not-the-service-token")
	req.Header.Set("X-Org-Id", "svc-org")
	resp, err := app.Fiber().Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	t.Logf("status=%d handler runs=%d", resp.StatusCode, runs)
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("status=200 for a non-platform caller — the mint gate did not hold on the typed-op path")
	}
	if runs != 0 {
		t.Fatalf("handler ran %d times for an unauthorized caller, want 0", runs)
	}
}
