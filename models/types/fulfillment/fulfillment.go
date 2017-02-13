package fulfillment

import (
	"time"

	"hanzo.io/models/types/currency"
)

type Status string
type Type string

const (
	Pending   Status = "pending"
	Processed Status = "processed"
	Canceled  Status = "canceled"
	Completed Status = "completed"
	Delivered Status = "delivered"
	Returned  Status = "returned"
	Submitted Status = "submitted"
	Held      Status = "held"
	Tracked   Status = "tracked"
)

const (
	Shipwire    Type = "shipwire"
	ShipStation Type = "shipstation"
	Manual      Type = "manual"
)

type Fulfillment struct {
	Type           Type           `json:"type"`
	Status         Status         `json:"status"`
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
