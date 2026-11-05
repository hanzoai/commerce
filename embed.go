// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

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
	"strings"
	"time"

	"github.com/hanzoai/commerce/secrets"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/billing/depositledger"
	"github.com/hanzoai/commerce/billing/gateway"
	"github.com/hanzoai/commerce/mail"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/webhook/census"
)

// legacyWebhooks logs the census of commerce's legacy webhook rows: one line per
// row (store, org, host, a digest of the URL, the events it takes — never a
// credential) and one line with the totals. See models/webhook/census.
func legacyWebhooks(l Log) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	subs, err := census.Subscribers(ctx)
	if err != nil {
		l.Warn("commerce: legacy webhook census failed", "err", err.Error(), "read", len(subs))
		return
	}
	stores := map[string]int{}
	for _, s := range subs {
		stores[s.Store]++
		l.Warn("commerce: legacy webhook subscriber", "store", s.Store, "org", s.Org, "id", s.ID,
			"name", s.Name, "host", s.Host, "url_hash", s.URLHash, "all", s.All,
			"events", strings.Join(s.Events, ","), "enabled", s.Enabled, "live", s.Live)
	}
	l.Info("commerce: legacy webhook census", "rows", len(subs), "system", stores["system"], "org", stores["org"])
}

// EmbedConfig configures the in-process Commerce server. Empty values
// fall through to commerce.DefaultConfig (env-based) so commerced binds
// the same env contract the legacy commerce binary did.
// SecretReader is the host's secret plane, as commerce needs it: one read, by
// reference, returning the value or an error. Deliberately narrower than any
// KMS client — commerce never writes, never lists, and never learns the
// transport, so a host can satisfy this over UDS, in-process, or from a test
// map without commerce knowing which.
type SecretReader interface {
	GetSecret(ctx context.Context, ref string) ([]byte, error)
}

// Log is the host's logger as commerce needs it: two levels and key/value
// pairs. It is an interface rather than *slog.Logger because the hosts differ —
// the standalone boot has an *slog.Logger, the cloud binary has a
// luxfi/log.Logger — and both already have these two methods, so neither side
// writes a shim and commerce learns nothing about either logger's package.
type Log interface {
	Info(msg string, kv ...any)
	Warn(msg string, kv ...any)
}

type EmbedConfig struct {
	DataDir         string   // "" → COMMERCE_DIR or ./commerce_data
	HTTPAddr        string   // "" → COMMERCE_HTTP or 127.0.0.1:8090
	Dev             bool     // dev mode — reload-friendly logging
	RequireIdentity bool     // gateway trust: refuse requests without X-Org-Id/X-User-Id
	Logger          Log      // nil → slog.Default()
	AllowedOrigins  []string // CORS — usually ["*"] behind gateway

	// App is the NATIVE co-residence contract: when set, commerce registers
	// its routes directly on this shared zip app (one router, one specificity
	// space — no handler adaptation, no second engine) and skips the
	// standalone-only surfaces (/healthz, the legacy /admin SPA, the checkout
	// SPA root catch-all, Listen). nil → commerce builds its own app and
	// serves standalone.
	App *zip.App

	// MasterKey is the 32-byte at-rest encryption master key. When set it is used
	// verbatim and COMMERCE_KMS_MASTER_KEY is never read — so a host that already
	// holds a KEK passes the one it has instead of a second key being provisioned
	// for the same process. nil ⇒ the env var decides, unchanged, which is the
	// standalone path.
	MasterKey []byte

	// Ledger is the host-injected double-entry credit ledger. When the cloud
	// binary embeds commerce it passes a ledger-backed impl here, so
	// POST /v1/billing/credit and GET /v1/billing/balance route to the SAME
	// per-org account the AI spend-gate reads (one ledger, no split). nil →
	// commerce falls back to its own datastore (standalone-safe).
	Ledger creditledger.CreditLedger
	// Secrets is commerce's one secret reader: the host's KMS handle (the cloud
	// binary passes deps.KMS) or, for the standalone binary, hanzoai/kms/sdk/go.
	// Commerce holds no KMS client of its own. An org's payment credentials are
	// read through it at orgs/<org>/<provider>/<NAME> (payment.Hydrate). The same
	// inversion Ledger uses: a plugin reads secrets through its host.
	//
	// nil reads nothing: every credential is then absent.
	Secrets SecretReader

	// Mail delivers commerce's outbound mail — the sale notice to the operator
	// (BILLING_SALE_NOTIFY_TO). The cloud binary passes its notify rail, whose
	// provider credentials are in KMS, so commerce holds no mail credential of
	// its own. nil sends nothing (mail.ErrNoSender, logged by the caller).
	Mail mail.Sender

	// Born is the kind a new org's record is created in (organization.Born):
	// Live where the host's network takes real money, Test where it does not.
	// Only the host knows which world it runs in, so it declares it. Anything
	// but Live, "" included, is Test.
	Born organization.Kind

	// Ceiling is the most any org transacts in (organization.Ceiling): Test
	// where the host's network holds no real money, so even a record marked
	// Live charges sandbox credentials and writes the test books. "" and Live
	// are no cap; any other value caps at Test.
	Ceiling organization.Kind
}

