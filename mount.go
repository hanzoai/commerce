// Copyright © 2026 Hanzo AI. MIT License.

//go:build cloud
// +build cloud

// Mount implements the HIP-0106 unified Hanzo Cloud binary contract.
//
// Commerce is a LIGHT ROUTER, NOT in PCI-DSS scope. It handles tokens
// and intent IDs only; it MUST NEVER touch a PAN. Anything that would
// touch a PAN MUST be refactored to call deps.Payments / deps.Vault
// (ZAP-RPC clients that resolve to the out-of-process Payments + Vault
// services).
//
// NATIVE zip/ZAP — no gin, no net/http adaptation. Commerce registers its
// handlers directly on the host binary's zip.App: one router, one specificity
// space, one middleware chain. There is no second engine and no request
// crossing an adapter boundary.
//
// This file used to bring up a gin engine, take its http.Handler, and front it
// with `app.All("/v1/commerce/*", zip.AdaptNetHTTP(handler))`. v1.48.0 removed
// gin from the serving path, which deleted Embedded.HTTPHandler and left this
// file referring to a method that no longer exists — so every build carrying
// `-tags cloud` failed to compile, and the cloud mount shipped nothing. The
// adapter is not replaced with another adapter; it is deleted, because the
// co-residence contract exists precisely so there is nothing to adapt.

package commerce

import (
	"context"
	"fmt"

	"github.com/hanzoai/cloud"
	"github.com/zap-proto/zip"
)

// Mount registers commerce on app per the HIP-0106 contract.
//
// Per HIP-0106 — commerce is a LIGHT ROUTER, NOT in PCI-DSS scope.
// Token handling only. ANY code path here that would touch a PAN MUST
// be refactored to call deps.Payments / deps.Vault instead.
func Mount(app *zip.App, deps cloud.Deps) error {
	if app == nil {
		return fmt.Errorf("commerce.Mount: nil zip.App")
	}

	logger := deps.Logger
	if logger == nil {
		return fmt.Errorf("commerce.Mount: nil deps.Logger")
	}

	// PCI scope guard. Commerce is mounted because the deployment wants
	// the checkout router; if the out-of-process Payments / Vault
	// clients aren't wired the deployment can still serve tenant config
	// + admin endpoints, but every payment-path handler will fail closed
	// when it reaches for deps.Payments. Warn loudly at startup so the
	// operator sees the gap before a customer does.
	if deps.Payments == nil {
		logger.Warn("commerce.Mount: deps.Payments is nil — payment intent paths will fail; tenant config + admin still served")
	}
	if deps.Vault == nil {
		logger.Warn("commerce.Mount: deps.Vault is nil — vault charge paths unavailable; tenant config + admin still served")
	}

	// The DataDir flows from cloud.Deps so every per-tenant SQLite lands
	// under one tree per HIP-0302.
	dataDir := deps.DataDir
	if dataDir == "" {
		dataDir = "/var/lib/cloud/commerce"
	}

	// App: app is the whole mount. Bootstrap registers commerce's route groups
	// (/v1/commerce public, /_/commerce admin) ON THE HOST'S ROUTER and skips
	// every standalone-only surface — /healthz, the legacy admin SPA, the
	// checkout SPA catch-all, and Listen, since the host owns the listener.
	// HTTPAddr stays empty for the same reason.
	//
	// Logger is left unset: deps.Logger is luxfi/log.Logger and EmbedConfig wants
	// *slog.Logger, so passing it would need a shim. Embed defaults to
	// slog.Default(), which the host has already configured.
	embedded, err := Embed(context.Background(), EmbedConfig{
		DataDir: dataDir,
		App:     app,
		// RequireIdentity stays env-driven; the gateway in front of the
		// cloud binary is the trust boundary per HIP-0026.
		RequireIdentity: false,
	})
	if err != nil {
		return fmt.Errorf("commerce.Mount: embed: %w", err)
	}
	_ = embedded

	// Native zip health endpoint, on a commerce-scoped path so a probe can
	// target this subsystem specifically rather than the whole binary.
	app.Get("/_/commerce/healthz", func(c *zip.Ctx) error {
		return c.JSON(200, map[string]string{
			"status":  "ok",
			"service": "commerce",
		})
	})

	// The /v1 model bundle (api.Route — ~140 routes across /v1/billing,
	// /v1/product, /v1/order …) is deliberately NOT registered here.
	//
	// In the cloud binary those prefixes are cloud's own: /v1/billing is served
	// by clients/billing + clients/account. Registering them again on the same
	// app is not additive — byte-identical patterns MERGE silently with the
	// first registration winning, and two params at equal specificity with
	// different names PANIC at registration. Either way the money path would be
	// decided by import order.
	//
	// So the mounted surface is exactly the commerce-scoped one, which is what
	// it has always been in practice. cmd/commerced still calls api.Route for
	// the standalone binary, where those prefixes have no other owner.

	logger.Info("commerce mounted",
		"prefix.public", "/v1/commerce",
		"prefix.admin", "/_/commerce",
		"data_dir", dataDir,
		"brand", deps.Brand,
	)
	return nil
}

func init() {
	cloud.Register("commerce", 100, func(app any, deps cloud.Deps) error {
		zapp, ok := app.(*zip.App)
		if !ok {
			return fmt.Errorf("commerce.Mount: expected *zip.App, got %T", app)
		}
		return Mount(zapp, deps)
	})
}
