// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package commerce

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	luxlog "github.com/luxfi/log"
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
)

// A group the host opens on the shared app after Embed runs the identity gate,
// then IAMTokenRequired, then TokenRequired — cloud's /v1/commerce chain. An
// X-Org-Id with no validated X-User-Id authenticates nobody there.
func TestSharedAppOrgHeaderAloneIsRefused(t *testing.T) {
	app := zip.New(zip.Config{Logger: luxlog.New("test")})
	srv, err := Embed(context.Background(), EmbedConfig{DataDir: t.TempDir(), HTTPAddr: "127.0.0.1:0", Dev: true, App: app})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop(context.Background()) })

	g := app.Group("/v1/commerce/probe")
	g.Use(iammiddleware.IAMTokenRequired())
	g.Raw(http.MethodGet, "", middleware.TokenRequired(), func(c *zip.Ctx) error { return c.String(http.StatusOK, "reached") })

	req := httptest.NewRequest(http.MethodGet, "/v1/commerce/probe", nil)
	req.Header.Set("X-Org-Id", "victim")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("X-Org-Id alone: status %d, want 401", resp.StatusCode)
	}
}
