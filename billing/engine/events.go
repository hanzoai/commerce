package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/billingevent"

	. "github.com/hanzoai/commerce/types"
)

// Emit records a billing event, fully detached from the caller's request
// lifecycle (fire-and-forget). ctx MUST already carry the org namespace
// (org.Namespaced). Errors are logged, never returned — the event ledger must
// NEVER touch the money path.
//
// DELIVERY LIVES ELSEWHERE. Webhook delivery to subscriber URLs is the
// platform-global layer (hanzoai/cloud clients/webhooks: /v1/webhooks registry
// + the bus-driven dispatcher). Commerce publishes lifecycle events to the bus
// (events/publisher.go); this ledger is the queryable audit record.
func Emit(ctx context.Context, eventType, objectType, objectId, customerId string, data Map) {
	go func() {
		db := datastore.New(ctx)
		if _, err := EmitBillingEvent(db, eventType, objectType, objectId, customerId, data, nil); err != nil {
			log.Error("billing webhook emit %s (%s %s): %v", eventType, objectType, objectId, err)
		}
	}()
}

// EmitBillingEvent creates an append-only billing event record. It runs
// synchronously; callers that must stay off the money path invoke it via Emit
// (goroutine + detached context).
func EmitBillingEvent(db *datastore.Datastore, eventType, objectType, objectId, customerId string, data, previousData Map) (*billingevent.BillingEvent, error) {
	evt := billingevent.New(db)
	evt.Type = eventType
	evt.ObjectType = objectType
	evt.ObjectId = objectId
	evt.CustomerId = customerId
	evt.Data = data
	evt.PreviousData = previousData
	evt.Pending = true

	if err := evt.Create(); err != nil {
		return nil, fmt.Errorf("failed to create billing event: %w", err)
	}

	return evt, nil
}

// EmitBillingEventOnce records the billing event that key names, once: it is
// stored under a storage key derived from key, and a later call with the same key
// answers the event already there. An act that is retried after its event was
// written, or whose event is written again after the act, records one event.
func EmitBillingEventOnce(db *datastore.Datastore, key, eventType, objectType, objectId, customerId string, data Map) (*billingevent.BillingEvent, error) {
	sum := sha256.Sum256([]byte("billing-event\x00" + key))
	storageKey := db.NewKey("billing-event", "bevt_"+hex.EncodeToString(sum[:16]), 0, db.NewKey("synckey", "", 1, nil))
	evt := billingevent.New(db)
	switch err := evt.Get(storageKey); {
	case err == nil:
		return evt, nil
	case !errors.Is(err, datastore.ErrNoSuchEntity):
		return nil, fmt.Errorf("read billing event %s: %w", key, err)
	}
	evt = billingevent.New(db)
	if err := evt.SetKey(storageKey); err != nil {
		return nil, err
	}
	evt.Type = eventType
	evt.ObjectType = objectType
	evt.ObjectId = objectId
	evt.CustomerId = customerId
	evt.Data = data
	evt.Pending = true
	if err := evt.Create(); err != nil {
		return nil, fmt.Errorf("failed to create billing event: %w", err)
	}
	return evt, nil
}
