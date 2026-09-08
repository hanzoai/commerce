// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	commerce "github.com/hanzoai/commerce"
)

// bootLegacy is the historical direct-Gin boot path — the shape the
// production deployment has been running. Embed() boots the gin engine,
// api.Route() wires the full /v1 API surface, net/http serves it.
//
// Kept default through Phase 1 of the cloud-mount migration so a flag
// flip is required to switch over; the same binary serves both shapes.
func bootLegacy(dataDir, httpAddr string, dev, requireIdentity bool) error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv, err := commerce.Embed(ctx, commerce.EmbedConfig{
		DataDir:         dataDir,
		HTTPAddr:        httpAddr,
		Dev:             dev,
		RequireIdentity: requireIdentity,
		Logger:          logger,
	})
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Stop(shutdownCtx)
	}()

	addr := srv.App().Config().HTTPAddr

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http listener", "addr", addr, "version", commerce.Version, "mode", "legacy")
		if err := srv.Zip().Listen("http://" + addr); err != nil {
			errCh <- err
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Zip().ShutdownWithContext(shutdownCtx); err != nil {
		return err
	}
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}
