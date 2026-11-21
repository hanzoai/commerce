// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"context"
	"sync"
	"time"
)

// Idempotency is handled at the business layer (signatures, "Replay
// Protection"): a state-changing request carries an Idempotency-Key; a duplicate
// whose raw body matches the original returns the original response and runs
// nothing; a duplicate whose body differs is rejected with 409; a store that
// cannot answer fails closed with 503.

// Keep is how long a business stores a key: the specification requires at
// least 24 hours and recommends 48.
const Keep = 48 * time.Hour

// minKeyLength is the shortest key that can hold 128 bits of entropy at six
// bits per character (a UUID is 36 characters, "22+ char alphanumeric" is the
// specification's own example).
const minKeyLength = 22

// Stored is a response kept for replay.
type Stored struct {
	Status      int
	ContentType string
	Body        []byte
}

// Seen is what a store knows of a key.
type Seen struct {
	// Digest is the hex SHA-256 of the raw body of the request that first used the key.
	Digest string
	// Done is false while that request is still running.
	Done     bool
	Response Stored
}

// Store keeps idempotency records. Implementations must make Begin atomic:
// of two concurrent first sightings of a key, one is new and the other sees it.
type Store interface {
	// Begin reserves (scope, key) for a request whose body digest is digest. It
	// returns nil when the key is new, or what was recorded when it is not.
	Begin(ctx context.Context, scope, key, digest string) (*Seen, error)
	// Finish records the response of a reserved key; digest is the one Begin saw.
	Finish(ctx context.Context, scope, key, digest string, r Stored) error
	// Release drops a reservation whose operation did not complete, so the
	// request can be sent again.
	Release(ctx context.Context, scope, key string) error
}

// Memory is an in-process Store. A host that runs more than one replica needs a
// shared store.
type Memory struct {
	keep time.Duration
	now  func() time.Time
	mu   sync.Mutex
	m    map[string]*memoryRecord
}

type memoryRecord struct {
	seen    Seen
	expires time.Time
}

// NewMemory returns a Memory store that keeps records for keep; zero is Keep.
func NewMemory(keep time.Duration) *Memory {
	if keep <= 0 {
		keep = Keep
	}
	return &Memory{keep: keep, now: time.Now, m: map[string]*memoryRecord{}}
}

func (s *Memory) Begin(_ context.Context, scope, key, digest string) (*Seen, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for k, r := range s.m {
		if now.After(r.expires) {
			delete(s.m, k)
		}
	}
	id := scope + "\x00" + key
	if r, ok := s.m[id]; ok {
		seen := r.seen
		return &seen, nil
	}
	s.m[id] = &memoryRecord{seen: Seen{Digest: digest}, expires: now.Add(s.keep)}
	return nil, nil
}

func (s *Memory) Finish(_ context.Context, scope, key, _ string, r Stored) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.m[scope+"\x00"+key]; ok {
		rec.seen.Done, rec.seen.Response = true, r
	}
	return nil
}

func (s *Memory) Release(_ context.Context, scope, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, scope+"\x00"+key)
	return nil
}
