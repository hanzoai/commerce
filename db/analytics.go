package db

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// NewAnalytics creates a new analytics datastore connection
// This connects to ClickHouse via hanzo/datastore-go for deep analytics
func NewAnalytics(cfg *Config) (Analytics, error) {
	if cfg.AnalyticsDSN == "" {
		return nil, errors.New("db: AnalyticsDSN is required for analytics")
	}

	// Import hanzo/datastore-go for ClickHouse connectivity
	// The actual implementation will use the datastore-go package
	return &clickhouseAnalytics{
		dsn:    cfg.AnalyticsDSN,
		config: cfg.Analytics,
	}, nil
}

// clickhouseAnalytics implements Analytics using ClickHouse
type clickhouseAnalytics struct {
	dsn    string
	config AnalyticsConfig
	// conn is the ClickHouse connection from datastore-go
	// conn *datastore.Conn
}

// Query executes an analytics query
func (c *clickhouseAnalytics) Query(ctx context.Context, query string, args ...interface{}) (AnalyticsRows, error) {
	// TODO: Implement using hanzo/datastore-go
	// This will be implemented when we integrate the datastore-go package
	//
	// Example usage with datastore-go:
	// rows, err := c.conn.Query(ctx, query, args...)
	// return &clickhouseRows{rows: rows}, err

	return nil, errors.New("analytics: not implemented - requires hanzo/datastore-go integration")
}

// Select scans results into a destination
func (c *clickhouseAnalytics) Select(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	// TODO: Implement using hanzo/datastore-go
	// return c.conn.Select(ctx, dest, query, args...)

	return errors.New("analytics: not implemented - requires hanzo/datastore-go integration")
}

// Exec executes a non-query statement
func (c *clickhouseAnalytics) Exec(ctx context.Context, query string, args ...interface{}) error {
	// TODO: Implement using hanzo/datastore-go
	// return c.conn.Exec(ctx, query, args...)

	return errors.New("analytics: not implemented - requires hanzo/datastore-go integration")
}

// PrepareBatch prepares a batch insert
func (c *clickhouseAnalytics) PrepareBatch(ctx context.Context, query string) (AnalyticsBatch, error) {
	// TODO: Implement using hanzo/datastore-go
	// batch, err := c.conn.PrepareBatch(ctx, query)
	// return &clickhouseBatch{batch: batch}, err

	return nil, errors.New("analytics: not implemented - requires hanzo/datastore-go integration")
}

// AsyncInsert performs an async insert
func (c *clickhouseAnalytics) AsyncInsert(ctx context.Context, query string, wait bool, args ...interface{}) error {
	// TODO: Implement using hanzo/datastore-go
	// return c.conn.AsyncInsert(ctx, query, wait, args...)

	return errors.New("analytics: not implemented - requires hanzo/datastore-go integration")
}

// Close closes the analytics connection
func (c *clickhouseAnalytics) Close() error {
	// TODO: Implement
	// return c.conn.Close()
	return nil
}

// NoOpAnalytics is a no-op implementation when analytics is disabled
type NoOpAnalytics struct{}

func (n *NoOpAnalytics) Query(ctx context.Context, query string, args ...interface{}) (AnalyticsRows, error) {
	return &noOpRows{}, nil
}

func (n *NoOpAnalytics) Select(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return nil
}

func (n *NoOpAnalytics) Exec(ctx context.Context, query string, args ...interface{}) error {
	return nil
}

func (n *NoOpAnalytics) PrepareBatch(ctx context.Context, query string) (AnalyticsBatch, error) {
	return &noOpBatch{}, nil
}

func (n *NoOpAnalytics) AsyncInsert(ctx context.Context, query string, wait bool, args ...interface{}) error {
	return nil
}

func (n *NoOpAnalytics) Close() error {
	return nil
}

// noOpRows implements AnalyticsRows
type noOpRows struct{}

func (r *noOpRows) Next() bool                        { return false }
func (r *noOpRows) Scan(dest ...interface{}) error    { return nil }
func (r *noOpRows) ScanStruct(dest interface{}) error { return nil }
func (r *noOpRows) Columns() []string                 { return nil }
func (r *noOpRows) Close() error                      { return nil }
func (r *noOpRows) Err() error                        { return nil }

