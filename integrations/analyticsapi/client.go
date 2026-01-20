// Package analyticsapi provides integration with Hanzo Analytics.
//
// Hanzo Analytics is a privacy-focused web analytics platform (similar to Umami).
// This package enables Commerce to send events to Analytics for tracking
// page views, sessions, and custom events.
//
// Usage:
//
//	client := analyticsapi.NewClient(&analyticsapi.Config{
//	    Endpoint: "https://analytics.hanzo.ai",
//	    WebsiteID: "website-uuid",
//	})
//
//	client.TrackPageView(&analyticsapi.PageViewEvent{
//	    URL: "https://example.com/products",
//	    Title: "Products",
//	    Referrer: "https://google.com",
//	})
package analyticsapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Config holds Analytics client configuration.
type Config struct {
	// Endpoint is the Analytics API endpoint (e.g., "https://analytics.hanzo.ai")
	Endpoint string

	// WebsiteID is the website identifier
	WebsiteID string

	// BatchSize is the number of events to batch before sending (default: 50)
	BatchSize int

	// FlushInterval is how often to flush batched events (default: 10s)
	FlushInterval time.Duration

	// Timeout is the HTTP request timeout (default: 5s)
	Timeout time.Duration

	// Async enables asynchronous event sending (default: true)
	Async bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Endpoint:      "https://analytics.hanzo.ai",
		BatchSize:     50,
		FlushInterval: 10 * time.Second,
		Timeout:       5 * time.Second,
		Async:         true,
	}
}

// PageViewEvent represents a page view event.
type PageViewEvent struct {
	URL       string `json:"url"`
	Title     string `json:"title,omitempty"`
	Referrer  string `json:"referrer,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
	Language  string `json:"language,omitempty"`
	Screen    string `json:"screen,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// CustomEvent represents a custom analytics event.
type CustomEvent struct {
	Name      string                 `json:"name"`
	Data      map[string]interface{} `json:"data,omitempty"`
	URL       string                 `json:"url,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
}

// IdentifyEvent represents a session identification event.
type IdentifyEvent struct {
	SessionID string                 `json:"session_id"`
	Data      map[string]interface{} `json:"data"`
}

// analyticsEvent is the internal event representation.
type analyticsEvent struct {
	Type    string      `json:"type"` // "event" or "identify"
	Payload interface{} `json:"payload"`
}

// Client is the Analytics API client.
type Client struct {
	config     *Config
	httpClient *http.Client
	eventQueue chan *analyticsEvent
	wg         sync.WaitGroup
	closed     bool
	mu         sync.RWMutex
}

// NewClient creates a new Analytics client.
func NewClient(config *Config) *Client {
	if config == nil {
		config = DefaultConfig()
	}

	if config.BatchSize == 0 {
		config.BatchSize = 50
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = 10 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}

	client := &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		eventQueue: make(chan *analyticsEvent, config.BatchSize*10),
	}

	if config.Async {
		client.wg.Add(1)
		go client.processBatch()
	}

	return client
}

// TrackPageView sends a page view event.
func (c *Client) TrackPageView(event *PageViewEvent) error {
	return c.sendEvent(&analyticsEvent{
		Type: "event",
		Payload: map[string]interface{}{
			"website":  c.config.WebsiteID,
			"url":      event.URL,
			"title":    event.Title,
			"referrer": event.Referrer,
			"hostname": event.Hostname,
			"language": event.Language,
			"screen":   event.Screen,
		},
	})
}

// TrackEvent sends a custom event.
func (c *Client) TrackEvent(event *CustomEvent) error {
	return c.sendEvent(&analyticsEvent{
		Type: "event",
		Payload: map[string]interface{}{
			"website": c.config.WebsiteID,
			"name":    event.Name,
			"data":    event.Data,
			"url":     event.URL,
		},
	})
}

// Identify sends session identification data.
func (c *Client) Identify(event *IdentifyEvent) error {
	return c.sendEvent(&analyticsEvent{
		Type: "identify",
		Payload: map[string]interface{}{
			"website": c.config.WebsiteID,
			"session": event.SessionID,
			"data":    event.Data,
		},
	})
}

// TrackCommerceEvent sends a commerce-specific event.
func (c *Client) TrackCommerceEvent(eventName string, orderID string, total float64, properties map[string]interface{}) error {
	data := map[string]interface{}{
		"order_id": orderID,
		"total":    total,
	}
	for k, v := range properties {
		data[k] = v
	}

	return c.TrackEvent(&CustomEvent{
		Name: eventName,
		Data: data,
	})
}

// sendEvent queues or sends an event.
func (c *Client) sendEvent(event *analyticsEvent) error {
	c.mu.RLock()
	async := c.config.Async && !c.closed
	c.mu.RUnlock()

	if async {
		select {
		case c.eventQueue <- event:
			return nil
		default:
			// Queue full, send synchronously
			return c.sendEvents([]*analyticsEvent{event})
		}
	}

	return c.sendEvents([]*analyticsEvent{event})
}

// sendEvents sends events to the Analytics API.
func (c *Client) sendEvents(events []*analyticsEvent) error {
	if len(events) == 0 {
		return nil
	}

	for _, event := range events {
		body, err := json.Marshal(event.Payload)
		if err != nil {
			return fmt.Errorf("failed to marshal event: %w", err)
		}

		endpoint := c.config.Endpoint + "/api/send"

		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodPost,
			endpoint,
			bytes.NewReader(body),
		)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send event: %w", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("analytics API error: status %d", resp.StatusCode)
		}
	}

	return nil
}

// processBatch processes the event queue in batches.
func (c *Client) processBatch() {
	defer c.wg.Done()

	batch := make([]*analyticsEvent, 0, c.config.BatchSize)
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
	batch := make([]*analyticsEvent, 0, c.config.BatchSize)
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
