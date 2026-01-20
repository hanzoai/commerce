// Package infra provides infrastructure clients.
//
// This file implements the Valkey (Redis-compatible) client for caching
// and session storage.
package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// KVConfig holds Valkey/Redis configuration
type KVConfig struct {
	// Enabled enables the KV service
	Enabled bool

	// Addr is the Valkey server address (host:port)
	Addr string

	// Password for authentication (optional)
	Password string

	// DB is the database number to use
	DB int

	// TLS enables TLS connection
	TLS bool

	// PoolSize is the connection pool size
	PoolSize int

	// MinIdleConns minimum idle connections
	MinIdleConns int

	// ConnMaxIdleTime maximum idle time for connections
	ConnMaxIdleTime time.Duration

	// ReadTimeout for read operations
	ReadTimeout time.Duration

	// WriteTimeout for write operations
	WriteTimeout time.Duration

	// KeyPrefix is prepended to all keys
	KeyPrefix string
}

// KVClient wraps the Redis client for Valkey
type KVClient struct {
	config *KVConfig
	client *redis.Client
}

// NewKVClient creates a new Valkey KV client
func NewKVClient(ctx context.Context, cfg *KVConfig) (*KVClient, error) {
	if cfg.PoolSize == 0 {
		cfg.PoolSize = 10
	}
	if cfg.MinIdleConns == 0 {
		cfg.MinIdleConns = 2
	}
	if cfg.ConnMaxIdleTime == 0 {
		cfg.ConnMaxIdleTime = 5 * time.Minute
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 3 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 3 * time.Second
	}

	opts := &redis.Options{
		Addr:            cfg.Addr,
		Password:        cfg.Password,
		DB:              cfg.DB,
		PoolSize:        cfg.PoolSize,
		MinIdleConns:    cfg.MinIdleConns,
		ConnMaxIdleTime: cfg.ConnMaxIdleTime,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
	}

	client := redis.NewClient(opts)

	// Verify connection
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to valkey: %w", err)
	}

	return &KVClient{
		config: cfg,
		client: client,
	}, nil
}

// key returns the full key with prefix
func (c *KVClient) key(k string) string {
	if c.config.KeyPrefix == "" {
		return k
	}
	return c.config.KeyPrefix + ":" + k
}

// Get retrieves a value by key
func (c *KVClient) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, c.key(key)).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("kv get failed: %w", err)
	}
	return val, nil
}

// GetJSON retrieves and unmarshals a JSON value
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

// Set stores a value with optional expiration
func (c *KVClient) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	err := c.client.Set(ctx, c.key(key), value, ttl).Err()
	if err != nil {
		return fmt.Errorf("kv set failed: %w", err)
	}
	return nil
}

// SetJSON marshals and stores a value
func (c *KVClient) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("kv json marshal failed: %w", err)
	}
	return c.Set(ctx, key, string(data), ttl)
}

// Delete removes a key
func (c *KVClient) Delete(ctx context.Context, keys ...string) error {
	fullKeys := make([]string, len(keys))
	for i, k := range keys {
		fullKeys[i] = c.key(k)
	}
	err := c.client.Del(ctx, fullKeys...).Err()
	if err != nil {
		return fmt.Errorf("kv delete failed: %w", err)
	}
	return nil
}

// Exists checks if keys exist
func (c *KVClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	fullKeys := make([]string, len(keys))
	for i, k := range keys {
		fullKeys[i] = c.key(k)
	}
	count, err := c.client.Exists(ctx, fullKeys...).Result()
	if err != nil {
		return 0, fmt.Errorf("kv exists failed: %w", err)
	}
	return count, nil
}

// Expire sets expiration on a key
func (c *KVClient) Expire(ctx context.Context, key string, ttl time.Duration) error {
	err := c.client.Expire(ctx, c.key(key), ttl).Err()
	if err != nil {
		return fmt.Errorf("kv expire failed: %w", err)
	}
	return nil
}

// TTL returns the remaining TTL of a key
func (c *KVClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := c.client.TTL(ctx, c.key(key)).Result()
	if err != nil {
		return 0, fmt.Errorf("kv ttl failed: %w", err)
	}
	return ttl, nil
}

// Incr increments a counter
func (c *KVClient) Incr(ctx context.Context, key string) (int64, error) {
	val, err := c.client.Incr(ctx, c.key(key)).Result()
	if err != nil {
		return 0, fmt.Errorf("kv incr failed: %w", err)
	}
	return val, nil
}

// IncrBy increments a counter by a value
func (c *KVClient) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	val, err := c.client.IncrBy(ctx, c.key(key), value).Result()
	if err != nil {
		return 0, fmt.Errorf("kv incrby failed: %w", err)
	}
	return val, nil
}

// HGet retrieves a hash field
func (c *KVClient) HGet(ctx context.Context, key, field string) (string, error) {
	val, err := c.client.HGet(ctx, c.key(key), field).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("kv hget failed: %w", err)
	}
	return val, nil
}

// HSet sets hash fields
func (c *KVClient) HSet(ctx context.Context, key string, values ...interface{}) error {
	err := c.client.HSet(ctx, c.key(key), values...).Err()
	if err != nil {
		return fmt.Errorf("kv hset failed: %w", err)
	}
	return nil
}

