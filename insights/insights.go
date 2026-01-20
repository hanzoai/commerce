// Package insights provides integration with Hanzo Insights (PostHog fork).
//
// Insights is Hanzo's product analytics platform, forked from PostHog.
// This package enables Commerce to send events to Insights for tracking
// user behavior, conversions, feature usage, and more.
//
// Usage:
//
//	client := insights.NewClient(&insights.Config{
//	    Endpoint: "https://insights.hanzo.ai",
//	    APIKey:   "phc_...",
//	})
//
//	client.Capture(&insights.Event{
//	    DistinctID: "user_123",
//	    Event:      "order_completed",
//	    Properties: map[string]interface{}{
//	        "order_id": "ord_abc",
//	        "total":    99.99,
//	    },
//	})
package insights

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Config holds Insights client configuration.
type Config struct {
	// Endpoint is the Insights API endpoint (e.g., "https://insights.hanzo.ai")
	Endpoint string

	// APIKey is the project API key (e.g., "phc_...")
	APIKey string

	// BatchSize is the number of events to batch before sending (default: 100)
	BatchSize int

	// FlushInterval is how often to flush batched events (default: 30s)
	FlushInterval time.Duration

	// Timeout is the HTTP request timeout (default: 10s)
	Timeout time.Duration

	// Async enables asynchronous event sending (default: true)
	Async bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Endpoint:      "https://insights.hanzo.ai",
		BatchSize:     100,
		FlushInterval: 30 * time.Second,
		Timeout:       10 * time.Second,
		Async:         true,
	}
}

// Event represents an analytics event to send to Insights.
type Event struct {
	// Event name (required)
	Event string `json:"event"`

	// DistinctID is the user identifier (required)
	DistinctID string `json:"distinct_id"`

	// Properties contains custom event data
	Properties map[string]interface{} `json:"properties,omitempty"`

	// Timestamp is when the event occurred (defaults to now)
	Timestamp time.Time `json:"timestamp,omitempty"`

	// SentAt is when the event was sent (defaults to now)
	SentAt time.Time `json:"sent_at,omitempty"`
}

// IdentifyEvent represents a user identification event.
type IdentifyEvent struct {
	DistinctID string                 `json:"distinct_id"`
	Properties map[string]interface{} `json:"$set,omitempty"`
	SetOnce    map[string]interface{} `json:"$set_once,omitempty"`
}

// Client is the Insights API client.
type Client struct {
	config     *Config
	httpClient *http.Client
	eventQueue chan *Event
	wg         sync.WaitGroup
	closed     bool
	mu         sync.RWMutex
}

// NewClient creates a new Insights client.
func NewClient(config *Config) *Client {
	if config == nil {
		config = DefaultConfig()
	}

	if config.BatchSize == 0 {
		config.BatchSize = 100
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = 30 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	client := &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		eventQueue: make(chan *Event, config.BatchSize*10),
	}

	if config.Async {
		client.wg.Add(1)
		go client.processBatch()
	}

	return client
}

// Capture sends an event to Insights.
func (c *Client) Capture(event *Event) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.SentAt.IsZero() {
		event.SentAt = time.Now()
	}

	c.mu.RLock()
	async := c.config.Async && !c.closed
	c.mu.RUnlock()

	if async {
		select {
		case c.eventQueue <- event:
			return nil
		default:
			// Queue full, send synchronously
			return c.sendEvents([]*Event{event})
		}
	}

	return c.sendEvents([]*Event{event})
}

// CaptureOrderEvent sends a commerce order event.
func (c *Client) CaptureOrderEvent(distinctID string, eventName string, orderID string, total float64, items []map[string]interface{}) error {
	return c.Capture(&Event{
		DistinctID: distinctID,
		Event:      eventName,
		Properties: map[string]interface{}{
			"order_id":    orderID,
			"total":       total,
			"items":       items,
			"item_count":  len(items),
			"$lib":        "hanzo-commerce",
			"$lib_method": "server",
		},
	})
}

