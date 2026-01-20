// Package events provides unified event storage via ClickHouse.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/hanzoai/commerce/db"
)

// DatastoreWriter writes events directly to ClickHouse.
// This is the unified storage that both Insights and Analytics read from.
type DatastoreWriter struct {
	datastore db.Datastore
	config    *DatastoreWriterConfig
	eventCh   chan *RawEvent
	wg        sync.WaitGroup
	closed    bool
	mu        sync.RWMutex
}

// DatastoreWriterConfig configures the datastore writer.
type DatastoreWriterConfig struct {
	// BatchSize is the number of events to batch before writing
	BatchSize int

	// FlushInterval is how often to flush partial batches
	FlushInterval time.Duration

	// AsyncInsert uses ClickHouse async insert (faster, less guaranteed)
	AsyncInsert bool

	// BufferSize is the event channel buffer size
	BufferSize int
}

// DefaultDatastoreWriterConfig returns sensible defaults.
func DefaultDatastoreWriterConfig() *DatastoreWriterConfig {
	return &DatastoreWriterConfig{
		BatchSize:     500,
		FlushInterval: 5 * time.Second,
		AsyncInsert:   true,
		BufferSize:    10000,
	}
}

// RawEvent is the unified event format written to ClickHouse.
type RawEvent struct {
	// Core identifiers
	DistinctID string `json:"distinct_id"`
	Event      string `json:"event"`

	// Organization
	OrganizationID string `json:"organization_id"`
	ProjectID      string `json:"project_id,omitempty"`

	// Session
	SessionID string `json:"session_id,omitempty"`
	VisitID   string `json:"visit_id,omitempty"`

	// Properties
	Properties       map[string]interface{} `json:"properties,omitempty"`
	PersonProperties map[string]interface{} `json:"person_properties,omitempty"`

	// Group
	GroupType       string                 `json:"group_type,omitempty"`
	GroupKey        string                 `json:"group_key,omitempty"`
	GroupProperties map[string]interface{} `json:"group_properties,omitempty"`

	// Web analytics
	URL            string `json:"url,omitempty"`
	URLPath        string `json:"url_path,omitempty"`
	Referrer       string `json:"referrer,omitempty"`
	ReferrerDomain string `json:"referrer_domain,omitempty"`
	Hostname       string `json:"hostname,omitempty"`

	// Device
	Browser        string `json:"browser,omitempty"`
	BrowserVersion string `json:"browser_version,omitempty"`
	OS             string `json:"os,omitempty"`
	OSVersion      string `json:"os_version,omitempty"`
	Device         string `json:"device,omitempty"`
	DeviceType     string `json:"device_type,omitempty"`
	Screen         string `json:"screen,omitempty"`
	Language       string `json:"language,omitempty"`

	// Geo
	Country string `json:"country,omitempty"`
	Region  string `json:"region,omitempty"`
	City    string `json:"city,omitempty"`

	// UTM
	UTMSource   string `json:"utm_source,omitempty"`
	UTMMedium   string `json:"utm_medium,omitempty"`
	UTMCampaign string `json:"utm_campaign,omitempty"`
	UTMContent  string `json:"utm_content,omitempty"`
	UTMTerm     string `json:"utm_term,omitempty"`

	// Click IDs
	GCLID  string `json:"gclid,omitempty"`
	FBCLID string `json:"fbclid,omitempty"`
	MSCLID string `json:"msclid,omitempty"`

	// Request
	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`

	// Commerce
	OrderID   string  `json:"order_id,omitempty"`
	ProductID string  `json:"product_id,omitempty"`
	CartID    string  `json:"cart_id,omitempty"`
	Revenue   float64 `json:"revenue,omitempty"`
	Quantity  int     `json:"quantity,omitempty"`

	// AST/Structured Data (astley.js support)
	// JSON-LD context for semantic data
	ASTContext string `json:"@context,omitempty"`
	ASTType    string `json:"@type,omitempty"`

	// Page structure elements
	PageTitle       string `json:"page_title,omitempty"`
	PageDescription string `json:"page_description,omitempty"`
	PageType        string `json:"page_type,omitempty"` // hero, block, cta, etc.

	// Element interaction tracking
	ElementID       string `json:"element_id,omitempty"`
	ElementType     string `json:"element_type,omitempty"`     // button, link, form, section
	ElementSelector string `json:"element_selector,omitempty"` // CSS selector
	ElementText     string `json:"element_text,omitempty"`     // visible text
	ElementHref     string `json:"element_href,omitempty"`     // link destination

	// Section tracking (astley.js WebsiteSection)
	SectionName string `json:"section_name,omitempty"`
	SectionType string `json:"section_type,omitempty"` // hero, block, cta
	SectionID   string `json:"section_id,omitempty"`

	// Component hierarchy
	ComponentPath string `json:"component_path,omitempty"` // e.g., "header/nav/menu/item"
	ComponentData string `json:"component_data,omitempty"` // JSON blob of component props

	// AI/Cloud events
	ModelProvider string  `json:"model_provider,omitempty"` // openai, anthropic, etc.
	ModelName     string  `json:"model_name,omitempty"`     // gpt-4, claude-3, etc.
	TokenCount    int     `json:"token_count,omitempty"`
	TokenPrice    float64 `json:"token_price,omitempty"`
	PromptTokens  int     `json:"prompt_tokens,omitempty"`
	OutputTokens  int     `json:"output_tokens,omitempty"`

	// Timestamps
	Timestamp time.Time `json:"timestamp"`
	SentAt    time.Time `json:"sent_at,omitempty"`

	// Library
	Lib        string `json:"lib,omitempty"`
	LibVersion string `json:"lib_version,omitempty"`
}

// NewDatastoreWriter creates a new datastore writer.
func NewDatastoreWriter(datastore db.Datastore, config *DatastoreWriterConfig) *DatastoreWriter {
	if config == nil {
		config = DefaultDatastoreWriterConfig()
	}

	w := &DatastoreWriter{
		datastore: datastore,
		config:    config,
		eventCh:   make(chan *RawEvent, config.BufferSize),
	}

	// Start background writer
	w.wg.Add(1)
	go w.processEvents()

	return w
}

// Write queues an event for writing to ClickHouse.
func (w *DatastoreWriter) Write(event *RawEvent) error {
	w.mu.RLock()
	if w.closed {
		w.mu.RUnlock()
		return fmt.Errorf("writer is closed")
	}
	w.mu.RUnlock()

	// Set defaults
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.SentAt.IsZero() {
		event.SentAt = time.Now()
	}
	if event.Lib == "" {
		event.Lib = "hanzo-commerce"
	}

	select {
	case w.eventCh <- event:
		return nil
	default:
		// Channel full - write synchronously
		return w.writeBatch([]*RawEvent{event})
	}
}

// processEvents runs in background, batching and writing events.
func (w *DatastoreWriter) processEvents() {
	defer w.wg.Done()

	batch := make([]*RawEvent, 0, w.config.BatchSize)
	ticker := time.NewTicker(w.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-w.eventCh:
			if !ok {
				// Channel closed - flush remaining
				if len(batch) > 0 {
					w.writeBatch(batch)
				}
				return
			}

			batch = append(batch, event)
			if len(batch) >= w.config.BatchSize {
				w.writeBatch(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				w.writeBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

// writeBatch writes a batch of events to ClickHouse.
func (w *DatastoreWriter) writeBatch(events []*RawEvent) error {
	if len(events) == 0 || w.datastore == nil {
		return nil
	}

	ctx := context.Background()

	if w.config.AsyncInsert {
		// Use async insert for each event (fire-and-forget)
		for _, event := range events {
			if err := w.writeEventAsync(ctx, event); err != nil {
				// Log but continue - async insert is best-effort
				continue
			}
		}
		return nil
	}

	// Use batch insert for guaranteed delivery
	batch, err := w.datastore.PrepareBatch(ctx, `INSERT INTO commerce.events`)
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}
	defer batch.Close()

	for _, event := range events {
		propsJSON, _ := json.Marshal(event.Properties)
		personPropsJSON, _ := json.Marshal(event.PersonProperties)
		groupPropsJSON, _ := json.Marshal(event.GroupProperties)

		if err := batch.Append(
			// event_id generated by ClickHouse
			event.DistinctID,
			event.Event,
			event.Timestamp,
			event.SentAt,
			time.Now(), // created_at
			event.OrganizationID,
			event.ProjectID,
			event.SessionID,
			event.VisitID,
			string(propsJSON),
			string(personPropsJSON),
			event.GroupType,
			event.GroupKey,
			string(groupPropsJSON),
			event.URL,
			event.URLPath,
			event.Referrer,
			event.ReferrerDomain,
			event.Hostname,
			event.Browser,
			event.BrowserVersion,
			event.OS,
			event.OSVersion,
			event.Device,
			event.DeviceType,
			event.Screen,
			event.Language,
			event.Country,
			event.Region,
			event.City,
			event.UTMSource,
			event.UTMMedium,
			event.UTMCampaign,
			event.UTMContent,
			event.UTMTerm,
			event.GCLID,
			event.FBCLID,
			event.MSCLID,
			event.IP,
			event.UserAgent,
			event.OrderID,
			event.ProductID,
			event.CartID,
			event.Revenue,
			event.Quantity,
			// AST/Structured Data
			event.ASTContext,
			event.ASTType,
			event.PageTitle,
			event.PageDescription,
			event.PageType,
			event.ElementID,
			event.ElementType,
			event.ElementSelector,
			event.ElementText,
			event.ElementHref,
			event.SectionName,
			event.SectionType,
			event.SectionID,
			event.ComponentPath,
			event.ComponentData,
			// AI/Cloud
			event.ModelProvider,
			event.ModelName,
			event.TokenCount,
			event.TokenPrice,
			event.PromptTokens,
			event.OutputTokens,
			// Library
			event.Lib,
			event.LibVersion,
		); err != nil {
			batch.Abort()
			return fmt.Errorf("failed to append to batch: %w", err)
		}
	}

	return batch.Send()
}

// writeEventAsync writes a single event using async insert.
func (w *DatastoreWriter) writeEventAsync(ctx context.Context, event *RawEvent) error {
	propsJSON, _ := json.Marshal(event.Properties)
	personPropsJSON, _ := json.Marshal(event.PersonProperties)
	groupPropsJSON, _ := json.Marshal(event.GroupProperties)

	query := `INSERT INTO commerce.events (
		distinct_id, event, timestamp, sent_at, created_at,
		organization_id, project_id, session_id, visit_id,
		properties, person_properties, group_type, group_key, group_properties,
		url, url_path, referrer, referrer_domain, hostname,
		browser, browser_version, os, os_version, device, device_type, screen, language,
		country, region, city,
		utm_source, utm_medium, utm_campaign, utm_content, utm_term,
		gclid, fbclid, msclid,
		ip, user_agent,
		order_id, product_id, cart_id, revenue, quantity,
		ast_context, ast_type, page_title, page_description, page_type,
		element_id, element_type, element_selector, element_text, element_href,
		section_name, section_type, section_id,
		component_path, component_data,
		model_provider, model_name, token_count, token_price, prompt_tokens, output_tokens,
		lib, lib_version
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	return w.datastore.AsyncInsert(ctx, query, false,
		event.DistinctID,
		event.Event,
		event.Timestamp,
		event.SentAt,
		time.Now(),
		event.OrganizationID,
		event.ProjectID,
		event.SessionID,
		event.VisitID,
		string(propsJSON),
		string(personPropsJSON),
		event.GroupType,
		event.GroupKey,
		string(groupPropsJSON),
		event.URL,
		event.URLPath,
		event.Referrer,
		event.ReferrerDomain,
		event.Hostname,
		event.Browser,
		event.BrowserVersion,
		event.OS,
		event.OSVersion,
		event.Device,
		event.DeviceType,
		event.Screen,
		event.Language,
		event.Country,
		event.Region,
		event.City,
		event.UTMSource,
		event.UTMMedium,
		event.UTMCampaign,
		event.UTMContent,
		event.UTMTerm,
		event.GCLID,
		event.FBCLID,
		event.MSCLID,
		event.IP,
		event.UserAgent,
		event.OrderID,
		event.ProductID,
		event.CartID,
		event.Revenue,
		event.Quantity,
		// AST/Structured Data
		event.ASTContext,
		event.ASTType,
		event.PageTitle,
		event.PageDescription,
		event.PageType,
		event.ElementID,
		event.ElementType,
		event.ElementSelector,
		event.ElementText,
		event.ElementHref,
		event.SectionName,
		event.SectionType,
		event.SectionID,
		event.ComponentPath,
		event.ComponentData,
		// AI/Cloud
		event.ModelProvider,
		event.ModelName,
		event.TokenCount,
		event.TokenPrice,
		event.PromptTokens,
		event.OutputTokens,
		// Library
		event.Lib,
		event.LibVersion,
	)
}

// Flush writes all pending events immediately.
func (w *DatastoreWriter) Flush() error {
	// Drain the channel
	batch := make([]*RawEvent, 0, w.config.BatchSize)
	for {
		select {
		case event := <-w.eventCh:
			batch = append(batch, event)
		default:
			// Channel empty
			if len(batch) > 0 {
				return w.writeBatch(batch)
			}
			return nil
		}
	}
}

// Close gracefully shuts down the writer.
func (w *DatastoreWriter) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	w.mu.Unlock()

	close(w.eventCh)
	w.wg.Wait()
	return nil
}

// EnsureSchema creates the required tables in ClickHouse.
func EnsureSchema(ctx context.Context, datastore db.Datastore) error {
	// Create database if needed
	if err := datastore.Exec(ctx, `CREATE DATABASE IF NOT EXISTS commerce`); err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	// Execute schema (split by semicolons and execute each statement)
	// Note: In production, you'd want proper migration management
	if err := datastore.Exec(ctx, Schema); err != nil {
		// Schema might already exist, that's OK
		// Log but don't fail
	}

	return nil
}
