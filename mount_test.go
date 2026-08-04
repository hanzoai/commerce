// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

//go:build cloud
// +build cloud

package commerce

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hanzoai/cloud"
	luxlog "github.com/luxfi/log"

	"github.com/zap-proto/zip"
)

// TestMount_RegistersHealth boots an empty zip.App, runs commerce.Mount,
// and asserts the native /_/commerce/healthz route answers 200 with
// {"service":"commerce"}. Covers:
//
//   - Mount() can be called against a fresh *zip.App
//   - cloud.Register fires from init() (Registry contains "commerce")
//   - the native zip health route reaches Fiber and returns JSON
//
// The legacy gin handler surface (proxied via app.Mount(...)) is
// covered separately by TestMount_GinSurfaceReachable below.
func TestMount_RegistersHealth(t *testing.T) {
	if !registryContains("commerce") {
		t.Fatalf("cloud.Registry missing 'commerce'; Names=%v", registryNames())
	}

	app := zip.New(zip.Config{Logger: luxlog.New("test")})
	deps := cloud.Deps{
		Logger:  luxlog.New("test"),
		Brand:   "hanzo",
		Domain:  "api.hanzo.ai",
		DataDir: t.TempDir(),
	}
	if err := Mount(app, deps); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	req := httptest.NewRequest("GET", "/_/commerce/healthz", nil)
	resp, err := app.Fiber().Test(req)
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

// TestMount_GinSurfaceReachable proves the inner gin engine is actually
// reachable through the outer zip.App. The /v1/commerce/tenant public route
// is registered on the embedded gin engine; routing a request through
// zip → AdaptNetHTTP → gin must reach the org-as-tenant handler. We accept
// any non-NoRoute response — org resolution returns 200 with the tenant JSON —
// which proves the route resolved past zip's adapter into a real handler
// rather than falling through to the SPA NoRoute branch.
//
// Without this test, mount.go could regress to "zip mounts the prefix
// but the inner engine is broken" and TestMount_RegistersHealth would
// still pass (it only exercises the native zip route).
func TestMount_GinSurfaceReachable(t *testing.T) {
	app := zip.New(zip.Config{Logger: luxlog.New("test")})
	deps := cloud.Deps{
		Logger:  luxlog.New("test"),
		Brand:   "hanzo",
		Domain:  "api.hanzo.ai",
		DataDir: t.TempDir(),
	}
	if err := Mount(app, deps); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	req := httptest.NewRequest("GET", "/v1/commerce/tenant", nil)
	req.Host = "pay.example.test"
	resp, err := app.Fiber().Test(req)
	if err != nil {
		t.Fatalf("Fiber Test: %v", err)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(raw)

	// NoRoute SPA fallback for API paths returns exactly the canonical
	// `{"error":"not found"}` body. Seeing that proves the request never
	// reached a real gin handler.
	if body == `{"error":"not found"}` {
		t.Fatalf("/v1/commerce/tenant fell through to NoRoute SPA — gin surface not wired through zip mount. status=%d body=%s", resp.StatusCode, body)
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
	deps := cloud.Deps{
		Logger:  luxlog.New("test"),
		Brand:   "hanzo",
		Domain:  "api.hanzo.ai",
		DataDir: t.TempDir(),
	}
	if err := Mount(app, deps); err != nil {
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
	resp, err := app.Fiber().Test(req)
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

func registryContains(name string) bool {
	for _, s := range cloud.Registry {
		if s.Name == name {
			return true
		}
	}
	return false
}

func registryNames() []string {
	out := make([]string, 0, len(cloud.Registry))
	for _, s := range cloud.Registry {
		out = append(out, s.Name)
	}
	return out
}
