// Copyright © 2026 Hanzo AI. MIT License.

package org

import (
	"context"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/datastore/key"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The 2026-07-18 SEV1 (api.hanzo.ai default model down) came from resolving an
// org through a by-ID lookup seeded by IAM's shared Valkey cache: that cache
// still held a pre-hashid GAE numeric id for "hanzo", which could never match
// the org's real key, so every lookup missed and — stacked on the pooled-
// connection leak — blocked to the 10s deadline.
//
// Resolve no longer reads any external id cache and never issues a by-id
// lookup: it resolves by NAME only, memoized in-process. The failure mode is
// therefore structurally absent rather than guarded, so the guard predicate
// (isLegacyNumericID) and its fake-KV tests are gone with the path they
// protected.
//
// What remains here are the two PRIMITIVE regressions the post-mortem fingered.
// They hold regardless of how org resolution is layered above them: a legacy
// numeric id must decode and look up gracefully, never hot-loop.

// legacyNumericID is a real-world artifact: a pre-hashid GAE datastore numeric
// id that IAM's cache still held for the "hanzo" org.
const legacyNumericID = "1772587477"

// within fails the test if fn does not return inside d. A hot-loop / hang (the
// SEV1 failure mode) trips this instead of wedging the whole test binary.
func within(t *testing.T, d time.Duration, name string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); fn() }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s did not return within %s — hot-loop / hang regression", name, d)
	}
}

// TestDecode_LegacyNumericID_Graceful pins the primitive the post-mortem
// fingered: key.Decode of a non-hashid numeric id must fall back to a numeric
// key with NO error and NO loop (it logs "Failed to decode hashid" once at DEBUG
// then ParseInt-succeeds — it must not retry).
func TestDecode_LegacyNumericID_Graceful(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	within(t, 5*time.Second, "key.Decode(legacyNumericID)", func() {
		k, err := key.Decode(context.Background(), legacyNumericID)
		if err != nil {
			t.Errorf("key.Decode(%q) returned error, want graceful fallback: %v", legacyNumericID, err)
			return
		}
		if k == nil {
			t.Errorf("key.Decode(%q) returned nil key", legacyNumericID)
			return
		}
		if k.IntID() != 1772587477 {
			t.Errorf("key.Decode(%q) IntID = %d, want 1772587477", legacyNumericID, k.IntID())
		}
	})
}

// TestById_LegacyNumericID_Graceful is the literal STEP-1 guard: (*Query).ById
// with a legacy non-hashid numeric id for the "organization" kind must return
// (graceful not-found) without looping or erroring-fatally.
func TestById_LegacyNumericID_Graceful(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	q := datastore.New(ctx).Query("organization")
	var o organization.Organization
	within(t, 5*time.Second, "Query.ById(legacyNumericID)", func() {
		// A failed hashid decode must degrade to a bounded lookup, never a loop.
		// found=false (empty store) is the expected, healthy outcome.
		if _, found, err := q.ById(legacyNumericID, &o); found {
			t.Errorf("ById(%q) unexpectedly found an org in an empty store", legacyNumericID)
			_ = err
		}
	})
}
