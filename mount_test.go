// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package commerce

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	luxlog "github.com/luxfi/log"

	"github.com/zap-proto/zip"
)

// TestMount_RegistersHealth boots an empty zip.App, runs commerce.Mount,
// and asserts the native /_/commerce/healthz route answers 200 with
// {"service":"commerce"}. Covers:
//
//   - Mount() can be called against a fresh *zip.App
//   - the native zip health route reaches Fiber and returns JSON
//
// The rest of the mounted surface is covered by TestMount_GinSurfaceReachable
// below.
func TestMount_RegistersHealth(t *testing.T) {
	app := zip.New(zip.Config{Logger: luxlog.New("test")})
	if err := Mount(app, t.TempDir(), luxlog.New("test")); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	req := httptest.NewRequest("GET", "/_/commerce/healthz", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Fiber Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status: got %d want 200", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, `"service":"commerce"`) {
		t.Fatalf("body: got %q, want service=commerce", body)
	}
}

// TestMount_PublicSurfaceReachable proves the /v1/commerce group actually
// reaches commerce's handlers on the host's app — not just the health route
// Mount registers by hand.
//
// It asks for the public catalog, which needs no host resolution and no
// credential, so a 200 means the route resolved into the real handler. It used
// to ask for /v1/commerce/tenant and only assert the body was not the SPA
// fallback string; that route stopped existing when org-as-tenant became
// GET /v1/commerce/org, and a mounted app has no SPA fallback to produce that
// string anyway, so the assertion held no matter what — including if Mount
// registered nothing at all. Which is the regression it was written to catch.
func TestMount_PublicSurfaceReachable(t *testing.T) {
	app := zip.New(zip.Config{Logger: luxlog.New("test")})
	if err := Mount(app, t.TempDir(), luxlog.New("test")); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	req := httptest.NewRequest("GET", "/v1/commerce/catalog", nil)
	req.Host = "pay.example.test"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Fiber Test: %v", err)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("GET /v1/commerce/catalog = %d (%s); the mount did not register commerce's public group on the host app",
			resp.StatusCode, strings.TrimSpace(string(raw)))
	}
}

// TestMount_NoUngatedTypedOpSurface is a TRIPWIRE, not a feature test.
//
// zip auto-derives an MCP tool surface from the typed-op registry and mounts it
// at /mcp — "enabled by default", installed by prepare() whenever len(app.ops)
// > 0 (zip/mcp.go). Every typed op becomes a tool whose tools/call runs the op's
// invoke core DIRECTLY. That path does not traverse the Fiber route chain, so
// NO transport middleware runs on it — including the money-mint gate.
//
// Concretely: registering Deposit as a zip.Post[In,Out] would publish
// billing.deposit as an MCP tool that any caller can invoke with no credential,
// while middleware.PlatformOnly (mounted as route middleware) never fires. That
// mints unbounded spendable balance — the exact real-money-GA hole PlatformOnly
// exists to close.
//
// The only seam that gates the invoke core is App.Authorize (zip/authorize.go),
// which runs over REST and MCP alike. It landed in zip v1.8.3; NO earlier
// version — including the v1.8.2 the cloud binary pins — can gate a typed op at
// all. Authorize is also a single app-level slot (a setter, not an appender), so
// on a shared App the LAST subsystem to call it silently wins.
//
// Commerce therefore contributes ZERO typed ops today: its surface is the gin
// engine behind AdaptNetHTTP, whose middleware chain is intact. This test pins
// that. It fails the moment commerce registers a typed op — at which point the
// author MUST prove the money ops are gated at the invoke seam (App.Authorize on
// zip >= v1.8.3, or MCP disabled) before deleting this guard.
func TestMount_NoUngatedTypedOpSurface(t *testing.T) {
	app := zip.New(zip.Config{Logger: luxlog.New("test")})
	if err := Mount(app, t.TempDir(), luxlog.New("test")); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	// Install the deferred projections (OpenAPI + MCP) exactly as a served app
	// does. zip mounts /mcp here iff at least one typed op is registered.
	if err := app.Build(); err != nil {
		t.Fatalf("Build: %v", err)
	}

	req := httptest.NewRequest("POST", "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Fiber Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("commerce.Mount now registers typed zip ops: /mcp answered %d (%s).\n"+
			"Every typed op is an MCP tool whose tools/call BYPASSES route middleware —\n"+
			"so middleware.PlatformOnly does NOT gate it. Before removing this guard,\n"+
			"gate the money ops at the invoke seam via app.Authorize (zip >= v1.8.3)\n"+
			"and prove an unauthenticated tools/call of a mint op is refused.",
			resp.StatusCode, strings.TrimSpace(string(raw)))
	}
}
