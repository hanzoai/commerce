// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestMemoryStoreNeedsItsLimits(t *testing.T) {
	if _, err := NewMemory(0, 0); err == nil {
		t.Error("a store with no cap on its entries was accepted")
	}
	if _, err := NewMemory(time.Hour, 10); err == nil {
		t.Error("a retention under the 24 hours the specification requires was accepted")
	}
	if _, err := NewMemory(0, 10); err != nil {
		t.Fatal(err)
	}
}

// Begin is an index lookup, not a scan: one lock is shared by every tenant.
func TestMemoryBeginDoesNotScan(t *testing.T) {
	s, err := NewMemory(0, 60000)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for i := 0; i < 50000; i++ {
		if _, err := s.Begin(ctx, "scope", fmt.Sprintf("k%d", i), "d"); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	for i := 0; i < 2000; i++ {
		if _, err := s.Begin(ctx, "scope", fmt.Sprintf("n%d", i), "d"); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("2,000 Begins over 50,000 keys took %s: Begin scans the store", d)
	}
}

func TestMemoryStoreExpiresAndCapsWithoutEvictingLiveKeys(t *testing.T) {
	s, err := NewMemory(0, 3)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	s.now = func() time.Time { return now }
	ctx := context.Background()
	for _, k := range []string{"a", "b", "c"} {
		if seen, err := s.Begin(ctx, "s", k, "d"); seen != nil || err != nil {
			t.Fatalf("%s: %v %v", k, seen, err)
		}
		now = now.Add(time.Minute)
	}
	// Full: a new key is refused, it does not evict a live one.
	if _, err := s.Begin(ctx, "s", "d", "d"); !errors.Is(err, ErrFull) {
		t.Fatalf("a full store: %v", err)
	}
	for _, k := range []string{"a", "b", "c"} {
		if seen, err := s.Begin(ctx, "s", k, "d"); seen == nil || err != nil {
			t.Fatalf("live key %s was lost: %v %v", k, seen, err)
		}
	}
	// Once the oldest expires the store makes room for it, and drops it.
	now = now.Add(Keep - 150*time.Second)
	if seen, err := s.Begin(ctx, "s", "d", "d"); seen != nil || err != nil {
		t.Fatalf("after the oldest key expired: %v %v", seen, err)
	}
	if seen, _ := s.Begin(ctx, "s", "a", "d"); seen != nil {
		t.Fatal("an expired key was still remembered")
	}
	// Everything expires: the store empties.
	now = now.Add(2 * Keep)
	s.Begin(ctx, "s", "z", "d")
	s.mu.Lock()
	n := len(s.m)
	s.mu.Unlock()
	if n != 1 {
		t.Fatalf("%d records after everything expired", n)
	}
}

func TestServerFailsClosedWhenTheIdempotencyStoreIsFull(t *testing.T) {
	mem, err := NewMemory(0, 1)
	if err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, shopPay(), func(c *Config) { c.Store = mem })
	h.backend.next = fixtureCheckout(t, "shopping-checkout-rest-115.json")
	create := call{method: "POST", path: "/checkout-sessions", body: fixture(t, "shopping-checkout-rest-95.json")}
	if w := h.do(create); w.Code != 201 {
		t.Fatalf("first: %d", w.Code)
	}
	create.header = map[string]string{"Idempotency-Key": "6f1c1d5e-0f0a-4c55-9b61-9a1f1f9c9d10"}
	w := h.do(create)
	if w.Code != http.StatusServiceUnavailable || w.Header().Get("Retry-After") == "" || h.backend.count("create") != 1 {
		t.Fatalf("a full store: %d, Retry-After %q, %d creates", w.Code, w.Header().Get("Retry-After"), h.backend.count("create"))
	}
}

func TestServerRequiresAnIdempotencyStore(t *testing.T) {
	prof, _ := NewProfile(ProfileConfig{Endpoint: "https://merchant.example/ucp/v1"})
	_, err := New(Config{Profile: prof, Backend: &fakeBackend{}, Profiles: &peers{},
		Identify: func(*http.Request) (Caller, error) { return Caller{}, nil }})
	if err == nil {
		t.Fatal("a server with no idempotency store was accepted; its size is the host's to choose")
	}
}
