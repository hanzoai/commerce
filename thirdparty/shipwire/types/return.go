package types

type ReturnOptions struct {
	EmailCustomer        bool   `json:"email"`
	GeneratePrepaidLabel bool   `json:"prepaid"`
	Summary              string `json:"summary"`
	WarehouseRegion      string `json:"warehouseRegion"`
}

type ReturnRequest struct {
	ExternalID    string `json:"externalId"`
	OriginalOrder struct {
		ID int `json:"id"`
	} `json:"originalOrder"`
	Items   []Item `json:"items"`
	Options struct {
		GeneratePrepaidLabel int    `json:"generatePrepaidLabel"`
		EmailCustomer        int    `json:"emailCustomer"`
		WarehouseID          int    `json:"warehouseId,omitempty"`
		WarehouseExternalID  string `json:"warehouseExternalId,omitempty"`
		WarehouseRegion      string `json:"warehouseRegion,omitempty"`
	} `json:"options"`
}

type Return struct {
	ID              int    `json:"id"`
	ExternalID      string `json:"externalId"`
	TransactionID   string `json:"transactionId"`
	ExpectedDate    Date   `json:"expectedDate"`
	CommerceName    string `json:"commerceName"`
	LastUpdatedDate Date   `json:"lastUpdatedDate"`
	Status          string `json:"status"`
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
			CancelledDate        Date `json:"cancelledDate"`
			CompletedDate        Date `json:"completedDate"`
			CreatedDate          Date `json:"createdDate"`
			DeliveredDate        Date `json:"deliveredDate"`
			ExpectedDate         Date `json:"expectedDate"`
			LastManualUpdateDate Date `json:"lastManualUpdateDate"`
			PickedUpDate         Date `json:"pickedUpDate"`
			ProcessedDate        Date `json:"processedDate"`
			ReturnedDate         Date `json:"returnedDate"`
			SubmittedDate        Date `json:"submittedDate"`
		} `json:"resource"`
		ResourceLocation interface{} `json:"resourceLocation"`
	} `json:"events"`
	Routing struct {
		Resource struct {
			OriginLatitude      string `json:"originLatitude"`
			OriginLongitude     string `json:"originLongitude"`
			WarehouseExternalID string `json:"warehouseExternalId"`
			WarehouseID         int    `json:"warehouseId"`
			WarehouseName       string `json:"warehouseName"`
		} `json:"resource"`
		ResourceLocation interface{} `json:"resourceLocation"`
	} `json:"routing"`
	Options struct {
		ResourceLocation interface{} `json:"resourceLocation"`
	} `json:"options"`
}
