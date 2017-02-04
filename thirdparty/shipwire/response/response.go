package response

import "time"

type WarehouseRef struct {
	ResourceLocation string    `json:"resourceLocation"`
	Resource         Warehouse `json:"resource"`
}

type Warehouse struct {
	WarehouseId         int    `json:"warehouseId"`
	WarehouseExternalId string `json:"warehouseExternalId"`
	WarehouseRegion     string `json:"warehouseRegion"`
	WarehouseArea       string `json:"warehouseArea"`
	ServiceLevelCode    string `json:"serviceLevelCode"`
	CarrierCode         string `json:"carrierCode"`
	SameDay             string `json:"sameDay"`
	ChannelName         string `json:"channelName"`
	ForceDuplicate      string `json:"forceDuplicate"`
	ForceAddress        string `json:"forceAddress"`
	Referrer            string `json:"referrer"`
}

type PricingRef struct {
	ResourceLocation string  `json:"resourceLocation"`
	Resource         Pricing `json:"resource"`
}

type Pricing struct {
	Shipping  float64 `json:"shipping"`
	Packaging float64 `json:"packaging"`
	Insurance float64 `json:"insurance"`
	Handling  float64 `json:"handling"`
	Total     float64 `json:"total"`
}

type ShipFromRef struct {
	ResourceLocation string   `json:"resourceLocation"`
	Resource         ShipFrom `json:"resource"`
}

type ShipFrom struct {
	Company string `json:"company"`
}

type ShipToRef struct {
	ResourceLocation string `json:"resourceLocation"`
	Resource         ShipTo `json:"resource"`
}

type ShipTo struct {
	Email        string `json:"email"`
	Name         string `json:"name"`
	Company      string `json:"company"`
	Address1     string `json:"address1"`
	Address2     string `json:"address2"`
	Address3     string `json:"address3"`
	City         string `json:"city"`
	State        string `json:"state"`
	PostalCode   string `json:"postalCode"`
	Country      string `json:"country"`
	Phone        string `json:"phone"`
	IsCommercial int    `json:"isCommercial"`
	IsPoBox      int    `json:"isPoBox"`
}

type CommercialInvoiceRef struct {
	ResourceLocation string            `json:"resourceLocation"`
	Resource         CommercialInvoice `json:"resource"`
}

type CommercialInvoice struct {
	ShippingValue         float64 `json:"shippingValue"`
	InsuranceValue        float64 `json:"insuranceValue"`
	AdditionalValue       float64 `json:"additionalValue"`
	DocumentationLocation string  `json:"documentationLocation"`
}

type TrackingRef struct {
	ResourceLocation string   `json:"resourceLocation"`
	Resource         Tracking `json:"resource"`
}

type Tracking struct {
	Id                  int       `json:"id"`
	OrderId             int       `json:"orderId"`
	OrderExternalId     string    `json:"orderExternalId"`
	Tracking            string    `json:"tracking"`
	Carrier             string    `json:"carrier"`
	Url                 string    `json:"url"`
	Summary             string    `json:"summary"`
	SummaryDate         time.Time `json:"summaryDate"`
	LabelCreatedDate    time.Time `json:"labelCreatedDate"`
	TrackedDate         time.Time `json:"trackedDate"`
	FirstScanDate       time.Time `json:"firstScanDate"`
	FirstScanRegion     string    `json:"firstScanRegion"`
	FirstScanPostalCode string    `json:"firstScanPostalCode"`
	FirstScanCountry    string    `json:"firstScanCountry"`
	DeliveredDate       time.Time `json:"deliveredDate"`
	DeliveryCity        string    `json:"deliveryCity"`
	DeliveryRegion      string    `json:"deliveryRegion"`
	DeliveryPostalCode  string    `json:"DeliveryPostalCode"`
	DeliveryCountry     string    `json:"DeliveryCountry"`
}

type OrderRef struct {
	ResourceLocation string `json:"resourceLocation"`
	Resource         Order  `json:"resource"`
}

