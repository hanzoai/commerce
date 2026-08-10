// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

// Mount implements the HIP-0106 unified Hanzo Cloud binary contract.
//
// Commerce is a LIGHT ROUTER, NOT in PCI-DSS scope. It handles tokens
// and intent IDs only; it MUST NEVER touch a PAN. Anything that would
// touch a PAN belongs behind the out-of-process Payments + Vault services,
// which the host calls — commerce holds no client to either.
//
// NATIVE zip/ZAP — no gin, no net/http adaptation. Commerce registers its
// handlers directly on the host binary's zip.App: one router, one specificity
// space, one middleware chain. There is no second engine and no request
// crossing an adapter boundary.
//
// A host passes a data dir and a logger. It does not pass itself: commerce is
// mounted BY a host, so naming a host type here would let exactly one host
// mount it and would point the dependency backwards — the host already imports
// commerce in order to mount it.

package commerce

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/zap-proto/zip"
)

// Mount registers commerce on app per the HIP-0106 contract.
//
// dataDir is the per-deployment data root; per-tenant SQLite lands under it
// per HIP-0302. Empty falls back to the cloud default tree. log is the host's
// logger — nil means slog.Default(), the same rule Embed uses, so a missing
// log sink never costs a deployment its checkout router.
func Mount(app *zip.App, dataDir string, log Log) error {
	if app == nil {
		return fmt.Errorf("commerce.Mount: nil zip.App")
	}
	if log == nil {
		log = slog.Default()
	}
	if dataDir == "" {
		dataDir = "/var/lib/cloud/commerce"
	}

	// App: app is the whole mount. Bootstrap registers commerce's route groups
	// (/v1/commerce public, /_/commerce admin) ON THE HOST'S ROUTER and skips
	// every standalone-only surface — /healthz, the legacy admin SPA, the
	// checkout SPA catch-all, and Listen, since the host owns the listener.
	// HTTPAddr stays empty for the same reason.
	if _, err := Embed(context.Background(), EmbedConfig{
		DataDir: dataDir,
		App:     app,
		Logger:  log,
		// RequireIdentity stays env-driven; the gateway in front of the
		// cloud binary is the trust boundary per HIP-0026.
		RequireIdentity: false,
	}); err != nil {
		return fmt.Errorf("commerce.Mount: embed: %w", err)
	}

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

	log.Info("commerce mounted",
		"prefix.public", "/v1/commerce",
		"prefix.admin", "/_/commerce",
		"data_dir", dataDir,
	)
	return nil
}
