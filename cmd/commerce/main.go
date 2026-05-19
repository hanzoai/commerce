// Copyright © 2026 Hanzo AI. MIT License.
//
// Legacy entry-point shim. cmd/commerce is the historical binary path
// that Dockerfiles and the Makefile build. The implementation has
// moved to cmd/commerced (mirrors cmd/tasksd), but to avoid a
// flag-day rename across CI/CD this shim re-execs the same package
// graph. Once the Dockerfiles flip to ./cmd/commerced/main.go this
// file can be deleted.

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

	// Mount the full Commerce API routes directly on the live router.
	//
	// Previously this used OnRouteSetup hook binding AFTER Bootstrap had
	// already fired — silent no-op, /v1/billing/* always 404. Fix is to
	// register imperatively on the live *gin.Engine once Embed returns.
	apiGroup := srv.App().Router.Group("/v1")
	api.Route(apiGroup)

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
