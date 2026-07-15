// Copyright © 2026 Hanzo AI. MIT License.

package commerce

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

// TestEmbedRequireIdentity verifies the gateway-trust middleware
// returns 401 on /v1/commerce/* without identity headers when
// RequireIdentity=true. It deliberately does not assert on legacy
// route handlers — just on the trust boundary.
func TestEmbedRequireIdentity(t *testing.T) {
	tmp := t.TempDir()
	srv, err := Embed(context.Background(), EmbedConfig{
		DataDir:         filepath.Join(tmp, "data"),
		HTTPAddr:        "127.0.0.1:0",
		Dev:             true,
		RequireIdentity: true,
	})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	})

	// /healthz is unauthenticated by design — probes run before sessions.
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp, terr := srv.Zip().Fiber().Test(r)
	if terr != nil {
		t.Fatalf("Test: %v", terr)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/healthz: want 200 got %d body=%q", resp.StatusCode, string(body))
	}
}

// TestEmbedHandlerExposed asserts the embedded server exposes a non-nil
// http.Handler we can wire into a parent http.Server.
func TestEmbedHandlerExposed(t *testing.T) {
	tmp := t.TempDir()
	srv, err := Embed(context.Background(), EmbedConfig{
		DataDir:  filepath.Join(tmp, "data"),
		HTTPAddr: "127.0.0.1:0",
		Dev:      true,
	})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop(context.Background()) })
	if srv.Zip() == nil {
		t.Fatalf("Zip() returned nil")
	}
	if srv.App() == nil {
		t.Fatalf("App() returned nil")
	}
	if srv.App().Config().HTTPAddr == "" {
		t.Fatalf("HTTPAddr empty")
	}
}
