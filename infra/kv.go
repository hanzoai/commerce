// Package infra provides infrastructure clients.
//
// This file implements the KV client used for caching and org-id resolution.
// It is no longer Redis/Valkey-backed: per Hanzo policy (no Mongo, no KV
// stores) the cache is served by hanzo/base — a per-org/user embedded SQLite
// store. KVClient is a thin adapter over *store.KVStore that preserves the
// method set the rest of commerce consumes (Get/Set/Delete/Exists/Health/
// Close) plus the SetNX-family primitives the distributed lock builds on.
//
// The KVCache interface (string-valued Get/Set, variadic Delete) is what
// pkg/org binds to. KVClient satisfies it so org-id caching is unchanged.
package infra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hanzoai/commerce/store"
)

// KVConfig holds the base-backed KV configuration. The former Redis/Valkey
// fields (Addr, Password, DB, TLS, pool sizing) are gone — base needs only a
// data directory and an optional key prefix.
type KVConfig struct {
	// Enabled enables the KV service.
	Enabled bool

	// DataDir is the filesystem path for the base SQLite store. When empty,
	// the manager supplies a default under the app data dir.
	DataDir string

	// DataDSN optionally routes the store at Postgres (multi-instance
	// deployments). Empty selects the file-path SQLite default under DataDir.
	DataDSN string

	// KeyPrefix is prepended to all keys (namespacing within one store).
	KeyPrefix string
}

// KVClient is the base-backed cache adapter.
type KVClient struct {
	config *KVConfig
	store  *store.Store
	kv     *store.KVStore
}

// NewKVClient opens (or creates) the base store and returns a KV client over
// its commerce_kv collection. The store is owned by this client and released
// on Close.
func NewKVClient(_ context.Context, cfg *KVConfig) (*KVClient, error) {
	s, err := store.New(store.Config{
		DataDir: cfg.DataDir,
		DataDSN: cfg.DataDSN,
	})
	if err != nil {
		return nil, fmt.Errorf("kv: open base store: %w", err)
	}
	return &KVClient{
		config: cfg,
		store:  s,
		kv:     s.KV,
	}, nil
}

// NewKVClientFromStore adapts an already-constructed store (the common case —
// commerce builds one store for the whole process). The caller retains
// ownership of the store; Close here does NOT release it.
func NewKVClientFromStore(cfg *KVConfig, s *store.Store) *KVClient {
	return &KVClient{config: cfg, kv: s.KV}
}

// key returns the full key with prefix.
func (c *KVClient) key(k string) string {
	if c.config == nil || c.config.KeyPrefix == "" {
		return k
	}
	return c.config.KeyPrefix + ":" + k
}

// Get retrieves a value by key. A miss returns ("", nil) — matching the old
// Redis-nil-as-empty contract pkg/org depends on.
func (c *KVClient) Get(ctx context.Context, key string) (string, error) {
	val, err := c.kv.Get(c.key(key))
	if errors.Is(err, store.ErrKVNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("kv get failed: %w", err)
	}
	return string(val), nil
}

// GetJSON retrieves and unmarshals a JSON value. A miss leaves dst untouched.
func (c *KVClient) GetJSON(ctx context.Context, key string, dst interface{}) error {
	val, err := c.Get(ctx, key)
	if err != nil {
		return err
	}
	if val == "" {
		return nil
	}
	return json.Unmarshal([]byte(val), dst)
}

// Set stores a value with optional expiration (ttl==0 means no expiry).
func (c *KVClient) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	if err := c.kv.Set(c.key(key), []byte(value), ttl); err != nil {
		return fmt.Errorf("kv set failed: %w", err)
	}
	return nil
}

// SetJSON marshals and stores a value.
func (c *KVClient) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("kv json marshal failed: %w", err)
	}
	return c.Set(ctx, key, string(data), ttl)
}

// Delete removes one or more keys. Deleting an absent key is not an error.
func (c *KVClient) Delete(ctx context.Context, keys ...string) error {
	for _, k := range keys {
		if err := c.kv.Delete(c.key(k)); err != nil {
			return fmt.Errorf("kv delete failed: %w", err)
		}
	}
	return nil
}

// Exists returns the count of keys that currently hold a live value.
func (c *KVClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	var n int64
	for _, k := range keys {
		ok, err := c.kv.Exists(c.key(k))
		if err != nil {
			return 0, fmt.Errorf("kv exists failed: %w", err)
		}
		if ok {
			n++
		}
	}
	return n, nil
}

// Health checks the backing store with a round-trip set/get/delete on a
// reserved health key.
func (c *KVClient) Health(ctx context.Context) HealthStatus {
	start := time.Now()
	const probe = "__health__"
	if err := c.kv.Set(c.key(probe), []byte("1"), time.Second); err != nil {
		return HealthStatus{Healthy: false, Latency: time.Since(start), Error: err.Error()}
	}
	if _, err := c.kv.Get(c.key(probe)); err != nil {
		return HealthStatus{Healthy: false, Latency: time.Since(start), Error: err.Error()}
	}
	_ = c.kv.Delete(c.key(probe))
	return HealthStatus{Healthy: true, Latency: time.Since(start)}
}

// Close releases the store if this client owns it (constructed via
// NewKVClient). Clients adapted from a shared store (NewKVClientFromStore)
// leave the store untouched.
func (c *KVClient) Close() error {
	if c.store != nil {
		return c.store.Close(context.Background())
	}
	return nil
}

// Store exposes the underlying base KV store for advanced callers (locking).
func (c *KVClient) Store() *store.KVStore { return c.kv }
