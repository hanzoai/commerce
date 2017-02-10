package shipwire

import (
	"hanzo.io/models/order"
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

type ReturnResponse struct {
	Status           int    `json:"status"`
	Message          string `json:"message"`
	ResourceLocation string `json:"resourceLocation"`
	Resource         struct {
		ID              int         `json:"id"`
		ExternalID      interface{} `json:"externalId"`
		TransactionID   string      `json:"transactionId"`
		ExpectedDate    string      `json:"expectedDate"`
		CommerceName    string      `json:"commerceName"`
		LastUpdatedDate string      `json:"lastUpdatedDate"`
		Status          string      `json:"status"`
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
				CancelledDate        interface{} `json:"cancelledDate"`
				CompletedDate        interface{} `json:"completedDate"`
				CreatedDate          string      `json:"createdDate"`
				DeliveredDate        interface{} `json:"deliveredDate"`
				ExpectedDate         string      `json:"expectedDate"`
				LastManualUpdateDate interface{} `json:"lastManualUpdateDate"`
				PickedUpDate         interface{} `json:"pickedUpDate"`
				ProcessedDate        string      `json:"processedDate"`
				ReturnedDate         interface{} `json:"returnedDate"`
				SubmittedDate        interface{} `json:"submittedDate"`
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
	} `json:"resource"`
}

func (c *Client) CreateReturn(ord *order.Order) {
	req := ReturnRequest{}
	req.ExternalID = ord.Id()

	// req.OriginalOrder.ID =
	// req.Options.EmailCustomer =
	// req.Options.GeneratePrepaidLabel =

	for i, item := range ord.Items {
		req.Items[i] = Item{
			SKU:      item.SKU(),
			Quantity: item.Quantity,
		}
	}

	c.Request("POST", "/returns", req)
}
