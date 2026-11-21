// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/hanzoai/commerce/util/test/ae"
)

// unique is a key no other test, and no earlier run of this one, has used.
func unique(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }

func TestDurableStore(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	var s Durable
	k := unique("durable")
	until := time.Now().Add(time.Minute)
	if ok, err := s.Take(ctx, k, until); !ok || err != nil {
		t.Fatalf("first take: %v %v", ok, err)
	}
	if ok, err := s.Take(ctx, k, until); ok || err != nil {
		t.Fatalf("second take: %v %v", ok, err)
	}
	// A taken key stays taken, whatever the time it was taken until.
	if ok, _ := s.Take(ctx, k, time.Now().Add(-time.Hour)); ok {
		t.Fatal("a taken key was forgotten")
	}
	if err := s.Drop(ctx, k); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.Take(ctx, k, until); !ok {
		t.Fatal("take after drop")
	}
	if err := s.Drop(ctx, unique("never taken")); err != nil {
		t.Fatalf("dropping a key not taken: %v", err)
	}

	// Saved values are separate from taken keys, replaced on a second save, and binary-safe.
	if _, found, err := s.Load(ctx, unique("none")); found || err != nil {
		t.Fatalf("load of nothing: %v %v", found, err)
	}
	v := []byte{0, 1, 2, 0xff, '\n', '"'}
	if err := s.Save(ctx, k, v, until); err != nil {
		t.Fatal(err)
	}
	got, found, err := s.Load(ctx, k)
	if !found || err != nil || !bytes.Equal(got, v) {
		t.Fatalf("load: %v %v %v", got, found, err)
	}
	if err := s.Save(ctx, k, []byte("second"), until); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := s.Load(ctx, k); string(got) != "second" {
		t.Fatalf("a second save did not replace the first: %q", got)
	}
	if ok, _ := s.Take(ctx, k, until); ok {
		t.Fatal("saving a value released a taken key")
	}
	other := unique("saved only")
	_ = s.Save(ctx, other, []byte("x"), until)
	if ok, _ := s.Take(ctx, other, until); !ok {
		t.Fatal("saving a value took the key")
	}
}

// Two replicas serving one resource over one Durable store spend a credential once: the
// second is told it is spent, and settles nothing (sections 11.3 and 11.5).
func TestDurableStoreIsSharedByReplicas(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	replica := func() *rig {
		r := newRig(t, func(c *Config) { c.Store = Durable{} })
		r.ctx = ctx
		return r
	}
	a, b := replica(), replica()
	ch := a.challenge("GET", "/resource", nil, "")[0]
	c := auth(credential(ch, proof).String())
	if resp := a.do("GET", "/resource", c, ""); resp.StatusCode != 200 {
		t.Fatalf("first replica: %d", resp.StatusCode)
	}
	wantProblem(t, b.do("GET", "/resource", c, ""), 402, Invalid)
	if a.invoice.count()+b.invoice.count() != 1 || a.hits.Load()+b.hits.Load() != 1 {
		t.Fatalf("settled %d, delivered %d", a.invoice.count()+b.invoice.count(), a.hits.Load()+b.hits.Load())
	}
	// The idempotent retry of a settled payment is served by whichever replica gets it.
	r1 := newRig(t, func(c *Config) { c.Store, c.Replay = Durable{}, 1<<20 })
	r2 := newRig(t, func(c *Config) { c.Store, c.Replay = Durable{}, 1<<20 })
	r1.ctx, r2.ctx = ctx, ctx
	ch = r1.challenge("POST", "/resource", nil, "")[0]
	h := http1(credential(ch, proof).String(), "k")
	first := r1.do("POST", "/resource", h, "")
	again := r2.do("POST", "/resource", h, "")
	if first.StatusCode != 200 || again.StatusCode != 200 || again.Header.Get("Payment-Receipt") != first.Header.Get("Payment-Receipt") {
		t.Fatalf("%d then %d", first.StatusCode, again.StatusCode)
	}
	if r1.invoice.count()+r2.invoice.count() != 1 || r1.hits.Load()+r2.hits.Load() != 1 {
		t.Fatalf("settled %d, delivered %d", r1.invoice.count()+r2.invoice.count(), r1.hits.Load()+r2.hits.Load())
	}
}

func http1(credential, key string) http.Header {
	return http.Header{"Authorization": {credential}, "Idempotency-Key": {key}}
}
