// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.
//
// commerce is the canonical entry-point. Two boot modes — chosen at
// startup, same binary:
//
//	legacy (default):  flag --cloud absent, env COMMERCE_MODE unset/!=cloud
//	                   → bootLegacy(): direct-Gin behind net/http listener,
//	                     wires api.Route(/v1) imperatively. The shape the
//	                     production deployment runs today.
//
//	cloud:             flag --cloud OR env COMMERCE_MODE=cloud
//	                   → bootCloud(): zip.App with commerce mounted via
//	                     the host app directly — commerce is native zip, so
//	                     inner handler, no public route surface of its own.
//
// Default stays legacy until cloud-mount is validated in production.
// Phase 1 of the staged Gin → zip migration.

package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	// Register the EVM ERC-20 transfer implementation into util/blockchain so
	// the contributor payout path can execute on-chain HUSD payouts
	// (PayoutMethod="crypto"). Without this blank import,
	// blockchain.TransferToken returns ErrNoTokenTransfer and crypto payouts
	// are skipped. This is the single wiring point that links luxfi/geth into
	// the production commerce binary.
)

func main() {
	// One subcommand, and it is the only thing this binary does that is not
	// serving: `commerce sweep <chain> <token> --to <addr>` moves money out of
	// custody and exits. It is read before flag.Parse because it takes its own
	// flags — a sweep has nothing to say about listen addresses.
	//
	// There is a cobra tree in the library (App.RootCmd, serve/admin/seed) and
	// nothing executes it; putting the sweep there would have been a command no
	// operator could run, which is the shape of the defect it exists to end.
	if len(os.Args) > 1 && os.Args[1] == "sweep" {
		if err := runSweep(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "commerce: sweep: %v\n", err)
			os.Exit(1)
		}
		return
	}

	var (
		dataDir         = flag.String("data", envStr("COMMERCE_DIR", "./commerce_data"), "data directory")
		httpAddr        = flag.String("http", envStr("COMMERCE_HTTP", "127.0.0.1:8090"), "HTTP listen address")
		dev             = flag.Bool("dev", envBool("COMMERCE_DEV", false), "enable development mode")
		requireIdentity = flag.Bool("require-identity", envBool("COMMERCED_REQUIRE_IDENTITY", false), "refuse requests without X-Org-Id/X-User-Id (gateway trust)")
		cloudMode       = flag.Bool("cloud", envCloudMode(), "boot via cloud-mount: commerce registered on a zip.App the way a host mounts it")
	)
	flag.Parse()

	if *cloudMode {
		if err := bootCloud(*dataDir, *httpAddr, *dev, *requireIdentity); err != nil {
			fmt.Fprintf(os.Stderr, "commerce: cloud boot: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := bootLegacy(*dataDir, *httpAddr, *dev, *requireIdentity); err != nil {
		fmt.Fprintf(os.Stderr, "commerce: legacy boot: %v\n", err)
		os.Exit(1)
	}
}

// envCloudMode is true iff COMMERCE_MODE=cloud (case-insensitive trim
// not needed — env values are exact in production). Used as the default
// for --cloud so operators can flip modes via env without changing argv.
func envCloudMode() bool {
	return os.Getenv("COMMERCE_MODE") == "cloud"
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
