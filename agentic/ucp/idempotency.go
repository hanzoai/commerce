// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"context"
	"time"
)

// Idempotency is business-layer replay protection (signatures, "Replay
// Protection"): a duplicate request whose raw body matches the original returns
// the original response and runs nothing, a duplicate whose body differs is a
// 409, and a store that cannot answer fails closed with 503.

// Keep is how long a business stores a key: at least 24 hours, 48 recommended
// ("Idempotency Key Requirements").
const Keep = 48 * time.Hour

// minKeyLength is the shortest key that can carry the 128 bits of entropy the
// specification requires, at six bits a character; a UUID is 36 characters.
const minKeyLength = 22

// Stored is a response kept for replay.
type Stored struct {
	Status int
	Body   []byte
}

// Seen is what a store knows of a key.
type Seen struct {
	// Digest is the hex SHA-256 of the raw body of the request that first used the key.
	Digest string
	// Done is false while that request is still running.
	Done     bool
	Response Stored
}

// Store keeps idempotency records. Begin must be atomic: of two concurrent first
// sightings of a key, one is new and the other sees it.
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
