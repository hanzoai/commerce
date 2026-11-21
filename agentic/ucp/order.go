// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Order is the current-state snapshot of a placed order (schemas/shopping/
// order.json; shopping/order, "Operations"). The same entity answers Get Order
// and is the payload of every order webhook.
type Order struct {
	UCP          *Meta             `json:"ucp,omitempty"`
	ID           string            `json:"id"`
	Label        string            `json:"label,omitempty"`
	CheckoutID   string            `json:"checkout_id"`
	PermalinkURL string            `json:"permalink_url"`
	LineItems    []OrderLine       `json:"line_items"`
	Fulfillment  OrderFulfilment   `json:"fulfillment"`
	Adjustments  []Adjustment      `json:"adjustments,omitempty"`
	Currency     string            `json:"currency"`
	Totals       []Total           `json:"totals"`
	Policies     json.RawMessage   `json:"policies,omitempty"`
	Messages     []Message         `json:"messages,omitempty"`
	Attribution  map[string]string `json:"attribution,omitempty"`
}

// MarshalJSON writes adjustments when the slice is not nil, so an order that
// has none yet can state an empty list.
func (o Order) MarshalJSON() ([]byte, error) {
	type plain Order
	w := struct {
		plain
		Adjustments *[]Adjustment `json:"adjustments,omitempty"`
	}{plain: plain(o)}
	if o.Adjustments != nil {
		w.Adjustments = &o.Adjustments
	}
	return json.Marshal(w)
}

// UnmarshalJSON reads an order, keeping an empty adjustments list.
func (o *Order) UnmarshalJSON(b []byte) error {
	type plain Order
	w := struct {
		*plain
		Adjustments *[]Adjustment `json:"adjustments"`
	}{plain: (*plain)(o)}
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	o.Adjustments = nil
	if w.Adjustments != nil {
		o.Adjustments = *w.Adjustments
		if o.Adjustments == nil {
			o.Adjustments = []Adjustment{}
		}
	}
	return nil
}

// OrderLine is a line item of an order (schemas/shopping/types/order_line_item.json).
type OrderLine struct {
	ID       string        `json:"id"`
	Item     Item          `json:"item"`
	Quantity OrderQuantity `json:"quantity"`
	Totals   []Total       `json:"totals"`
	Status   string        `json:"status"`
	ParentID string        `json:"parent_id,omitempty"`
}

// OrderQuantity counts steps of the item's sale basis: what was ordered, what
// the line totals now, and what has been fulfilled.
type OrderQuantity struct {
	Original  *int64 `json:"original,omitempty"`
	Total     int64  `json:"total"`
	Fulfilled int64  `json:"fulfilled"`
}

// Line statuses of an order line (order_line_item.json, status).
const (
	LineProcessing = "processing"
	LinePartial    = "partial"
	LineFulfilled  = "fulfilled"
	LineRemoved    = "removed"
)

// Status derives a line's status from its counts ("Status Derivation").
func (q OrderQuantity) Status() string {
	switch {
	case q.Total == 0:
		return LineRemoved
	case q.Fulfilled == q.Total:
		return LineFulfilled
	case q.Fulfilled > 0:
		return LinePartial
	}
	return LineProcessing
}

// OrderFulfilment holds the expectations buyers are promised and the
// append-only log of what happened.
type OrderFulfilment struct {
	Expectations []Expectation `json:"expectations,omitempty"`
	Events       []Event       `json:"events,omitempty"`
}

// Expectation is a buyer-facing grouping of items (types/expectation.json).
type Expectation struct {
	ID            string    `json:"id"`
	LineItems     []LineQty `json:"line_items"`
	MethodType    string    `json:"method_type"`
	Destination   Address   `json:"destination"`
	Description   string    `json:"description,omitempty"`
	FulfillableOn string    `json:"fulfillable_on,omitempty"`
}

// LineQty names a line and a count of its steps.
type LineQty struct {
	ID       string   `json:"id"`
	Quantity int64    `json:"quantity"`
	Measure  *Measure `json:"measure,omitempty"`
}

// Event is one fulfillment event (types/fulfillment_event.json).
type Event struct {
	ID             string    `json:"id"`
	OccurredAt     string    `json:"occurred_at"`
	Type           string    `json:"type"`
	LineItems      []LineQty `json:"line_items"`
	TrackingNumber string    `json:"tracking_number,omitempty"`
	TrackingURL    string    `json:"tracking_url,omitempty"`
	Carrier        string    `json:"carrier,omitempty"`
	Description    string    `json:"description,omitempty"`
}

// Adjustment is a post-order event independent of fulfillment (types/adjustment.json).
type Adjustment struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	OccurredAt  string    `json:"occurred_at"`
	Status      string    `json:"status"`
	LineItems   []LineQty `json:"line_items,omitempty"`
	Totals      []Total   `json:"totals,omitempty"`
	Description string    `json:"description,omitempty"`
}

// Check validates an order snapshot: the required members, the status of each
// line against its counts, and the totals.
func (o *Order) Check() error {
	var errs []string
	bad := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }
	if o.ID == "" || o.CheckoutID == "" || o.PermalinkURL == "" || o.Currency == "" {
		bad("id, checkout_id, permalink_url and currency are required")
	}
	if o.LineItems == nil {
		bad("line_items is required")
	}
	for i, l := range o.LineItems {
		if l.ID == "" || l.Item.ID == "" || l.Item.Title == "" || l.Item.Price == nil {
			bad("line_items[%d] needs id and an item with id, title and price", i)
		}
		if l.Status != l.Quantity.Status() {
			bad("line_items[%d].status is %q but its counts derive %q", i, l.Status, l.Quantity.Status())
		}
	}
	if err := CheckTotals(o.Totals); err != nil {
		bad("%v", err)
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}
