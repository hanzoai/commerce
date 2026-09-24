// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package commerce

import (
	"context"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hanzoai/commerce/ui"
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
	resp, terr := srv.Zip().Test(r)
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

// TestEmbedAdminServesItsOwnAssets asserts the embedded admin is served from the
// mount it was BUILT for. Twice now the binary has shipped an export whose chunk
// URLs were root-absolute (/_next/*): they escape the /admin mount, and because
// the static handler falls back to index.html every miss still answers 200 — with
// HTML. So a status check alone cannot see this break; the content type is what
// tells a real chunk from the fallback. Asserting on the URLs the shell actually
// asks for keeps the export and the mount one contract.
func TestEmbedAdminServesItsOwnAssets(t *testing.T) {
	// ui/dist is BUILD OUTPUT: a fresh clone embeds only .gitkeep, and there is no
	// admin to assert on. Decide that from the embedded FS rather than from a
	// response — an unsynced tree must skip here, not fail, or `go test ./...` is
	// red for anyone who has not run the producer.
	if _, err := fs.Stat(ui.FS(), "index.html"); err != nil {
		t.Skip("ui/dist holds no export — run scripts/sync-admin-ui.sh")
	}

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

	get := func(path string) (*http.Response, string) {
		t.Helper()
		resp, terr := srv.Zip().Test(httptest.NewRequest(http.MethodGet, path, nil))
		if terr != nil {
			t.Fatalf("GET %s: %v", path, terr)
		}
		body, _ := io.ReadAll(resp.Body)
		return resp, string(body)
	}

	resp, shell := get("/admin/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /admin/: want 200 got %d", resp.StatusCode)
	}

	// Collect the absolute asset URLs the shell names, WITHOUT assuming the prefix:
	// asserting on /admin/_next/ would make an export built for "/" look like an
	// absent one and skip past the very regression this guards.
	refs := regexp.MustCompile(`(?:src|href)="(/[^"]*_next/[^"]+)"`).FindAllStringSubmatch(shell, -1)
	if len(refs) == 0 {
		t.Fatalf("the shell references no _next assets — this is not the Next export")
	}

	for _, m := range refs {
		asset := m[1]
		if !strings.HasPrefix(asset, adminMount+"/") {
			t.Errorf("%s is root-absolute: the export was built for %q, not the %q mount — set basePath (app/admin/src/lib/basepath.ts)", asset, "/", adminMount)
			continue
		}
		r, _ := get(asset)
		if r.StatusCode != http.StatusOK {
			t.Errorf("GET %s: want 200 got %d", asset, r.StatusCode)
			continue
		}
		if ct := r.Header.Get("Content-Type"); strings.HasPrefix(ct, "text/html") {
			t.Errorf("GET %s: served the index.html fallback (%s) — the asset is not under the mount", asset, ct)
		}
	}
}

// Where commerce.go mounts the admin, and what app/admin is built for (BASE_PATH).
const adminMount = "/admin"
