package models

import (
	"time"

	"crowdstart.com/models/types/currency"
)

type FulfillmentStatus string

const (
	FulfillmentUnfulfilled FulfillmentStatus = "unfulfilled"
	FulfillmentFulfilled                     = "fulfilled"
	FulfillmentProcessing                    = "processing"
	FulfillmentFailed                        = "failed"
)

type Fulfillment struct {
	Carrier        string         `json:"carrier"`
	Service        string         `json:"service"`
	TrackingNumber string         `json:"trackingNumber"`
	CreatedAt      time.Time      `json:"createdAt"`
	ShippedAt      time.Time      `json:"shippedAt"`
	Cost           currency.Cents `json:"cost"`
}
