// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryTake(t *testing.T) {
	ctx := context.Background()
	c := &clock{at: epoch}
	m := &Memory{Now: c.now}
	until := epoch.Add(time.Minute)
	if ok, err := m.Take(ctx, "k", until); !ok || err != nil {
		t.Fatalf("first take: %v %v", ok, err)
	}
	if ok, _ := m.Take(ctx, "k", until); ok {
		t.Fatal("second take succeeded")
	}
	if ok, _ := m.Take(ctx, "other", until); !ok {
		t.Fatal("another key")
	}
	_ = m.Drop(ctx, "k")
	if ok, _ := m.Take(ctx, "k", until); !ok {
		t.Fatal("take after drop")
	}
	_ = m.Drop(ctx, "never taken")
	// A taken key is forgotten at its time.
	c.advance(time.Minute)
	if ok, _ := m.Take(ctx, "k", c.now().Add(time.Minute)); !ok {
		t.Fatal("take after expiry")
	}
	c.advance(10 * time.Second)
	m.mu.Lock()
	if len(m.taken) > 2 {
		t.Errorf("%d keys kept", len(m.taken))
	}
	m.mu.Unlock()
}

func TestMemorySave(t *testing.T) {
	ctx := context.Background()
	c := &clock{at: epoch}
	m := &Memory{Now: c.now}
	if _, found, _ := m.Load(ctx, "k"); found {
		t.Fatal("found nothing")
	}
	v := []byte("value")
	_ = m.Save(ctx, "k", v, epoch.Add(time.Minute))
	v[0] = 'X' // the store keeps its own copy
	got, found, err := m.Load(ctx, "k")
	if !found || err != nil || string(got) != "value" {
		t.Fatalf("%q %v %v", got, found, err)
	}
	got[0] = 'Y'
	if again, _, _ := m.Load(ctx, "k"); string(again) != "value" {
		t.Fatal("a loaded value aliases the store")
	}
	c.advance(time.Minute)
	if _, found, _ := m.Load(ctx, "k"); found {
		t.Fatal("a saved value outlived its time")
	}
}

// Take is atomic: of many callers at once exactly one holds the key.
func TestMemoryTakeIsAtomic(t *testing.T) {
	m := &Memory{}
	var wg sync.WaitGroup
	var won atomic.Int32
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _ := m.Take(context.Background(), "k", time.Now().Add(time.Minute)); ok {
				won.Add(1)
			}
		}()
	}
	wg.Wait()
	if won.Load() != 1 {
		t.Fatalf("%d callers took the key", won.Load())
	}
}