// HGetAll retrieves all hash fields
func (c *KVClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	val, err := c.client.HGetAll(ctx, c.key(key)).Result()
	if err != nil {
		return nil, fmt.Errorf("kv hgetall failed: %w", err)
	}
	return val, nil
}

// HDel removes hash fields
func (c *KVClient) HDel(ctx context.Context, key string, fields ...string) error {
	err := c.client.HDel(ctx, c.key(key), fields...).Err()
	if err != nil {
		return fmt.Errorf("kv hdel failed: %w", err)
	}
	return nil
}

// LPush prepends values to a list
func (c *KVClient) LPush(ctx context.Context, key string, values ...interface{}) error {
	err := c.client.LPush(ctx, c.key(key), values...).Err()
	if err != nil {
		return fmt.Errorf("kv lpush failed: %w", err)
	}
	return nil
}

// RPush appends values to a list
func (c *KVClient) RPush(ctx context.Context, key string, values ...interface{}) error {
	err := c.client.RPush(ctx, c.key(key), values...).Err()
	if err != nil {
		return fmt.Errorf("kv rpush failed: %w", err)
	}
	return nil
}

// LRange retrieves list elements
func (c *KVClient) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	val, err := c.client.LRange(ctx, c.key(key), start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("kv lrange failed: %w", err)
	}
	return val, nil
}

// LLen returns the length of a list
func (c *KVClient) LLen(ctx context.Context, key string) (int64, error) {
	val, err := c.client.LLen(ctx, c.key(key)).Result()
	if err != nil {
		return 0, fmt.Errorf("kv llen failed: %w", err)
	}
	return val, nil
}

// SAdd adds members to a set
func (c *KVClient) SAdd(ctx context.Context, key string, members ...interface{}) error {
	err := c.client.SAdd(ctx, c.key(key), members...).Err()
	if err != nil {
		return fmt.Errorf("kv sadd failed: %w", err)
	}
	return nil
}

// SMembers retrieves all set members
func (c *KVClient) SMembers(ctx context.Context, key string) ([]string, error) {
	val, err := c.client.SMembers(ctx, c.key(key)).Result()
	if err != nil {
		return nil, fmt.Errorf("kv smembers failed: %w", err)
	}
	return val, nil
}

// SIsMember checks if a member is in a set
func (c *KVClient) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	val, err := c.client.SIsMember(ctx, c.key(key), member).Result()
	if err != nil {
		return false, fmt.Errorf("kv sismember failed: %w", err)
	}
	return val, nil
}

// SRem removes members from a set
func (c *KVClient) SRem(ctx context.Context, key string, members ...interface{}) error {
	err := c.client.SRem(ctx, c.key(key), members...).Err()
	if err != nil {
		return fmt.Errorf("kv srem failed: %w", err)
	}
	return nil
}

// ZAdd adds members to a sorted set
func (c *KVClient) ZAdd(ctx context.Context, key string, members ...redis.Z) error {
	err := c.client.ZAdd(ctx, c.key(key), members...).Err()
	if err != nil {
		return fmt.Errorf("kv zadd failed: %w", err)
	}
	return nil
}

// ZRange retrieves sorted set members by rank
func (c *KVClient) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	val, err := c.client.ZRange(ctx, c.key(key), start, stop).Result()
	if err != nil {
		return nil, fmt.Errorf("kv zrange failed: %w", err)
	}
	return val, nil
}

// ZRangeByScore retrieves sorted set members by score
func (c *KVClient) ZRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) ([]string, error) {
	val, err := c.client.ZRangeByScore(ctx, c.key(key), opt).Result()
	if err != nil {
		return nil, fmt.Errorf("kv zrangebyscore failed: %w", err)
	}
	return val, nil
}

// Pipeline returns a pipeline for batched operations
func (c *KVClient) Pipeline() redis.Pipeliner {
	return c.client.Pipeline()
}

// Watch executes a transaction with WATCH
func (c *KVClient) Watch(ctx context.Context, fn func(*redis.Tx) error, keys ...string) error {
	fullKeys := make([]string, len(keys))
	for i, k := range keys {
		fullKeys[i] = c.key(k)
	}
	return c.client.Watch(ctx, fn, fullKeys...)
}

// Publish publishes a message to a channel
func (c *KVClient) Publish(ctx context.Context, channel string, message interface{}) error {
	err := c.client.Publish(ctx, channel, message).Err()
	if err != nil {
		return fmt.Errorf("kv publish failed: %w", err)
	}
	return nil
}

// Subscribe subscribes to channels
func (c *KVClient) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return c.client.Subscribe(ctx, channels...)
}

// Health checks the Valkey connection
func (c *KVClient) Health(ctx context.Context) HealthStatus {
	start := time.Now()

	err := c.client.Ping(ctx).Err()
	if err != nil {
		return HealthStatus{
			Healthy: false,
			Latency: time.Since(start),
			Error:   err.Error(),
		}
	}

	return HealthStatus{
		Healthy: true,
		Latency: time.Since(start),
	}
}

// Close closes the Valkey connection
func (c *KVClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// Client returns the underlying Redis client for advanced operations
func (c *KVClient) Client() *redis.Client {
	return c.client
}
