package models

import (
	"time"

	"hanzo.io/models/types/currency"
)

type FulfillmentStatus string
type Integration string

const (
	FulfillmentUnfulfilled FulfillmentStatus = "unfulfilled"
	FulfillmentLabelled    FulfillmentStatus = "labelled"
	FulfillmentProcessing  FulfillmentStatus = "processing"
	FulfillmentShipped     FulfillmentStatus = "shipped"
	FulfillmentDelivered   FulfillmentStatus = "delivered"
	FulFillmentCancelled   FulfillmentStatus = "cancelled"
)

const (
	Shipwire Integration = "shipwire"
)

type Fulfillment struct {
	Integration    Integration    `json:"type,omitempty"`
	ExternalId     string         `json:"externalId,omitempty"`
	Carrier        string         `json:"carrier,omitempty"`
	Summary        string         `json:"summary,omitempty"`
	Service        string         `json:"service,omitempty"`
	TrackingNumber string         `json:"trackingNumber,omitempty"`
	CreatedAt      time.Time      `json:"createdAt,omitempty"`
	ShippedAt      time.Time      `json:"shippedAt,omitempty"`
	DeliveredAt    time.Time      `json:"deliveredAt,omitempty"`
	Cost           currency.Cents `json:"cost,omitempty"`
}
