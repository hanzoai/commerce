package models

import "time"

type FullfillmentStatus string

const (
	FullfillmentUnfullfilled FullfillmentStatus = "unfullfilled"
	FullfillmentFullfilled                      = "fullfilled"
	FullfillmentProcessing                      = "processing"
	FullfillmentFailed                          = "failed"
)

type Fullfillment struct {
	CreatedAt      time.Time
	Courier        string
	TrackingNumber string
}
