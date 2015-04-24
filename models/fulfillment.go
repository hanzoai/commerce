package models

type FulfillmentStatus string

const (
	FulfillmentUnfulfilled FulfillmentStatus = "unfulfilled"
	FulfillmentFulfilled                     = "fulfilled"
	FulfillmentProcessing                    = "processing"
	FulfillmentFailed                        = "failed"
)

type Fulfillment struct {
	Courier        string `json:"courier"`
	TrackingNumber string `json:"trackingNumber"`
}
