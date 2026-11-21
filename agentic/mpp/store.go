// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"context"
	"sync"
	"time"
)

// Store is the state a Gate keeps so that a credential is spent once (sections 11.3 and
// 11.5): which challenge ids and payment proofs are taken, and what a settled payment
// answered. Every call must be atomic with respect to the key it names, and the same
// state must be visible to every process serving the resource, or a credential can be
// spent once per process. Memory is a Store for tests and a single process; a durable,
// shared one is the host's.
type Store interface {
	// Take marks key taken until the instant given. It reports false, and changes
	// nothing, when key is already taken.
	Take(ctx context.Context, key string, until time.Time) (bool, error)
	// Drop frees a key taken, so that it can be taken again. Dropping a key not taken
	// is not an error.
	Drop(ctx context.Context, key string) error
	// Save keeps a value under key until the instant given, replacing any before it.
	Save(ctx context.Context, key string, value []byte, until time.Time) error
	// Load returns the value saved under key; found is false when there is none.
	Load(ctx context.Context, key string) (value []byte, found bool, err error)
}

// Memory is a Store held in the memory of one process, for tests and a resource served by
// one process that never restarts within a challenge's life. It is not shared: a
// credential replayed on another replica, or after a restart, is not seen as spent and
// settles again. Durable is the Store for anything else. Entries are forgotten at the
// instant they were kept until.
type Memory struct {
	// Now is the clock; time.Now when nil.
	Now func() time.Time

	mu    sync.Mutex
	taken map[string]time.Time
	saved map[string]memory
	swept time.Time
}

type memory struct {
	value []byte
	until time.Time
}

func (m *Memory) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

// sweep forgets the entries whose time has passed, at most once a second; mu is held.
func (m *Memory) sweep(now time.Time) {
	if now.Sub(m.swept) < time.Second {
		return
	}
	m.swept = now
	for k, u := range m.taken {
		if !now.Before(u) {
			delete(m.taken, k)
		}
	}
	for k, v := range m.saved {
		if !now.Before(v.until) {
			delete(m.saved, k)
		}
	}
}

// Take implements Store.
func (m *Memory) Take(_ context.Context, key string, until time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.sweep(now)
	if m.taken == nil {
		m.taken = map[string]time.Time{}
	}
	if u, ok := m.taken[key]; ok && now.Before(u) {
		return false, nil
	}
	m.taken[key] = until
	return true, nil
}

// Drop implements Store.
func (m *Memory) Drop(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.taken, key)
	return nil
}

// Save implements Store.
func (m *Memory) Save(_ context.Context, key string, value []byte, until time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saved == nil {
		m.saved = map[string]memory{}
	}
	m.saved[key] = memory{append([]byte(nil), value...), until}
	return nil
}

// Load implements Store.
func (m *Memory) Load(_ context.Context, key string) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.saved[key]
	if !ok || !m.now().Before(v.until) {
		return nil, false, nil
	}
	return append([]byte(nil), v.value...), true, nil
}
