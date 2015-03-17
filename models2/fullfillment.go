package models

type FullfillmentStatus string

const (
	FullfillmentUnfullfilled FullfillmentStatus = "unfullfilled"
	FullfillmentFullfilled                      = "fullfilled"
	FullfillmentProcessing                      = "processing"
	FullfillmentFailed                          = "failed"
)

type Fullfillment struct {
	Courier        string `json:"courier"`
	TrackingNumber string `json:"trackingNumber"`
}
