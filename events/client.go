// Package events provides a thin HTTP client for the analytics collector.
//
// Commerce fires events via HTTP to the analytics-collector sidecar
// rather than writing directly to ClickHouse. This decouples analytics
// from the commerce binary.
package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client sends events to the analytics-collector via HTTP.
type Client struct {
	endpoint   string
	httpClient *http.Client
}

// NewClient creates a new analytics client.
// Endpoint should be the analytics-collector base URL (e.g., "http://analytics-collector.hanzo.svc:8091").
func NewClient(endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Order represents a Commerce order for event emission.
type Order struct {
	ID       string
	UserID   string
	Email    string
	Total    float64
	Currency string
	Items    []OrderItem
	Status   string
	OrgID    string
}

// OrderItem represents an item in an order.
type OrderItem struct {
	ProductID   string
	ProductName string
	SKU         string
	Quantity    int
	Price       float64
}

// EmitOrderCompleted sends an order completed event to the collector.
func (c *Client) EmitOrderCompleted(ctx context.Context, order *Order) error {
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

	return c.EmitRaw(ctx, map[string]interface{}{
		"event":           "order_completed",
		"distinct_id":     order.UserID,
		"organization_id": order.OrgID,
		"order_id":        order.ID,
		"revenue":         order.Total,
		"quantity":        totalQuantity,
		"properties": map[string]interface{}{
			"currency":   order.Currency,
			"items":      string(itemsJSON),
			"item_count": len(order.Items),
			"status":     order.Status,
			"email":      order.Email,
		},
	})
}

// EmitRaw sends a raw event to the collector.
func (c *Client) EmitRaw(ctx context.Context, event map[string]interface{}) error {
	if c.endpoint == "" {
		return nil // No collector configured, silently skip
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/event", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("collector error: status %d", resp.StatusCode)
	}
	return nil
}

// Flush is a no-op for the HTTP client (collector handles batching).
func (c *Client) Flush() error { return nil }

// Close is a no-op for the HTTP client.
func (c *Client) Close() error { return nil }
