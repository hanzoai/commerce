// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"context"
	"time"
)

// Store is the state a Gate keeps so that a credential is spent once (sections 11.3,
// 11.5): the challenges and proofs taken, and the settled payments recorded. Each call is
// atomic for its key, and every process serving the resource sees the same state.
type Store interface {
	// Take marks key taken until the instant given; it reports false when already taken.
	Take(ctx context.Context, key string, until time.Time) (bool, error)
	// Drop frees a key taken.
	Drop(ctx context.Context, key string) error
	// Save keeps a value under key until the instant given, replacing any before it.
	Save(ctx context.Context, key string, value []byte, until time.Time) error
	// Load returns the value saved under key.
	Load(ctx context.Context, key string) (value []byte, found bool, err error)
}