// Order Body
type Order struct {
	Id               int       `json:"id"`
	ExternalId       string    `json:"externalId"`
	TransactionId    string    `json:"transactionId"`
	OrderNo          string    `json:"orderNo"`
	ProcessAfterDate time.Time `json:"processAfterDate"`
	NeedsReview      int       `json:"needsReview"`
	VendorId         int       `json:"vendorId"`
	VendorName       string    `json:"vendorName"`
	CommerceName     string    `json:"commerceName"`
	Status           string    `json:"status"`
	LastUpdatedDate  time.Time `json:"lastUpdatedDate"`
	// Holds HoldsRef `json:"holds"`
	// Items ItemsRef `json:"items"`
	// Trackings TrackingsRef `json:"trackings"`
	// Returns ReturnsRef `json:"returns"`
	// SplitOrders SplitOrdersRef `json:"splitOrders"`
	// Options OptionsRef `json:"options"`
	// Pricing PricingRef `json:"pricing"`
	// ShipFrom ShipFromRef `json:"shipFrom"`
	// ShipTo ShipToRef `json:"shipTo"`
	// "packingList": {
	// "resourceLocation": null,
	// "resource": {
	// # First message
	// "message1": {
	// "resourceLocation": null,
	// "resource": {
	// "document": "packing-list",
	// "header": "Enjoy this product!",
	// "location": "lhs",
	// "body": "This must be where pies go when they die. Enjoy!"
	// }
	// },
	// "message2": {
	// "resourceLocation": null,
	// "resource": {
	// "document": null,
	// "header": null,
	// "location": null,
	// "body": null
	// }
	// },
	// "message3": {
	// "resourceLocation": null,
	// "resource": {
	// "document": null,
	// "header": null,
	// "location": null,
	// "body": null
	// }
	// },
	// "other": {
	// "resourceLocation": null,
	// "resource": {
	// "document": null,
	// "header": null,
	// "location": null,
	// "body": null
	// }
	// },
	// "documentLocation": "https://api.shipwire.com/api/v3/orders/12345/packingList"
	// }
	// },
	// "shippingLabel": {
	// "resourceLocation": null,
	// "resource": {
	// "orderId": 12345,
	// "externalId": null,
	// "warehouseId": 11,
	// "warehouseExternalId": null,
	// "documentLocation": "https://api.shipwire.com/api/v3/orders/12345/shippingLabel"
	// }
	// },
	// # Package routing details
	// "routing": {
	// "resourceLocation": null,
	// "resource": {
	// "warehouseId": 11,
	// "warehouseExternalId": null,
	// "warehouseName": "LA",
	// "originLongitude": 34.0416,
	// "originLatitude": -117.369,
	// "destinationLongitude": -122.1738,
	// "destinationLatitude": 37.4337
	// }
	// },
	// # Order proccessing events
	// "events": {
	// "resourceLocation": null,
	// "resource": {
	// "createdDate": "2014-06-10T13:48:44-07:00",
	// "pickedUpDate": null,
	// "submittedDate": null,
	// "processedDate": null,
	// "completedDate": null,
	// "expectedDate": "2014-06-12T00:00:00-07:00",
	// "cancelledDate": null,
	// "returnedDate": null,
	// "lastManualUpdateDate": null
	// }
	// },
	// "pricingEstimate": {
	// "resourceLocation": null,
	// "resource": {
	// "total": 0,
	// "insurance": 0,
	// "shipping": 0,
	// "packaging": 0,
	// "handling": 0
	// }
	// },
	// "shipwireAnywhere": {
	// "resourceLocation": null,
	// "resource": {
	// "status": null
	// }
	// },
	// "splitOrders": {
	// {
	// "resourceLocation": "https://api.shipwire.com/api/v3/orders/12345/splitOrders?offset=0&limit=20",
	// "resource": {
	// "offset": 0,
	// "total": 0,
	// "previous": null,
	// "next": null,
	// "items": []
	// }
	// }
	// },
	// "freightSummary": {
	// "resourceLocation": null,
	// "resource": {
	// "totalWeight": "2.30",
	// "weightUnit": "LB",
	// "measurementType": "total"
	// }
	// }
}
