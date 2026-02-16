// Package infra provides infrastructure clients.
//
// This file implements distributed locking backed by Redis/Valkey.
// Locks use atomic SET NX with TTL for acquisition and Lua scripts
// for safe compare-and-delete release and compare-and-extend renewal.
package infra

import (
	"context"
	"errors"
	"fmt"
	"time"

	kv "github.com/hanzoai/kv-go/v9"

	"github.com/hanzoai/commerce/util/rand"
)

var (
	// ErrLockNotAcquired is returned when the lock is already held.
	ErrLockNotAcquired = errors.New("lock: not acquired")

	// ErrLockNotHeld is returned when releasing or extending a lock
	// that is no longer held by this instance.
	ErrLockNotHeld = errors.New("lock: not held")
)

// Lock represents a distributed lock backed by Redis/Valkey.
type Lock struct {
	kv    *KVClient
	key   string
	value string
	ttl   time.Duration
}

// Acquire attempts to acquire a distributed lock with the given key.
// The key is prefixed via the KV client's key prefix. Returns a Lock
// that must be released when done.
func (m *Manager) Acquire(ctx context.Context, key string, ttl time.Duration) (*Lock, error) {
	if m.kv == nil || !m.config.KV.Enabled {
		return nil, fmt.Errorf("lock: KV not enabled")
	}

	lockKey := m.kv.key("lock:" + key)
	lockValue := rand.ShortId()

	ok, err := m.kv.client.SetNX(ctx, lockKey, lockValue, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("lock: kv error: %w", err)
	}
	if !ok {
		return nil, ErrLockNotAcquired
	}

	return &Lock{
		kv:    m.kv,
		key:   lockKey,
		value: lockValue,
		ttl:   ttl,
	}, nil
}

// releaseScript atomically deletes the key only if it holds the expected value.
var releaseScript = kv.NewScript(`
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("del", KEYS[1])
	else
		return 0
	end
`)

// Release releases the distributed lock.
// Only releases if the lock is still held by this instance (compare-and-delete).
func (l *Lock) Release(ctx context.Context) error {
	result, err := releaseScript.Run(ctx, l.kv.client, []string{l.key}, l.value).Int64()
	if err != nil {
		return fmt.Errorf("lock: release error: %w", err)
	}
	if result == 0 {
		return ErrLockNotHeld
	}
	return nil
}

// extendScript atomically extends the TTL only if the key holds the expected value.
var extendScript = kv.NewScript(`
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("pexpire", KEYS[1], ARGV[2])
	else
		return 0
	end
`)

// Extend extends the TTL of the lock.
// Only extends if the lock is still held by this instance.
func (l *Lock) Extend(ctx context.Context, ttl time.Duration) error {
	result, err := extendScript.Run(ctx, l.kv.client, []string{l.key}, l.value, int64(ttl/time.Millisecond)).Int64()
	if err != nil {
		return fmt.Errorf("lock: extend error: %w", err)
	}
	if result == 0 {
		return ErrLockNotHeld
	}
	l.ttl = ttl
	return nil
}
