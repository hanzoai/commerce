// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"container/list"
	"context"
	"errors"
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

// MinKeep is the least a business stores an idempotency key: 24 hours.
const MinKeep = 24 * time.Hour

// ErrFull is returned by a store that holds as many keys as it may. The server
// answers 503, failing closed, rather than serve a request it cannot make
// repeatable; a live key is never evicted to make room.
var ErrFull = errors.New("ucp: the idempotency store is full")

// Memory is an in-process Store: an index by key, with records kept in the
// order they expire so that Begin drops the expired without scanning, and a cap
// on how many it holds. A host that runs more than one replica needs a shared
// store (Idempotency).
type Memory struct {
	keep time.Duration
	max  int
	now  func() time.Time
	mu   sync.Mutex
	m    map[string]*memoryRecord
	// order lists records oldest first; every record lives for the same time, so
	// that is also the order they expire in.
	order *list.List
}

type memoryRecord struct {
	id      string
	seen    Seen
	expires time.Time
	el      *list.Element
}

// NewMemory returns a Memory store that keeps records for keep (zero is Keep,
// and not less than MinKeep) and holds at most max of them, which the host
// chooses: no size is assumed for it.
func NewMemory(keep time.Duration, max int) (*Memory, error) {
	if keep == 0 {
		keep = Keep
	}
	if keep < MinKeep {
		return nil, errors.New("ucp: idempotency records must be kept at least 24 hours")
	}
	if max <= 0 {
		return nil, errors.New("ucp: a Memory store needs a cap on its entries")
	}
	return &Memory{keep: keep, max: max, now: time.Now, m: map[string]*memoryRecord{}, order: list.New()}, nil
}

// Begin implements Store.
func (s *Memory) Begin(_ context.Context, scope, key, digest string) (*Seen, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for el := s.order.Front(); el != nil; el = s.order.Front() {
		r := el.Value.(*memoryRecord)
		if !now.After(r.expires) {
			break
		}
		s.order.Remove(el)
		delete(s.m, r.id)
	}
	id := scope + "\x00" + key
	if r, ok := s.m[id]; ok {
		seen := r.seen
		return &seen, nil
	}
	if len(s.m) >= s.max {
		return nil, ErrFull
	}
	r := &memoryRecord{id: id, seen: Seen{Digest: digest}, expires: now.Add(s.keep)}
	r.el = s.order.PushBack(r)
	s.m[id] = r
	return nil, nil
}

// Finish implements Store.
func (s *Memory) Finish(_ context.Context, scope, key, _ string, r Stored) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.m[scope+"\x00"+key]; ok {
		rec.seen.Done, rec.seen.Response = true, r
	}
	return nil
}

// Release implements Store.
func (s *Memory) Release(_ context.Context, scope, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := scope + "\x00" + key
	if r, ok := s.m[id]; ok {
		s.order.Remove(r.el)
		delete(s.m, id)
	}
	return nil
}
