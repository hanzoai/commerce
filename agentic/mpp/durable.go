// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"context"
	"encoding/base64"
	"errors"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/util/nscontext"
)

// Durable is a Store kept in commerce's datastore, on models/idempotencykey in the system
// namespace, so what it holds survives a restart and is the same on every replica that
// serves the resource: a credential spent on one is spent on all of them. It is the Store
// for a deployment of more than one process; Memory is for tests and one process.
//
// A key taken is kept, not forgotten at its time: the time is recorded as the instant a
// sweep of the rows of scope mpp/taken and mpp/saved may delete them, which is safe once
// the challenges they belong to have expired. The context of each call must carry the
// datastore, as a commerce request's does.
type Durable struct{}

const (
	scopeTaken = "mpp/taken"
	scopeSaved = "mpp/saved"
)

func system(ctx context.Context) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(ctx, "system"))
}

// find reads the row of a scope and key by its exact storage key.
func find(db *datastore.Datastore, scope, key string) (*idempotencykey.IdempotencyKey, bool, error) {
	rec := idempotencykey.New(db)
	err := rec.Get(db.NewKey(rec.Kind(), idempotencykey.DeterministicID(scope, key), 0, nil))
	switch {
	case errors.Is(err, datastore.ErrNoSuchEntity):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	}
	return rec, true, nil
}

// Take implements Store. The guard is made complete at once: one left started would be
// re-claimed as crashed after idempotencykey.StartedTTL, and a credential spent must stay spent.
func (Durable) Take(ctx context.Context, key string, until time.Time) (bool, error) {
	rec, replay, err := idempotencykey.Begin(system(ctx), scopeTaken, key)
	if err != nil || replay {
		return false, err
	}
	rec.Facts = until.UTC().Format(time.RFC3339)
	if err := idempotencykey.Complete(rec, ""); err != nil {
		_ = rec.Delete()
		return false, err
	}
	return true, nil
}

// Drop implements Store.
func (Durable) Drop(ctx context.Context, key string) error {
	rec, found, err := find(system(ctx), scopeTaken, key)
	if err != nil || !found {
		return err
	}
	return rec.Delete()
}

// Save implements Store. The value is kept base64-encoded, since the row holds text.
func (Durable) Save(ctx context.Context, key string, value []byte, until time.Time) error {
	rec, _, err := idempotencykey.Begin(system(ctx), scopeSaved, key)
	if err != nil {
		return err
	}
	rec.Facts = until.UTC().Format(time.RFC3339)
	return idempotencykey.Complete(rec, base64.StdEncoding.EncodeToString(value))
}

// Load implements Store.
func (Durable) Load(ctx context.Context, key string) ([]byte, bool, error) {
	rec, found, err := find(system(ctx), scopeSaved, key)
	if err != nil || !found || rec.Status != idempotencykey.StatusCompleted {
		return nil, false, err
	}
	b, err := base64.StdEncoding.DecodeString(rec.Response)
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}