// CapturePageView sends a page view event.
func (c *Client) CapturePageView(distinctID string, url string, title string, referrer string) error {
	return c.Capture(&Event{
		DistinctID: distinctID,
		Event:      "$pageview",
		Properties: map[string]interface{}{
			"$current_url": url,
			"title":        title,
			"$referrer":    referrer,
			"$lib":         "hanzo-commerce",
		},
	})
}

// Identify sends user identification data.
func (c *Client) Identify(distinctID string, properties map[string]interface{}) error {
	return c.Capture(&Event{
		DistinctID: distinctID,
		Event:      "$identify",
		Properties: map[string]interface{}{
			"$set": properties,
		},
	})
}

// Alias links two distinct IDs (e.g., anonymous → authenticated).
func (c *Client) Alias(distinctID string, alias string) error {
	return c.Capture(&Event{
		DistinctID: distinctID,
		Event:      "$create_alias",
		Properties: map[string]interface{}{
			"alias": alias,
		},
	})
}

// GroupIdentify associates a user with a group (e.g., organization).
func (c *Client) GroupIdentify(groupType string, groupKey string, properties map[string]interface{}) error {
	return c.Capture(&Event{
		DistinctID: fmt.Sprintf("$%s_%s", groupType, groupKey),
		Event:      "$groupidentify",
		Properties: map[string]interface{}{
			"$group_type":       groupType,
			"$group_key":        groupKey,
			"$group_set":        properties,
			"distinct_id":       fmt.Sprintf("$%s_%s", groupType, groupKey),
			"$process_person_profile": false,
		},
	})
}

// sendEvents sends events to the Insights capture endpoint.
func (c *Client) sendEvents(events []*Event) error {
	if len(events) == 0 {
		return nil
	}

	// Build batch payload
	batch := make([]map[string]interface{}, len(events))
	for i, event := range events {
		batch[i] = map[string]interface{}{
			"api_key":     c.config.APIKey,
			"event":       event.Event,
			"distinct_id": event.DistinctID,
			"properties":  event.Properties,
			"timestamp":   event.Timestamp.Format(time.RFC3339),
			"sent_at":     event.SentAt.Format(time.RFC3339),
		}
	}

	body, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("failed to marshal events: %w", err)
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		c.config.Endpoint+"/batch/",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("insights API error: status %d", resp.StatusCode)
	}

	return nil
}

// processBatch processes the event queue in batches.
func (c *Client) processBatch() {
	defer c.wg.Done()

	batch := make([]*Event, 0, c.config.BatchSize)
	ticker := time.NewTicker(c.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-c.eventQueue:
			if !ok {
				// Channel closed, flush remaining
				if len(batch) > 0 {
					c.sendEvents(batch)
				}
				return
			}

			batch = append(batch, event)
			if len(batch) >= c.config.BatchSize {
				c.sendEvents(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				c.sendEvents(batch)
				batch = batch[:0]
			}
		}
	}
}

// Flush sends all queued events immediately.
func (c *Client) Flush() error {
	if !c.config.Async {
		return nil
	}

	// Drain the queue
	batch := make([]*Event, 0, c.config.BatchSize)
	for {
		select {
		case event := <-c.eventQueue:
			batch = append(batch, event)
		default:
			// Queue empty
			if len(batch) > 0 {
				return c.sendEvents(batch)
			}
			return nil
		}
	}
}

// Close gracefully shuts down the client.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	close(c.eventQueue)
	c.wg.Wait()
	return nil
}

// StandardEventNames defines common e-commerce event names.
var StandardEventNames = struct {
	PageView        string
	ProductViewed   string
	ProductAdded    string
	ProductRemoved  string
	CartViewed      string
	CheckoutStarted string
	CheckoutStep    string
	OrderCompleted  string
	OrderRefunded   string
	SignedUp        string
	SignedIn        string
	SignedOut       string
}{
	PageView:        "$pageview",
	ProductViewed:   "product_viewed",
	ProductAdded:    "product_added",
	ProductRemoved:  "product_removed",
	CartViewed:      "cart_viewed",
	CheckoutStarted: "checkout_started",
	CheckoutStep:    "checkout_step_completed",
	OrderCompleted:  "order_completed",
	OrderRefunded:   "order_refunded",
	SignedUp:        "signed_up",
	SignedIn:        "signed_in",
	SignedOut:       "signed_out",
}
