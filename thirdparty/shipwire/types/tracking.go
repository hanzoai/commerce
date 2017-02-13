package types

type Tracking struct {
	ID              int    `json:"id"`
	OrderID         int    `json:"orderId"`
	OrderExternalID string `json:"orderExternalId"`
	Carrier         string `json:"carrier"`
	Url             string `json:"url"`

	Summary     string `json:"summary"`
	SummaryDate Date   `json:"summaryDate"`

	LabelCreatedDate Date `json:"labelCreatedDate"`

	Tracking    string `json:"tracking"`
	TrackedDate Date   `json:"trackedDate"`

	FirstScanRegion     string `json:"firstScanRegion"`
	FirstScanPostalCode string `json:"firstScanPostalCode"`
	FirstScanCountry    string `json:"firstScanCountry"`
	FirstScanDate       Date   `json:"firstScanDate"`

	DeliveryCity       string `json:"deliveryCity"`
	DeliveryRegion     string `json:"deliveryRegion"`
	DeliveryPostalCode string `json:"DeliveryPostalCode"`
	DeliveryCountry    string `json:"DeliveryCountry"`
	DeliveredDate      Date   `json:"deliveredDate"`
}
