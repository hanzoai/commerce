// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import (
	"context"
	"testing"
	"time"
)

// The equality of rfc.agentic_checkout.md §6.2.
func TestFingerprintFollowsTheSpecsEquality(t *testing.T) {
	same := [][2]string{
		{`{"a":1,"b":2}`, `{"b":2,"a":1}`},
		{`{"a":{"x":1,"y":[1,2]}}`, `{"a":{"y":[1,2],"x":1}}`},
		{`{"n":1}`, `{"n":1.0}`},
		{`{"n":1}`, `{"n":1.000}`},
		{`{"n":0.1}`, `{"n":1e-1}`},
		{`{"n":10}`, `{"n":1e1}`},
		{`{"n":100}`, `{"n":1.0E2}`},
		{`{"n":-2.50}`, `{"n":-2.5}`},
		{`{"n":0}`, `{"n":0.0}`},
		{"{ \"a\" : [ 1 , 2 ] }", `{"a":[1,2]}`},
		{`{"s":"A"}`, `{"s":"A"}`},
	}
	for _, p := range same {
		a, err1 := Fingerprint([]byte(p[0]))
		b, err2 := Fingerprint([]byte(p[1]))
		if err1 != nil || err2 != nil || a != b {
			t.Errorf("%s and %s should be equal (%v %v)", p[0], p[1], err1, err2)
		}
	}
	different := [][2]string{
		{`{"a":[1,2]}`, `{"a":[2,1]}`},
		{`{"a":null}`, `{}`},
		{`{"a":null}`, `{"a":false}`},
		{`{"a":1}`, `{"a":2}`},
		{`{"a":1}`, `{"a":"1"}`},
		{`{"a":0.1}`, `{"a":0.10001}`},
		{`{"a":-1}`, `{"a":1}`},
		{`{"a":"x"}`, `{"a":"X"}`},
		{`{"a":1}`, `{"a":1,"b":null}`},
		{`{"a":[]}`, `{"a":{}}`},
		{``, `{}`},
		{`{"n":1e999999999}`, `{"n":1e999999998}`},
	}
	for _, p := range different {
		a, err1 := Fingerprint([]byte(p[0]))
		b, err2 := Fingerprint([]byte(p[1]))
		if err1 != nil || err2 != nil || a == b {
			t.Errorf("%s and %s should differ (%v %v)", p[0], p[1], err1, err2)
		}
	}
	for _, bad := range []string{`{`, `nope`, `{} {}`, `{"a":1}x`} {
		if _, err := Fingerprint([]byte(bad)); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

func TestNumberNormalisation(t *testing.T) {
	for in, want := range map[string]string{
		"1": "1e0", "1.0": "1e0", "10": "1e1", "0.1": "1e-1", "0.05": "5e-2", "100": "1e2", "1e2": "1e2",
		"-12.50": "-125e-1", "0": "0", "0.000": "0", "1E+3": "1e3", "1e999999999": "1e999999999", "-1e999999999": "-1e999999999",
	} {
		if got := number(in); got != want {
			t.Errorf("number(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestMemoryLedger(t *testing.T) {
	ctx := context.Background()
	clock := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	m := NewMemory()
	m.now = func() time.Time { return clock }

	v, _, lease, err := m.Begin(ctx, "s", "k", "p1")
	if err != nil || v != Fresh || lease == nil {
		t.Fatalf("first = %v %v", v, err)
	}
	// While it runs, the same key is busy whatever the body.
	for _, p := range []string{"p1", "p2"} {
		if v, _, _, _ := m.Begin(ctx, "s", "k", p); v != Busy {
			t.Fatalf("running, %s: %v", p, v)
		}
	}
	if err := lease.Settle(ctx, Stored{Status: 201, Body: []byte(`{"id":"x"}`)}); err != nil {
		t.Fatal(err)
	}
	v, got, _, _ := m.Begin(ctx, "s", "k", "p1")
	if v != Replay || got.Status != 201 || string(got.Body) != `{"id":"x"}` {
		t.Fatalf("settled, same body: %v %+v", v, got)
	}
	if v, _, _, _ := m.Begin(ctx, "s", "k", "p2"); v != Mismatch {
		t.Fatalf("settled, other body: %v", v)
	}
	// Another scope or key is another entry.
	for _, c := range [][2]string{{"s2", "k"}, {"s", "k2"}} {
		if v, _, _, _ := m.Begin(ctx, c[0], c[1], "p1"); v != Fresh {
			t.Fatalf("%v: %v", c, v)
		}
	}
	// Release frees a key.
	_, _, l2, _ := m.Begin(ctx, "s", "r", "p")
	if err := l2.Release(ctx); err != nil {
		t.Fatal(err)
	}
	if v, _, _, _ := m.Begin(ctx, "s", "r", "other"); v != Fresh {
		t.Fatalf("released: %v", v)
	}
	// Entries are kept for at least 24 hours and forgotten after.
	clock = clock.Add(Retention)
	if v, _, _, _ := m.Begin(ctx, "s", "k", "p1"); v != Replay {
		t.Fatalf("at 24h: %v", v)
	}
	clock = clock.Add(time.Second)
	if v, _, _, _ := m.Begin(ctx, "s", "k", "p2"); v != Fresh {
		t.Fatalf("after 24h: %v", v)
	}
	// Expired entries are swept as time passes.
	clock = clock.Add(2 * time.Minute)
	m.Begin(ctx, "s", "swept", "p")
	if n := len(m.entries); n != 2 {
		t.Fatalf("%d entries kept; want the live key and the new one", n)
	}
}

func TestReleasingALapsedLeaseDoesNotFreeANewerClaim(t *testing.T) {
	ctx := context.Background()
	clock := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	m := NewMemory()
	m.now = func() time.Time { return clock }
	_, _, old, _ := m.Begin(ctx, "s", "k", "p")
	clock = clock.Add(Retention + time.Second)
	v, _, fresh, _ := m.Begin(ctx, "s", "k", "p")
	if v != Fresh {
		t.Fatal(v)
	}
	_ = old.Release(ctx)
	if v, _, _, _ := m.Begin(ctx, "s", "k", "p"); v != Busy {
		t.Fatalf("a release of an expired claim freed the new one: %v", v)
	}
	_ = fresh.Release(ctx)
}
