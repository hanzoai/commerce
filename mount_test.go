// Copyright © 2026 Hanzo AI. MIT License.

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

	"github.com/hanzoai/zip"
)

// TestMount_RegistersHealth boots an empty zip.App, runs commerce.Mount,
// and asserts the native /_/commerce/healthz route answers 200 with
// {"service":"commerce"}. Covers:
//
//   - Mount() can be called against a fresh *zip.App
//   - cloud.Register fires from init() (Registry contains "commerce")
//   - the native zip health route reaches Fiber and returns JSON
//
// The legacy gin handler surface (proxied via app.Mount(...)) is not
// exercised here — that path is covered by the existing Embed tests
// against the gin engine directly.
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
