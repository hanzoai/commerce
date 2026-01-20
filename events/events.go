// Package events provides unified event storage for Commerce.
//
// Events are written to a shared ClickHouse instance (Hanzo Datastore) that
// both Insights (PostHog) and Analytics (Umami) can query from. This ensures
// a single source of truth for all analytics data.
//
// Architecture:
//
//	Commerce App
//	    │
//	    ▼
//	┌─────────────────┐
//	│  Event Emitter  │
//	└────────┬────────┘
//	         │
//	         ▼
//	┌─────────────────┐
//	│   ClickHouse    │ ◄── Single source of truth
//	│   (Datastore)   │
//	└────────┬────────┘
//	         │
//	    ┌────┴────┐
//	    ▼         ▼
//	Insights  Analytics
//	(PostHog) (Umami)
//
// Optional: HTTP forwarding to external Insights/Analytics instances is
// still supported for backwards compatibility or hybrid deployments.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/insights"
	"github.com/hanzoai/commerce/integrations/analyticsapi"
)

// Config holds event emitter configuration.
type Config struct {
	// Datastore configuration (primary - writes to ClickHouse)
	DatastoreEnabled bool

	// Insights (PostHog) HTTP forwarding (optional)
	InsightsEndpoint string
	InsightsAPIKey   string
	InsightsEnabled  bool

	// Analytics (Umami) HTTP forwarding (optional)
	AnalyticsEndpoint  string
	AnalyticsWebsiteID string
	AnalyticsEnabled   bool
}

// Emitter sends events to the unified datastore and optionally to HTTP endpoints.
type Emitter struct {
	config          *Config
	datastoreWriter *DatastoreWriter
	insightsClient  *insights.Client
	analyticsClient *analyticsapi.Client
	mu              sync.RWMutex
}

// NewEmitter creates a new event emitter.
func NewEmitter(config *Config) *Emitter {
	return &Emitter{
		config: config,
	}
}

// NewEmitterWithDatastore creates an emitter with a datastore connection.
func NewEmitterWithDatastore(config *Config, datastore db.Datastore) *Emitter {
	emitter := &Emitter{
		config: config,
	}

	// Initialize datastore writer (primary storage)
	if config.DatastoreEnabled && datastore != nil {
		// Ensure ClickHouse schema exists
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := EnsureSchema(ctx, datastore); err != nil {
			// Log but don't fail - schema might already exist
			fmt.Printf("Warning: failed to ensure events schema: %v\n", err)
		}

		emitter.datastoreWriter = NewDatastoreWriter(datastore, nil)
	}

	// Initialize Insights client for HTTP forwarding (optional)
	if config.InsightsEnabled && config.InsightsEndpoint != "" && config.InsightsAPIKey != "" {
		emitter.insightsClient = insights.NewClient(&insights.Config{
			Endpoint: config.InsightsEndpoint,
			APIKey:   config.InsightsAPIKey,
		})
	}

	// Initialize Analytics client for HTTP forwarding (optional)
	if config.AnalyticsEnabled && config.AnalyticsEndpoint != "" && config.AnalyticsWebsiteID != "" {
		emitter.analyticsClient = analyticsapi.NewClient(&analyticsapi.Config{
			Endpoint:  config.AnalyticsEndpoint,
			WebsiteID: config.AnalyticsWebsiteID,
		})
	}

	return emitter
}

// SetDatastore sets the datastore writer (can be called after creation).
func (e *Emitter) SetDatastore(datastore db.Datastore) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.config.DatastoreEnabled && datastore != nil {
		e.datastoreWriter = NewDatastoreWriter(datastore, nil)
	}
}

// Order represents a Commerce order for event emission.
type Order struct {
	ID         string
	UserID     string
	Email      string
	Total      float64
	Currency   string
	Items      []OrderItem
	Status     string
	OrgID      string
}

// OrderItem represents an item in an order.
type OrderItem struct {
	ProductID   string
	ProductName string
	SKU         string
	Quantity    int
	Price       float64
}

// User represents a Commerce user for event emission.
type User struct {
	ID        string
	Email     string
	Name      string
	OrgID     string
	CreatedAt string
}

// Product represents a Commerce product for event emission.
type Product struct {
	ID       string
	Name     string
	SKU      string
	Price    float64
	Category string
	OrgID    string
}

// PageView represents a page view event.
type PageView struct {
	URL       string
	Title     string
	Referrer  string
	UserID    string
	SessionID string
	OrgID     string
	IP        string
	UserAgent string
	Language  string
	Screen    string
}

