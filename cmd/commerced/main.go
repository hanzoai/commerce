// Copyright © 2026 Hanzo AI. MIT License.
//
// commerced is the Hanzo Commerce daemon: one Go binary, gateway-trust
// identity (no in-binary JWKS), embedded admin SPA at /admin.
// Mirrors the cmd/tasksd / cmd/iamd shape — thin entrypoint, all
// surface area in pkg/commerce.

package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	commerce "github.com/hanzoai/commerce"
	commerceApp "github.com/hanzoai/commerce"
	api "github.com/hanzoai/commerce/api"
	billingapi "github.com/hanzoai/commerce/api/billing"

	"github.com/zap-proto/zip"
)

func main() {
	var (
		dataDir         = flag.String("data", envStr("COMMERCE_DIR", "./commerce_data"), "data directory")
		httpAddr        = flag.String("http", envStr("COMMERCE_HTTP", "127.0.0.1:8090"), "HTTP listen address")
		dev             = flag.Bool("dev", envBool("COMMERCE_DEV", false), "enable development mode")
		requireIdentity = flag.Bool("require-identity", envBool("COMMERCED_REQUIRE_IDENTITY", false), "refuse requests without X-Org-Id/X-User-Id (gateway trust)")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdown := initTelemetry(ctx, "hanzo-commerce")
	defer shutdown(context.Background())

	srv, err := commerce.Embed(ctx, commerce.EmbedConfig{
		DataDir:         *dataDir,
		HTTPAddr:        *httpAddr,
		Dev:             *dev,
		RequireIdentity: *requireIdentity,
		Logger:          logger,
	})
	if err != nil {
		logger.Error("commerce.Embed", "err", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Stop(shutdownCtx)
	}()

	// Register the full Commerce API routes (billing, checkout, store, …)
	// directly on the live router.
	//
	// Previously this attempted to bind OnRouteSetup AFTER Bootstrap had
	// already fired — which silently no-ops, so /v1/billing/* returned 404
	// for the entire life of commerced. The fix is to mount routes
	// imperatively on the live zip app the moment Embed returns.
	//
	// api.Route() takes a zip.Router. Group at "/v1/" so handlers land
	// under /v1/billing/*, /v1/checkout/*, etc. matching the global
	// "/v1/ only" rule and the Prefixes["api"]="/v1/" config.
	apiGroup := srv.App().Router.Group("/v1")
	api.Route(apiGroup)
	// The money plane's risk face names its whole path on the root — see
	// api/billing/risk.go.
	billingapi.RiskRoute(srv.App().Router)

	// ONE address, resolved by the zip Plugin contract. When a host started this
	// binary as a plugin it passes a unix socket in ZIP_ADDR and zip.Addr returns
	// it, so Listen serves ZAP over that socket; started directly, zip.Addr falls
	// back and Listen serves plain HTTP exactly as before. Same binary, same line,
	// both shapes — which is the point of the contract: a plugin and a linked-in
	// service compose identically.
	addr := zip.Addr("http://" + srv.App().Config().HTTPAddr)

	go func() {
		logger.Info("listener", "addr", addr, "version", commerceApp.Version)
		if err := srv.Zip().Listen(addr); err != nil {
			logger.Error("http", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = srv.Zip().ShutdownWithContext(shutdownCtx)
}

func envStr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envBool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
