// Copyright © 2026 Hanzo AI. MIT License.

package org

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/datastore/key"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/test/ae"
)

// legacyNumericID is a real-world artifact: a pre-hashid GAE datastore numeric
// id that the IAM Valkey cache (iam:org_by_name:hanzo) can still hold for the
// "hanzo" org. Resolving it must never loop or peg CPU — a regression here hangs
// EVERY completion (SEV1 2026-07-18: api.hanzo.ai default model down).
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

// TestIsLegacyNumericID pins the guard predicate: only an all-digit string is a
// legacy id; a real commerce hashid (letters + digits) must NOT be misrouted to
// name-based resolution.
func TestIsLegacyNumericID(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"1772587477", true}, // the SEV1 legacy id
		{"42", true},
		{"", false},             // no cached id
		{"BPuzGP7v8SY", false},  // a real hashid org id (has letters)
		{"o2Qt6nlXJVHZ", false}, // another real hashid
		{"123abc", false},       // mixed
		{"hanzo", false},        // a name
		{" 123", false},         // whitespace is not a digit
	}
	for _, c := range cases {
		if got := isLegacyNumericID(c.id); got != c.want {
			t.Errorf("isLegacyNumericID(%q) = %v, want %v", c.id, got, c.want)
		}
	}
}

// TestDecode_LegacyNumericID_Graceful pins the primitive neo's post-mortem
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

// fakeKV mimics the IAM Valkey cache: it hands back the STALE legacy numeric id
// for iam:org_by_name:hanzo, reproducing the exact production trigger.
type fakeKV struct {
	m    map[string]string
	gets int
}

func (f *fakeKV) Get(_ context.Context, k string) (string, error) {
	f.gets++
	if v, ok := f.m[k]; ok {
		return v, nil
	}
	return "", errors.New("miss")
}
func (f *fakeKV) Set(_ context.Context, k, v string, _ time.Duration) error {
	f.m[k] = v
	return nil
}
func (f *fakeKV) Delete(_ context.Context, keys ...string) error {
	for _, k := range keys {
		delete(f.m, k)
	}
	return nil
}

// TestResolve_CachedLegacyNumericID_NoHang is the full production path: the KV
// cache holds the legacy numeric id for "hanzo"; Resolve must skip the doomed
// GetById, degrade to GetOrCreate("Name=") and return a real "hanzo" org — never
// hang — AND self-heal the cache by re-writing a proper (non-numeric) hashid id.
// This is the exact call the IAM middleware makes on EVERY completion.
func TestResolve_CachedLegacyNumericID_NoHang(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	kv := &fakeKV{m: map[string]string{cacheKey("hanzo"): legacyNumericID}}
	Bind(kv)
	defer Bind(nil)

	var got *organization.Organization
	var gotErr error
	within(t, 5*time.Second, "Resolve(hanzo) with cached legacy id", func() {
		got, gotErr = Resolve(context.Background(), "hanzo")
	})
	if gotErr != nil {
		t.Fatalf("Resolve(hanzo) errored: %v", gotErr)
	}
	if got == nil || got.Name != "hanzo" {
		t.Fatalf("Resolve(hanzo) = %+v, want org named hanzo", got)
	}
	// Self-heal: the stale numeric entry must be replaced with a real hashid id
	// so subsequent requests take the fast by-id path.
	healed := kv.m[cacheKey("hanzo")]
	if healed == legacyNumericID {
		t.Fatalf("cache not self-healed: still %q", healed)
	}
	if isLegacyNumericID(healed) {
		t.Fatalf("cache re-cached another legacy numeric id %q, want a hashid", healed)
	}
}
