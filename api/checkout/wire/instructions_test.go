// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package wire

import (
	"net/http"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/organization"
)

// TestInstructions_NoOrgFailsClosed proves the RED finding is fixed: an org-less
// request (e.g. an anonymous caller after EdgeAuth strips a spoofed X-Org-Id)
// returns a clean 401 instead of panicking on GetOrganization's MustGet (500).
func TestInstructions_NoOrgFailsClosed(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	c := app.TestCtx(http.MethodGet, "/v1/checkout/wire/instructions")

	// Must not panic.
	_ = Instructions(c)

	if code := c.Fiber().Response().StatusCode(); code != http.StatusUnauthorized {
		t.Fatalf("org-less wire instructions: status=%d, want 401 (fail closed, no panic)", code)
	}
}

// TestInstructions_ConfiguredOrgReturns200 proves a resolved org with wire config
// still gets its instructions (no regression from the fail-closed fix).
func TestInstructions_ConfiguredOrgReturns200(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	c := app.TestCtx(http.MethodGet, "/v1/checkout/wire/instructions")

	org := &organization.Organization{}
	org.Wire.BankName = "Test Bank"
	org.Wire.AccountHolder = "Hanzo AI"
	c.Locals("organization", org)

	_ = Instructions(c)

	if code := c.Fiber().Response().StatusCode(); code != http.StatusOK {
		t.Fatalf("configured-org wire instructions: status=%d, want 200 (no regression)", code)
	}
}
