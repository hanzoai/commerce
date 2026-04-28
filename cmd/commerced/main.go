// Copyright © 2026 Hanzo AI. MIT License.
//
// commerced is the Hanzo Commerce daemon: one Go binary, gateway-trust
// identity (no in-binary JWKS), embedded admin SPA at /_/commerce/.
// Mirrors the cmd/tasksd / cmd/iamd shape — thin entrypoint, all
// surface area in pkg/commerce.

package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	commerceApp "github.com/hanzoai/commerce"
	api "github.com/hanzoai/commerce/api/api"
	"github.com/hanzoai/commerce/hooks"
	commerce "github.com/hanzoai/commerce/pkg/commerce"
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

	// Register the full Commerce API routes on the /v1/commerce group.
	// hooks.OnRouteSetup fires inside Bootstrap; since Embed has already
	// run Bootstrap by the time we get here, we register on the live
	// router via the hook re-trigger pathway below.
	srv.App().Hooks.OnRouteSetup().Bind(&hooks.Handler[*hooks.RouteEvent]{
		ID:       "commerce-api",
		Priority: 0,
		Func: func(e *hooks.RouteEvent) error {
			api.Route(e.Router)
			return nil
		},
	})

	httpSrv := &http.Server{
		Addr:              srv.HTTPAddr(),
		Handler:           srv.HTTPHandler(),
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		logger.Info("http listener", "addr", httpSrv.Addr, "version", commerceApp.Version)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
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

