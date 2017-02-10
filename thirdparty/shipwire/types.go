package shipwire

type OrderRequest struct {
	Items []struct {
		Sku      string `json:"sku"`
		Quantity int    `json:"quantity"`
	} `json:"items"`
	OrderNo string `json:"orderNo"`
	ShipTo  struct {
		City       string `json:"city"`
		State      string `json:"state"`
		Name       string `json:"name"`
		Country    string `json:"country"`
		PostalCode string `json:"postalCode"`
		Address1   string `json:"address1"`
		Address2   string `json:"address2"`
		Email      string `json:"email"`
	} `json:"shipTo"`
	ExternalID string `json:"externalId"`
	Options    struct {
		ServiceLevelCode string `json:"serviceLevelCode"`
	} `json:"options"`
}

type OrderResponse struct {
	Status           int    `json:"status"`
	ResourceLocation string `json:"resourceLocation"`
	Message          string `json:"message"`

	Resource struct {
		Previous interface{} `json:"previous"`
		Next     interface{} `json:"next"`
		Total    int         `json:"total"`
		Items    []struct {
			ResourceLocation string `json:"resourceLocation"`
			Resource         struct {
				VendorExternalID  interface{} `json:"vendorExternalId"`
				VendorID          interface{} `json:"vendorId"`
				CommercialInvoice struct {
					ResourceLocation string `json:"resourceLocation"`
				} `json:"commercialInvoice"`
				ShipwireAnywhere struct {
					ResourceLocation interface{} `json:"resourceLocation"`
					Resource         struct {
						Status interface{} `json:"status"`
					} `json:"resource"`
				} `json:"shipwireAnywhere"`
				Pieces struct {
					ResourceLocation string `json:"resourceLocation"`
				} `json:"pieces"`
				PurchaseOrderExternalID interface{} `json:"purchaseOrderExternalId"`
				ShippingLabel           struct {
					ResourceLocation string `json:"resourceLocation"`
				} `json:"shippingLabel"`
				OrderNo     string `json:"orderNo"`
				ID          int    `json:"id"`
				NeedsReview int    `json:"needsReview"`
				Holds       struct {
					ResourceLocation string `json:"resourceLocation"`
				} `json:"holds"`
				CommerceName     string `json:"commerceName"`
				ProcessAfterDate string `json:"processAfterDate"`
				Returns          struct {
					ResourceLocation string `json:"resourceLocation"`
				} `json:"returns"`
				ExternalID string `json:"externalId"`
				Routing    struct {
					ResourceLocation interface{} `json:"resourceLocation"`
					Resource         struct {
						WarehouseID          interface{} `json:"warehouseId"`
						OriginLongitude      interface{} `json:"originLongitude"`
						DestinationLatitude  interface{} `json:"destinationLatitude"`
						WarehouseName        interface{} `json:"warehouseName"`
						WarehouseExternalID  interface{} `json:"warehouseExternalId"`
						PhysicalWarehouseID  interface{} `json:"physicalWarehouseId"`
						DestinationLongitude interface{} `json:"destinationLongitude"`
						OriginLatitude       interface{} `json:"originLatitude"`
					} `json:"resource"`
				} `json:"routing"`
				PurchaseOrderID interface{} `json:"purchaseOrderId"`
				VendorName      interface{} `json:"vendorName"`
				Status          string      `json:"status"`
				ShipTo          struct {
					ResourceLocation interface{} `json:"resourceLocation"`
					Resource         struct {
						City         string      `json:"city"`
						Name         string      `json:"name"`
						IsPoBox      int         `json:"isPoBox"`
						Address1     string      `json:"address1"`
						Company      interface{} `json:"company"`
						Address3     interface{} `json:"address3"`
						IsCommercial int         `json:"isCommercial"`
						Email        string      `json:"email"`
						Phone        string      `json:"phone"`
						State        string      `json:"state"`
						Country      string      `json:"country"`
						PostalCode   string      `json:"postalCode"`
						Address2     string      `json:"address2"`
					} `json:"resource"`
				} `json:"shipTo"`
				PricingEstimate struct {
					ResourceLocation interface{} `json:"resourceLocation"`
					Resource         struct {
						Packaging int `json:"packaging"`
						Total     int `json:"total"`
						Insurance int `json:"insurance"`
						Shipping  int `json:"shipping"`
						Handling  int `json:"handling"`
					} `json:"resource"`
				} `json:"pricingEstimate"`
				FreightSummary struct {
					ResourceLocation interface{} `json:"resourceLocation"`
					Resource         struct {
						WeightUnit      interface{} `json:"weightUnit"`
						MeasurementType interface{} `json:"measurementType"`
						TotalWeight     string      `json:"totalWeight"`
					} `json:"resource"`
				} `json:"freightSummary"`
				PackingList struct {
					ResourceLocation string `json:"resourceLocation"`
				} `json:"packingList"`
				Items struct {
					ResourceLocation string `json:"resourceLocation"`
				} `json:"items"`
				SplitOrders struct {
					ResourceLocation string `json:"resourceLocation"`
				} `json:"splitOrders"`
				Events struct {
					ResourceLocation interface{} `json:"resourceLocation"`
					Resource         struct {
						ExpectedDate          interface{} `json:"expectedDate"`
						CancelledDate         interface{} `json:"cancelledDate"`
						ExpectedSubmittedDate string      `json:"expectedSubmittedDate"`
						CreatedDate           string      `json:"createdDate"`
						ReturnedDate          interface{} `json:"returnedDate"`
						SubmittedDate         interface{} `json:"submittedDate"`
						ExpectedCompletedDate string      `json:"expectedCompletedDate"`
						LastManualUpdateDate  interface{} `json:"lastManualUpdateDate"`
						PickedUpDate          interface{} `json:"pickedUpDate"`
						CompletedDate         interface{} `json:"completedDate"`
						ProcessedDate         interface{} `json:"processedDate"`
					} `json:"resource"`
				} `json:"events"`
				ShipFrom struct {
					ResourceLocation interface{} `json:"resourceLocation"`
					Resource         struct {
						Company string `json:"company"`
					} `json:"resource"`
				} `json:"shipFrom"`
				LastUpdatedDate string      `json:"lastUpdatedDate"`
				PurchaseOrderNo interface{} `json:"purchaseOrderNo"`
				TransactionID   string      `json:"transactionId"`
				Trackings       struct {
					ResourceLocation string `json:"resourceLocation"`
				} `json:"trackings"`
				Options struct {
					ResourceLocation interface{} `json:"resourceLocation"`
					Resource         struct {
						WarehouseID                    interface{} `json:"warehouseId"`
						BillingType                    interface{} `json:"billingType"`
						WarehouseRegion                interface{} `json:"warehouseRegion"`
						Referrer                       string      `json:"referrer"`
						ForceAddress                   int         `json:"forceAddress"`
						WarehouseExternalID            interface{} `json:"warehouseExternalId"`
						ForceDuplicate                 int         `json:"forceDuplicate"`
						ServiceLevelCode               string      `json:"serviceLevelCode"`
						CarrierAccountNumber           interface{} `json:"carrierAccountNumber"`
						SameDay                        string      `json:"sameDay"`
						ThirdPartyCarrierCodeRequested interface{} `json:"thirdPartyCarrierCodeRequested"`
						WarehouseArea                  interface{} `json:"warehouseArea"`
						CarrierCode                    string      `json:"carrierCode"`
						ChannelName                    interface{} `json:"channelName"`
						TestOrder                      int         `json:"testOrder"`
					} `json:"resource"`
				} `json:"options"`
				Pricing struct {
					ResourceLocation interface{} `json:"resourceLocation"`
					Resource         struct {
						Packaging int `json:"packaging"`
						Total     int `json:"total"`
						Handling  int `json:"handling"`
						Insurance int `json:"insurance"`
						Shipping  int `json:"shipping"`
					} `json:"resource"`
				} `json:"pricing"`
			} `json:"resource"`
		} `json:"items"`
		Offset int `json:"offset"`
	} `json:"resource"`
}

type ReturnRequest struct {
	ExternalID    string `json:"externalId"`
	OriginalOrder struct {
		ID int `json:"id"`
	} `json:"originalOrder"`
	Items []struct {
		Sku      string `json:"sku"`
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
