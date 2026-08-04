// Copyright (c) 2026-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package billing

import (
	"testing"

	"github.com/zap-proto/zip"
)

// TestResolveWebhookOrgSessionlessDoesNotPanic is the regression guard for the
// webhook-ingress 500: resolveWebhookOrg ran middleware.GetOrganization —
// gin MustGet — on requests that never passed the auth-token group, so every
// signature-VALID provider delivery panicked ("key organization does not
// exist") after passing HMAC verification. Sessionless resolution must fall
// through to header/env/default lookup instead.
func TestResolveWebhookOrgSessionlessDoesNotPanic(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	c := app.TestCtx("POST", "/v1/billing/webhooks/square")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("resolveWebhookOrg panicked on a sessionless request: %v", r)
		}
	}()
	// No "organization" key set — the sessionless webhook shape. The resolver
	// may return nil here (no datastore in a bare test context); the contract
	// under test is strictly "no panic", which the handler turns into an
	// honest 503 rather than a recovered-panic 500.
	_ = resolveWebhookOrg(c)
}
