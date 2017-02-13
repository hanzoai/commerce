package types

import (
	"time"

	"hanzo.io/util/json"
)

type ReturnRequest struct {
	ExternalID    string `json:"externalId"`
	OriginalOrder struct {
		ID int `json:"id"`
	} `json:"originalOrder"`
	Items []struct {
		SKU      string `json:"sku"`
		Quantity int    `json:"quantity"`
	} `json:"items"`
	Options struct {
		GeneratePrepaidLabel int    `json:"generatePrepaidLabel"`
		EmailCustomer        int    `json:"emailCustomer"`
		WarehouseID          int    `json:"warehouseId"`
		WarehouseExternalID  int    `json:"warehouseExternalId"`
		WarehouseRegion      string `json:"warehouseRegion"`
	} `json:"options"`
}

type Return struct {
	ID              int       `json:"id"`
	ExternalID      string    `json:"externalId"`
	TransactionID   string    `json:"transactionId"`
	ExpectedDate    time.Time `json:"expectedDate"`
	CommerceName    string    `json:"commerceName"`
	LastUpdatedDate time.Time `json:"lastUpdatedDate"`
	Status          string    `json:"status"`
	Holds           struct {
		ResourceLocation string `json:"resourceLocation"`
	} `json:"holds"`
	Items struct {
		ResourceLocation string `json:"resourceLocation"`
	} `json:"items"`
	Trackings struct {
		ResourceLocation string `json:"resourceLocation"`
	} `json:"trackings"`
	Labels struct {
		ResourceLocation string `json:"resourceLocation"`
	} `json:"labels"`
	OriginalOrder struct {
		ResourceLocation string `json:"resourceLocation"`
	} `json:"originalOrder"`
	Events struct {
		Resource struct {
			CancelledDate        time.Time `json:"cancelledDate"`
			CompletedDate        time.Time `json:"completedDate"`
			CreatedDate          time.Time `json:"createdDate"`
			DeliveredDate        time.Time `json:"deliveredDate"`
			ExpectedDate         time.Time `json:"expectedDate"`
			LastManualUpdateDate time.Time `json:"lastManualUpdateDate"`
			PickedUpDate         time.Time `json:"pickedUpDate"`
			ProcessedDate        time.Time `json:"processedDate"`
			ReturnedDate         time.Time `json:"returnedDate"`
			SubmittedDate        time.Time `json:"submittedDate"`
		} `json:"resource"`
		ResourceLocation interface{} `json:"resourceLocation"`
	} `json:"events"`
	Routing struct {
		Resource struct {
			OriginLatitude      string      `json:"originLatitude"`
			OriginLongitude     string      `json:"originLongitude"`
			WarehouseExternalID interface{} `json:"warehouseExternalId"`
			WarehouseID         int         `json:"warehouseId"`
			WarehouseName       string      `json:"warehouseName"`
		} `json:"resource"`
		ResourceLocation interface{} `json:"resourceLocation"`
	} `json:"routing"`
	Options struct {
		ResourceLocation interface{} `json:"resourceLocation"`
	} `json:"options"`
}

func (r *Return) Decode(data json.RawMessage) error {
	return json.Unmarshal(data, r)
}
