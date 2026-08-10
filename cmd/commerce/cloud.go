// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	commerce "github.com/hanzoai/commerce"
	luxlog "github.com/luxfi/log"
	"github.com/zap-proto/zip"
	"github.com/zap-proto/zip/middleware"
)

// bootCloud serves commerce as a subsystem inside a zip.App per the
// HIP-0106 unified Hanzo Cloud binary contract. commerce.Mount registers
// commerce's handlers natively on that app under /v1/commerce + /_/commerce;
// zip owns the listener.
//
// Same artifact runs every white-label deployment — only the gateway and
// brand differ.
func bootCloud(dataDir, httpAddr string, dev, requireIdentity bool) error {
	logger := luxlog.New("commerce")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Build the zip.App with the canonical middleware pipeline. Match
	// cmd/cloud/main.go ordering so the cloud-mounted commerce and the
	// in-cloud-binary commerce behave identically at the boundary:
	//   Recover → RequestID → Logger. Auth/Telemetry are gateway-owned
	//   per the HIP-0026 trust boundary.
	app := zip.New(zip.Config{Logger: logger})
	app.Use(middleware.Recover())
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger(logger))

	// commerce.Mount boots the embedded server via commerce.Embed, which
	// honors COMMERCE_DIR/HTTPAddr/etc. via DefaultConfig when dataDir is
	// empty. Set the env so the embedded server reads the same shape the
	// legacy boot does; the listener inside Embed stays unused because the
	// zip.App owns the listener.
	_ = os.Setenv("COMMERCE_DIR", dataDir)
	_ = os.Setenv("COMMERCED_REQUIRE_IDENTITY", boolStr(requireIdentity))
	if dev {
		_ = os.Setenv("COMMERCE_DEV", "true")
	}

	if err := commerce.Mount(app, dataDir, logger); err != nil {
		return err
	}

	listenErr := make(chan error, 1)
	go func() {
		logger.Info("http listener",
			"addr", httpAddr,
			"version", commerce.Version,
			"mode", "cloud",
		)
		if err := app.Listen(httpAddr); err != nil {
			listenErr <- err
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		return err
	}
	select {
	case err := <-listenErr:
		return err
	default:
		return nil
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