// EmitOrderCompleted sends order completed events.
func (e *Emitter) EmitOrderCompleted(ctx context.Context, order *Order) error {
	var errs []error

	// Convert items for properties
	items := make([]map[string]interface{}, len(order.Items))
	var totalQuantity int
	for i, item := range order.Items {
		items[i] = map[string]interface{}{
			"product_id":   item.ProductID,
			"product_name": item.ProductName,
			"sku":          item.SKU,
			"quantity":     item.Quantity,
			"price":        item.Price,
		}
		totalQuantity += item.Quantity
	}

	itemsJSON, _ := json.Marshal(items)

	// Write to datastore (primary)
	if e.datastoreWriter != nil {
		if err := e.datastoreWriter.Write(&RawEvent{
			DistinctID:     order.UserID,
			Event:          StandardEvents.OrderCompleted,
			OrganizationID: order.OrgID,
			OrderID:        order.ID,
			Revenue:        order.Total,
			Quantity:       totalQuantity,
			Properties: map[string]interface{}{
				"currency":   order.Currency,
				"items":      string(itemsJSON),
				"item_count": len(order.Items),
				"status":     order.Status,
				"email":      order.Email,
			},
			Timestamp: time.Now(),
		}); err != nil {
			errs = append(errs, fmt.Errorf("datastore: %w", err))
		}
	}

	// Forward to Insights HTTP (optional)
	if e.insightsClient != nil {
		if err := e.insightsClient.CaptureOrderEvent(
			order.UserID,
			insights.StandardEventNames.OrderCompleted,
			order.ID,
			order.Total,
			items,
		); err != nil {
			errs = append(errs, fmt.Errorf("insights: %w", err))
		}

		if order.OrgID != "" {
			e.insightsClient.GroupIdentify("organization", order.OrgID, map[string]interface{}{
				"name": order.OrgID,
			})
		}
	}

	// Forward to Analytics HTTP (optional)
	if e.analyticsClient != nil {
		if err := e.analyticsClient.TrackCommerceEvent(
			"order_completed",
			order.ID,
			order.Total,
			map[string]interface{}{
				"currency":   order.Currency,
				"item_count": len(order.Items),
				"user_id":    order.UserID,
			},
		); err != nil {
			errs = append(errs, fmt.Errorf("analytics: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("event emission errors: %v", errs)
	}
	return nil
}

// EmitOrderRefunded sends order refunded events.
func (e *Emitter) EmitOrderRefunded(ctx context.Context, order *Order, refundAmount float64) error {
	var errs []error

	// Write to datastore (primary)
	if e.datastoreWriter != nil {
		if err := e.datastoreWriter.Write(&RawEvent{
			DistinctID:     order.UserID,
			Event:          StandardEvents.OrderRefunded,
			OrganizationID: order.OrgID,
			OrderID:        order.ID,
			Revenue:        -refundAmount, // Negative for refunds
			Properties: map[string]interface{}{
				"refund_amount":  refundAmount,
				"original_total": order.Total,
			},
			Timestamp: time.Now(),
		}); err != nil {
			errs = append(errs, fmt.Errorf("datastore: %w", err))
		}
	}

	// Forward to Insights HTTP (optional)
	if e.insightsClient != nil {
		if err := e.insightsClient.Capture(&insights.Event{
			DistinctID: order.UserID,
			Event:      insights.StandardEventNames.OrderRefunded,
			Properties: map[string]interface{}{
				"order_id":       order.ID,
				"refund_amount":  refundAmount,
				"original_total": order.Total,
			},
		}); err != nil {
			errs = append(errs, fmt.Errorf("insights: %w", err))
		}
	}

	// Forward to Analytics HTTP (optional)
	if e.analyticsClient != nil {
		if err := e.analyticsClient.TrackCommerceEvent(
			"order_refunded",
			order.ID,
			refundAmount,
			nil,
		); err != nil {
			errs = append(errs, fmt.Errorf("analytics: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("event emission errors: %v", errs)
	}
	return nil
}

// EmitProductViewed sends product viewed events.
func (e *Emitter) EmitProductViewed(ctx context.Context, userID string, product *Product) error {
	var errs []error

	// Write to datastore (primary)
	if e.datastoreWriter != nil {
		if err := e.datastoreWriter.Write(&RawEvent{
			DistinctID:     userID,
			Event:          StandardEvents.ProductViewed,
			OrganizationID: product.OrgID,
			ProductID:      product.ID,
			Revenue:        product.Price,
			Properties: map[string]interface{}{
				"product_name": product.Name,
				"sku":          product.SKU,
				"category":     product.Category,
			},
			Timestamp: time.Now(),
		}); err != nil {
			errs = append(errs, fmt.Errorf("datastore: %w", err))
		}
	}

	// Forward to Insights HTTP (optional)
	if e.insightsClient != nil {
		if err := e.insightsClient.Capture(&insights.Event{
			DistinctID: userID,
			Event:      insights.StandardEventNames.ProductViewed,
			Properties: map[string]interface{}{
				"product_id":   product.ID,
				"product_name": product.Name,
				"sku":          product.SKU,
				"price":        product.Price,
				"category":     product.Category,
			},
		}); err != nil {
			errs = append(errs, fmt.Errorf("insights: %w", err))
		}
	}

	// Forward to Analytics HTTP (optional)
	if e.analyticsClient != nil {
		if err := e.analyticsClient.TrackEvent(&analyticsapi.CustomEvent{
			Name: "product_viewed",
			Data: map[string]interface{}{
				"product_id":   product.ID,
				"product_name": product.Name,
				"price":        product.Price,
			},
		}); err != nil {
			errs = append(errs, fmt.Errorf("analytics: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("event emission errors: %v", errs)
	}
	return nil
}

// EmitProductAdded sends product added to cart events.
func (e *Emitter) EmitProductAdded(ctx context.Context, userID string, product *Product, quantity int) error {
	var errs []error

	// Write to datastore (primary)
	if e.datastoreWriter != nil {
		if err := e.datastoreWriter.Write(&RawEvent{
			DistinctID:     userID,
			Event:          StandardEvents.ProductAdded,
			OrganizationID: product.OrgID,
			ProductID:      product.ID,
			Revenue:        product.Price * float64(quantity),
			Quantity:       quantity,
			Properties: map[string]interface{}{
				"product_name": product.Name,
				"sku":          product.SKU,
				"unit_price":   product.Price,
			},
			Timestamp: time.Now(),
		}); err != nil {
			errs = append(errs, fmt.Errorf("datastore: %w", err))
		}
	}

	// Forward to Insights HTTP (optional)
	if e.insightsClient != nil {
		if err := e.insightsClient.Capture(&insights.Event{
			DistinctID: userID,
			Event:      insights.StandardEventNames.ProductAdded,
			Properties: map[string]interface{}{
				"product_id":   product.ID,
				"product_name": product.Name,
				"sku":          product.SKU,
				"price":        product.Price,
				"quantity":     quantity,
			},
		}); err != nil {
			errs = append(errs, fmt.Errorf("insights: %w", err))
		}
	}

	// Forward to Analytics HTTP (optional)
	if e.analyticsClient != nil {
		if err := e.analyticsClient.TrackEvent(&analyticsapi.CustomEvent{
			Name: "product_added",
			Data: map[string]interface{}{
				"product_id": product.ID,
				"quantity":   quantity,
				"price":      product.Price,
			},
		}); err != nil {
			errs = append(errs, fmt.Errorf("analytics: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("event emission errors: %v", errs)
	}
	return nil
}

// EmitUserSignedUp sends user registration events.
func (e *Emitter) EmitUserSignedUp(ctx context.Context, user *User) error {
	var errs []error

	// Write to datastore (primary)
	if e.datastoreWriter != nil {
		if err := e.datastoreWriter.Write(&RawEvent{
			DistinctID:     user.ID,
			Event:          StandardEvents.SignedUp,
			OrganizationID: user.OrgID,
			PersonProperties: map[string]interface{}{
				"email":      user.Email,
				"name":       user.Name,
				"created_at": user.CreatedAt,
			},
			Timestamp: time.Now(),
		}); err != nil {
			errs = append(errs, fmt.Errorf("datastore: %w", err))
		}
	}

	// Forward to Insights HTTP (optional)
	if e.insightsClient != nil {
		if err := e.insightsClient.Capture(&insights.Event{
			DistinctID: user.ID,
			Event:      insights.StandardEventNames.SignedUp,
			Properties: map[string]interface{}{
				"email": user.Email,
			},
		}); err != nil {
			errs = append(errs, fmt.Errorf("insights: %w", err))
		}

		if err := e.insightsClient.Identify(user.ID, map[string]interface{}{
			"email":      user.Email,
			"name":       user.Name,
			"created_at": user.CreatedAt,
		}); err != nil {
			errs = append(errs, fmt.Errorf("insights identify: %w", err))
		}

		if user.OrgID != "" {
			e.insightsClient.GroupIdentify("organization", user.OrgID, nil)
		}
	}

	// Forward to Analytics HTTP (optional)
	if e.analyticsClient != nil {
		if err := e.analyticsClient.TrackEvent(&analyticsapi.CustomEvent{
			Name: "user_signed_up",
			Data: map[string]interface{}{
				"user_id": user.ID,
			},
		}); err != nil {
			errs = append(errs, fmt.Errorf("analytics: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("event emission errors: %v", errs)
	}
	return nil
}

// EmitPageView sends page view events.
func (e *Emitter) EmitPageView(ctx context.Context, pv *PageView) error {
	var errs []error

	userID := pv.UserID
	if userID == "" {
		userID = pv.SessionID
	}

	// Parse URL for components
	var urlPath, hostname, referrerDomain string
	if parsedURL, err := url.Parse(pv.URL); err == nil {
		urlPath = parsedURL.Path
		hostname = parsedURL.Host
	}
	if parsedRef, err := url.Parse(pv.Referrer); err == nil {
		referrerDomain = parsedRef.Host
	}

	// Write to datastore (primary)
	if e.datastoreWriter != nil {
		if err := e.datastoreWriter.Write(&RawEvent{
			DistinctID:     userID,
			Event:          StandardEvents.PageView,
			OrganizationID: pv.OrgID,
			SessionID:      pv.SessionID,
			URL:            pv.URL,
			URLPath:        urlPath,
			Hostname:       hostname,
			Referrer:       pv.Referrer,
			ReferrerDomain: referrerDomain,
			IP:             pv.IP,
			UserAgent:      pv.UserAgent,
			Language:       pv.Language,
			Screen:         pv.Screen,
			Properties: map[string]interface{}{
				"title": pv.Title,
			},
			Timestamp: time.Now(),
		}); err != nil {
			errs = append(errs, fmt.Errorf("datastore: %w", err))
		}
	}

	// Forward to Insights HTTP (optional)
	if e.insightsClient != nil {
		if err := e.insightsClient.CapturePageView(
			userID,
			pv.URL,
			pv.Title,
			pv.Referrer,
		); err != nil {
			errs = append(errs, fmt.Errorf("insights: %w", err))
		}
	}

	// Forward to Analytics HTTP (optional)
	if e.analyticsClient != nil {
		if err := e.analyticsClient.TrackPageView(&analyticsapi.PageViewEvent{
			URL:       pv.URL,
			Title:     pv.Title,
			Referrer:  pv.Referrer,
			SessionID: pv.SessionID,
			Hostname:  hostname,
			Language:  pv.Language,
			Screen:    pv.Screen,
		}); err != nil {
			errs = append(errs, fmt.Errorf("analytics: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("event emission errors: %v", errs)
	}
	return nil
}

// EmitRaw writes a raw event directly to the datastore.
func (e *Emitter) EmitRaw(ctx context.Context, event *RawEvent) error {
	if e.datastoreWriter == nil {
		return fmt.Errorf("datastore not configured")
	}
	return e.datastoreWriter.Write(event)
}

// Flush sends all queued events immediately.
func (e *Emitter) Flush() error {
	var errs []error

	if e.datastoreWriter != nil {
		if err := e.datastoreWriter.Flush(); err != nil {
			errs = append(errs, fmt.Errorf("datastore flush: %w", err))
		}
	}

	if e.insightsClient != nil {
		if err := e.insightsClient.Flush(); err != nil {
			errs = append(errs, fmt.Errorf("insights flush: %w", err))
		}
	}

	if e.analyticsClient != nil {
		if err := e.analyticsClient.Flush(); err != nil {
			errs = append(errs, fmt.Errorf("analytics flush: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("flush errors: %v", errs)
	}
	return nil
}

// Close gracefully shuts down all writers/clients.
func (e *Emitter) Close() error {
	var errs []error

	if e.datastoreWriter != nil {
		if err := e.datastoreWriter.Close(); err != nil {
			errs = append(errs, fmt.Errorf("datastore close: %w", err))
		}
	}

	if e.insightsClient != nil {
		if err := e.insightsClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("insights close: %w", err))
		}
	}

	if e.analyticsClient != nil {
		if err := e.analyticsClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("analytics close: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}