// ceilingOf is the cap a host's declaration puts on every org: none for "" or
// Live, Test for anything else, so a value nobody recognises caps rather than
// lifts.
func ceilingOf(k organization.Kind) organization.Kind {
	if k == "" || k == organization.Live {
		return organization.Live
	}
	return organization.Test
}

// Embedded is the handle to a running in-process Commerce server. The
// underlying *App owns the heavy lifting (DB, infra, KMS,
// hooks, cron) — Embedded wraps it for clean Stop/Zip access.
type Embedded struct {
	cfg EmbedConfig
	app *App
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
	if len(cfg.MasterKey) > 0 {
		appCfg.MasterKey = cfg.MasterKey
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

	// The host's secret plane, installed before routes register — same shape as
	// the ledger above, and for the same reason: a plugin asks its host.
	if cfg.Secrets != nil {
		secrets.Set(cfg.Secrets)
	}

	// The host's mail rail, installed like the two above. Each embed states its
	// whole world, so nil clears whatever an earlier embed in the process set.
	mail.Set(cfg.Mail)

	// The kind new tenants are born in, declared before any route can create
	// one. Each embed states its whole world, so one that declares nothing is
	// Test, never whatever an earlier embed in the process left behind.
	organization.Born = organization.Test
	if cfg.Born == organization.Live {
		organization.Born = organization.Live
	}
	// The cap every org transacts under, declared in the same act and for the
	// same reason. Stated whole each embed: one that declares no cap clears a
	// cap an earlier embed in the process set.
	organization.Ceiling = ceilingOf(cfg.Ceiling)

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

	// Start the crypto deposit schedule. This is the PRODUCTION entry point — no
	// standalone commerce backend is deployed, so Embed is the only path by
	// which the rail ever runs — and it is the right owner because this is the
	// call that produces the handle whose Stop() stops it. Start and stop now
	// sit on the same object; before, Bootstrap started it and only Shutdown
	// stopped it, so a caller that took the handle never had the pair.
	//
	// A no-op when no CRYPTO_DEPOSIT_* asset is configured, which is why the
	// tests that Embed a bare commerce spin up nothing.
	depositWatcher := depositledger.Default()
	depositWatcher.Start()
	// The USDC authorization reconciler: follows every relayed payment to its
	// confirmation and credit. A no-op unless USDC_TREASURY armed the rail.
	gateway.Default().Start()

	// Who commerce's legacy webhook rows deliver to, logged once per boot so the
	// move to /v1/webhook names every subscriber before the rows are deleted.
	// Detached and bounded: it reads every org's store, and a boot waits for none.
	go legacyWebhooks(cfg.Logger)

	cfg.Logger.Info("commerce.Embed ready",
		"http", appCfg.HTTPAddr,
		"data", appCfg.DataDir,
		"dev", appCfg.Dev,
		"require_identity", cfg.RequireIdentity,
		"version", Version,
		// The boot fact that used to go to stderr and could not be read back off
		// a running pod. It is here because a structured log line is greppable,
		// and — the part that matters — it is also served live at
		// GET /v1/commerce/deposits, so the answer does not depend on catching it.
		"deposit_watcher", depositWatcher.Running(),
		"deposit_assets", len(depositWatcher.Assets()),
		"gateway_rail", gateway.Default().Armed(),
		"gateway_network", gateway.Default().Network(),
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
