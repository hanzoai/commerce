package models

import (
	"time"

	"crowdstart.com/models/types/currency"
)

type FulfillmentStatus string

const (
	FulfillmentUnfulfilled FulfillmentStatus = "unfulfilled"
	FulfillmentShipped                       = "shipped"
	FulfillmentProcessing                    = "processing"
	FulFillmentCancelled                     = "cancelled"
)

type Fulfillment struct {
	Carrier        string         `json:"carrier,omitempty"`
	Service        string         `json:"service,omitempty"`
	TrackingNumber string         `json:"trackingNumber,omitempty"`
	CreatedAt      time.Time      `json:"createdAt,omitempty"`
	ShippedAt      time.Time      `json:"shippedAt,omitempty"`
	Cost           currency.Cents `json:"cost,omitempty"`
}