// noOpBatch implements AnalyticsBatch
type noOpBatch struct{}

func (b *noOpBatch) Append(v ...interface{}) error    { return nil }
func (b *noOpBatch) AppendStruct(v interface{}) error { return nil }
func (b *noOpBatch) Flush() error                     { return nil }
func (b *noOpBatch) Send() error                      { return nil }
func (b *noOpBatch) Abort() error                     { return nil }
func (b *noOpBatch) Rows() int                        { return 0 }
func (b *noOpBatch) Close() error                     { return nil }

// SyncConfig configures how data is synced to analytics
type SyncConfig struct {
	// Enabled turns on analytics sync
	Enabled bool

	// BatchSize is the number of records to batch before sending
	BatchSize int

	// FlushInterval is how often to flush partial batches
	FlushInterval time.Duration

	// Kinds specifies which entity kinds to sync (empty = all)
	Kinds []string

	// AsyncInsert uses ClickHouse async insert for fire-and-forget
	AsyncInsert bool
}

// DefaultSyncConfig returns default sync configuration
func DefaultSyncConfig() *SyncConfig {
	return &SyncConfig{
		Enabled:       false,
		BatchSize:     1000,
		FlushInterval: 10 * time.Second,
		AsyncInsert:   true,
	}
}

// Syncer handles syncing data from SQLite to analytics
type Syncer struct {
	config    *SyncConfig
	analytics Analytics
	pending   []syncRecord
	lastFlush time.Time
}

type syncRecord struct {
	kind      string
	id        string
	data      []byte
	timestamp time.Time
}

// NewSyncer creates a new analytics syncer
func NewSyncer(config *SyncConfig, analytics Analytics) *Syncer {
	return &Syncer{
		config:    config,
		analytics: analytics,
		pending:   make([]syncRecord, 0, config.BatchSize),
		lastFlush: time.Now(),
	}
}

// Sync queues a record for syncing to analytics
func (s *Syncer) Sync(kind, id string, data []byte) error {
	if !s.config.Enabled || s.analytics == nil {
		return nil
	}

	// Check if kind should be synced
	if len(s.config.Kinds) > 0 {
		found := false
		for _, k := range s.config.Kinds {
			if k == kind {
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}

	s.pending = append(s.pending, syncRecord{
		kind:      kind,
		id:        id,
		data:      data,
		timestamp: time.Now(),
	})

	// Flush if batch is full
	if len(s.pending) >= s.config.BatchSize {
		return s.Flush(context.Background())
	}

	// Flush if interval elapsed
	if time.Since(s.lastFlush) > s.config.FlushInterval {
		return s.Flush(context.Background())
	}

	return nil
}

// Flush sends pending records to analytics
func (s *Syncer) Flush(ctx context.Context) error {
	if len(s.pending) == 0 {
		return nil
	}

	// Group by kind
	byKind := make(map[string][]syncRecord)
	for _, r := range s.pending {
		byKind[r.kind] = append(byKind[r.kind], r)
	}

	var lastErr error
	for kind, records := range byKind {
		if err := s.flushKind(ctx, kind, records); err != nil {
			lastErr = err
		}
	}

	s.pending = s.pending[:0]
	s.lastFlush = time.Now()

	return lastErr
}

func (s *Syncer) flushKind(ctx context.Context, kind string, records []syncRecord) error {
	if s.config.AsyncInsert {
		// Use async insert for fire-and-forget
		for _, r := range records {
			query := fmt.Sprintf(`INSERT INTO %s_events (id, data, timestamp) VALUES (?, ?, ?)`, kind)
			if err := s.analytics.AsyncInsert(ctx, query, false, r.id, r.data, r.timestamp); err != nil {
				return err
			}
		}
		return nil
	}

	// Use batch insert for guaranteed delivery
	query := fmt.Sprintf(`INSERT INTO %s_events (id, data, timestamp) VALUES`, kind)
	batch, err := s.analytics.PrepareBatch(ctx, query)
	if err != nil {
		return err
	}
	defer batch.Close()

	for _, r := range records {
		if err := batch.Append(r.id, r.data, r.timestamp); err != nil {
			batch.Abort()
			return err
		}
	}

	return batch.Send()
}

// Close flushes any pending records and closes the syncer
func (s *Syncer) Close() error {
	return s.Flush(context.Background())
}
