// Copyright (c) 2026-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.
//
// Package organization — Namespaced context doctrine regression.
//
// Organization.Namespaced detaches the datastore context from the HTTP request
// lifecycle so a browser disconnect / upstream proxy timeout can't cancel an
// in-flight money write. The detach MUST use context.WithoutCancel, not
// context.Background(): the mintauth gate/grant and namespace ride as CONTEXT
// VALUES, and Background() would discard them — silently un-authorizing a
// legitimate mint (the v1.46.42 "service-token org has a canceled ctx" bug
// class). These tests pin: (1) cancellation is dropped, (2) namespace is
// preserved, (3) the mint gate is held, and (4) a grant applied upstream
// survives the detach so mintauth.Enforce stays fail-closed for the
// unauthorized case and allows the authorized case.
package organization

import (
	"context"
	"testing"

	"github.com/hanzoai/commerce/mintauth"
	"github.com/hanzoai/commerce/util/nscontext"
)

// mintEntity is a Guarded entity that always mints spendable balance, so
// mintauth.Enforce exercises the full gate/grant decision.
type mintEntity struct{}

func (mintEntity) MintRequiresAuthorization() bool { return true }

// canceledGatedCtx builds a mint-gated request context whose lifecycle is
// already canceled (simulating a client disconnect). This is the HTTP-boundary
// context the request middleware produces: RequestContext stamps WithGate; if
// authorized, PlatformOnly wraps it WithAuthorized.
func canceledGatedCtx(authorized bool) context.Context {
	reqCtx, cancel := context.WithCancel(context.Background())
	gated := mintauth.WithGate(reqCtx)
	if authorized {
		gated = mintauth.WithAuthorized(gated)
	}
	cancel() // client disconnects mid-request
	return gated
}

// TestNamespaced_SurvivesDisconnect_Gated proves the plain inbound request:
// after detach the context is not canceled, is namespace-scoped, is mint-gated,
// and — carrying NO grant — Enforce fails closed on a spendable mint.
func TestNamespaced_SurvivesDisconnect_Gated(t *testing.T) {
	org := Organization{Name: "acme"}
	ns := org.Namespaced(canceledGatedCtx(false))

	if err := ns.Err(); err != nil {
		t.Fatalf("Namespaced context is canceled (%v); WithoutCancel must drop cancellation so the DB write survives disconnect", err)
	}
	if got := nscontext.GetNamespace(ns); got != "acme" {
		t.Fatalf("namespace = %q, want \"acme\"", got)
	}
	if !mintauth.Gated(ns) {
		t.Fatal("inbound request context must be mint-gated")
	}
	if err := mintauth.Enforce(ns, mintEntity{}); err != mintauth.ErrNotAuthorized {
		t.Fatalf("Enforce on gated+unauthorized mint = %v, want ErrNotAuthorized (fail-closed)", err)
	}
}

// TestNamespaced_SurvivesDisconnect_PreservesGrant is the money-critical case:
// a grant applied upstream (PlatformOnly / settled payment) must ride through
// the detach so the authorized mint is ALLOWED even though the underlying HTTP
// request context was canceled. context.Background() would drop the grant here
// and wrongly refuse the write.
func TestNamespaced_SurvivesDisconnect_PreservesGrant(t *testing.T) {
	org := Organization{Name: "acme"}
	ns := org.Namespaced(canceledGatedCtx(true))

	if err := ns.Err(); err != nil {
		t.Fatalf("Namespaced context is canceled (%v); must survive disconnect", err)
	}
	if !mintauth.Gated(ns) {
		t.Fatal("context must still be mint-gated")
	}
	if !mintauth.Authorized(ns) {
		t.Fatal("mint grant applied upstream was LOST through the detach; WithoutCancel must preserve context values (Background would drop the grant)")
	}
	if err := mintauth.Enforce(ns, mintEntity{}); err != nil {
		t.Fatalf("Enforce on gated+authorized mint = %v, want nil (allowed)", err)
	}
}

// TestNamespaced_InternalCallerUngated proves an ungated caller (cron,
// migration, test) is namespace-scoped but NOT gated — internal ledger writes
// mint freely, unchanged by the HTTP-boundary gate.
func TestNamespaced_InternalCallerUngated(t *testing.T) {
	org := Organization{Name: "acme"}
	ns := org.Namespaced(context.Background())

	if got := nscontext.GetNamespace(ns); got != "acme" {
		t.Fatalf("namespace = %q, want \"acme\"", got)
	}
	if mintauth.Gated(ns) {
		t.Fatal("internal (ungated) caller must NOT be gated")
	}
	if err := mintauth.Enforce(ns, mintEntity{}); err != nil {
		t.Fatalf("Enforce on ungated mint = %v, want nil (internal authority mints freely)", err)
	}
}
