// Copyright (c) 2026-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

//go:build !cloud
// +build !cloud

package main

import "fmt"

// bootCloud is the stub returned when the binary is built WITHOUT the
// `cloud` build tag. The real implementation lives in cloud_boot.go,
// gated by //go:build cloud, and imports github.com/hanzoai/cloud +
// zap-proto/zip. Building without the tag keeps the production binary
// free of those imports and their transitive deps when only the
// legacy direct-Gin path is needed.
func bootCloud(dataDir, httpAddr string, dev, requireIdentity bool) error {
	_ = dataDir
	_ = httpAddr
	_ = dev
	_ = requireIdentity
	return fmt.Errorf("--cloud requested but binary built without `-tags cloud`; rebuild with `go build -tags cloud ./cmd/commerce`")
}
