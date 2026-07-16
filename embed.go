// Copyright © 2026 Hanzo AI. MIT License.

// Package commerce (pkg/commerce) is the embedded Commerce server. One
// backend, one HTTP handler. Mirrors pkg/tasks/embed.go shape:
//
//	cfg := commerce.EmbedConfig{DataDir: "/var/lib/commerce", HTTPAddr: ":8090"}
//	srv, err := commerce.Embed(ctx, cfg)
//	defer srv.Stop(ctx)
//
// The legacy App in commerce.go is the bootstrap — server.go wires it
// into commerced/main.go cleanly. The /v1/commerce/* surface stays
// behind hanzoai/gateway and is gated by COMMERCED_REQUIRE_IDENTITY.
package commerce

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/creditledger"
)

// EmbedConfig configures the in-process Commerce server. Empty values
// fall through to commerce.DefaultConfig (env-based) so commerced binds
// the same env contract the legacy commerce binary did.
type EmbedConfig struct {
	DataDir         string       // "" → COMMERCE_DIR or ./commerce_data
	HTTPAddr        string       // "" → COMMERCE_HTTP or 127.0.0.1:8090
	Dev             bool         // dev mode — reload-friendly logging
	RequireIdentity bool         // gateway trust: refuse requests without X-Org-Id/X-User-Id
	Logger          *slog.Logger // nil → slog.Default()
	AllowedOrigins  []string     // CORS — usually ["*"] behind gateway

	// App is the NATIVE co-residence contract: when set, commerce registers
	// its routes directly on this shared zip app (one router, one specificity
	// space — no handler adaptation, no second engine) and skips the
	// standalone-only surfaces (/healthz, the legacy /admin SPA, the checkout
	// SPA root catch-all, Listen). nil → commerce builds its own app and
	// serves standalone.
	App *zip.App

	// Ledger is the host-injected double-entry credit ledger. When the cloud
	// binary embeds commerce it passes a ledger-backed impl here, so
	// POST /v1/billing/credit and GET /v1/billing/balance route to the SAME
	// per-org account the AI spend-gate reads (one ledger, no split). nil →
	// commerce falls back to its own datastore (standalone-safe).
	Ledger creditledger.CreditLedger
}

// Embedded is the handle to a running in-process Commerce server. The
// underlying *App owns the heavy lifting (DB, infra, KMS,
// hooks, cron) — Embedded wraps it for clean Stop/Zip access.
type Embedded struct {
	cfg EmbedConfig
	app *App
	// brand is the deployment's white-label brand, surfaced in the in-process
	// commerce.Client's OrgConfig. Set by Mount; empty for a bare Embed
	// (the standalone/legacy boot never serves the inter-subsystem client).
	brand string
}

// Embed bootstraps the Commerce app and returns a handle. Call Stop
// before the process exits.
func Embed(ctx context.Context, cfg EmbedConfig) (*Embedded, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	appCfg := DefaultConfig()
	if cfg.DataDir != "" {
		appCfg.DataDir = cfg.DataDir
	}
	if cfg.HTTPAddr != "" {
		appCfg.HTTPAddr = cfg.HTTPAddr
	}
	if cfg.Dev {
		appCfg.Dev = true
	}
	if len(cfg.AllowedOrigins) > 0 {
		appCfg.AllowedOrigins = cfg.AllowedOrigins
	}
	// Bool override mirrors the cfg.Dev pattern: an explicit true from the
	// caller strengthens the env-derived default; an unset (false) cfg falls
	// through to DefaultConfig's COMMERCED_REQUIRE_IDENTITY read.
	if cfg.RequireIdentity {
		appCfg.RequireIdentity = true
	}
	appCfg.SharedApp = cfg.App

	// Install the host-injected credit ledger (process-wide) BEFORE routes are
	// registered, so the billing credit + balance handlers resolve it. nil is a
	// no-op: commerce keeps its datastore path (standalone-safe).
	creditledger.Set(cfg.Ledger)

	app := NewWithConfig(appCfg)

	// Run Bootstrap synchronously so the returned Embedded is fully
	// ready: Router populated (the shared app in co-resident mode), DB
	// connected, hooks fired. setupRoutes is called inside Bootstrap.
	// Bootstrap installs the identity trust boundary (EdgeAuth + auth.Gin)
	// BEFORE registering any route group and mounts the admin SPA after — so
	// /_/commerce/*, /v1/commerce/* and the post-Bootstrap /v1 api.Route()
	// bundle all inherit it. (Previously this binary mounted the boundary
	// here, AFTER Bootstrap had already registered the setupRoutes groups, so
	// gin left those groups unguarded — the in-cluster header-forge hole.)
	if err := app.Bootstrap(); err != nil {
		return nil, fmt.Errorf("commerce.Embed: bootstrap: %w", err)
	}

	cfg.Logger.Info("commerce.Embed ready",
		"http", appCfg.HTTPAddr,
		"data", appCfg.DataDir,
		"dev", appCfg.Dev,
		"require_identity", cfg.RequireIdentity,
		"version", Version,
	)

	return &Embedded{cfg: cfg, app: app}, nil
}

// Zip returns the zip app commerce's routes live on — its own in
// standalone mode, the shared app in co-resident mode.
func (e *Embedded) Zip() *zip.App {
	if e == nil || e.app == nil {
		return nil
	}
	return e.app.Router
}

// App exposes the underlying App for tests and hook registration.
func (e *Embedded) App() *App {
	if e == nil {
		return nil
	}
	return e.app
}

// Stop shuts the server down. Idempotent.
func (e *Embedded) Stop(ctx context.Context) error {
	if e == nil || e.app == nil {
		return nil
	}
	if err := e.app.Shutdown(); err != nil {
		return err
	}
	_ = ctx
	return nil
}
