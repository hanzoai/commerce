package types

import "time"

type Tracking struct {
	ID              int    `json:"id"`
	OrderID         int    `json:"orderId"`
	OrderExternalID string `json:"orderExternalId"`
	Carrier         string `json:"carrier"`
	Url             string `json:"url"`

	Summary     string    `json:"summary"`
	SummaryDate time.Time `json:"summaryDate"`

	LabelCreatedDate time.Time `json:"labelCreatedDate"`

	Tracking    string    `json:"tracking"`
	TrackedDate time.Time `json:"trackedDate"`

	FirstScanRegion     string    `json:"firstScanRegion"`
	FirstScanPostalCode string    `json:"firstScanPostalCode"`
	FirstScanCountry    string    `json:"firstScanCountry"`
	FirstScanDate       time.Time `json:"firstScanDate"`

	DeliveryCity       string    `json:"deliveryCity"`
	DeliveryRegion     string    `json:"deliveryRegion"`
	DeliveryPostalCode string    `json:"DeliveryPostalCode"`
	DeliveryCountry    string    `json:"DeliveryCountry"`
	DeliveredDate      time.Time `json:"deliveredDate"`
}
