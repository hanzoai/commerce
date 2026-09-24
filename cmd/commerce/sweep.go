// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	commerce "github.com/hanzoai/commerce"
)

// runSweep is the one thing this binary does that is not serving.
//
// It boots the app the way bootLegacy does — Embed() runs Bootstrap
// synchronously, which is what connects the datastore and registers the MPC
// processor — and then it does NOT listen. A sweep reads intents, asks the
// signer for signatures and exits; the HTTP surface has nothing to do with it.
//
//	commerce sweep <chain> <token> --to <address>
func runSweep(args []string) error {
	fs := flag.NewFlagSet("sweep", flag.ExitOnError)
	to := fs.String("to", "", "address the money is swept to (required)")
	dataDir := fs.String("data", envStr("COMMERCE_DIR", "./commerce_data"), "data directory")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: commerce sweep <chain> <token> --to <address>\n\n"+
			"Moves deposited funds out of every custody address holding <chain>/<token>\n"+
			"and into <address>. The asset must be one the deposit watcher is configured\n"+
			"for (CRYPTO_DEPOSIT_*); nothing else can be swept, because nothing else was\n"+
			"ever handed out as a deposit address.\n\n")
		fs.PrintDefaults()
	}
	// The chain and the token are read off the front before the flags, because
	// stdlib flag stops parsing at the first non-flag argument — so the natural
	// `sweep base usdc --to 0x…` would otherwise leave --to unset and sweep to
	// nowhere. Taking the two positionals first makes both orders work.
	if len(args) < 2 || strings.HasPrefix(args[0], "-") || strings.HasPrefix(args[1], "-") {
		fs.Usage()
		return fmt.Errorf("sweep needs a chain and a token, e.g. `commerce sweep base usdc --to 0x…`")
	}
	chain, token := args[0], args[1]
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}
	// Refused before anything boots: a sweep with no destination has nothing to
	// do, and opening the datastore to find that out helps nobody.
	if *to == "" {
		fs.Usage()
		return fmt.Errorf("sweep needs --to: the address the money is going to")
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv, err := commerce.Embed(ctx, commerce.EmbedConfig{DataDir: *dataDir, Logger: slog.Default()})
	if err != nil {
		return err
	}
	defer func() { _ = srv.Stop(context.Background()) }()

	return commerce.Sweep(ctx, chain, token, *to)
}
