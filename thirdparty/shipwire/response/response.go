package response

type Ref struct {
	ResourceLocation string   `json:"resourceLocation"`
	Resource         Resource `json:"resource"`
}

type Resource struct {
	// Order
	Warehouse
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

// // Order Body
// type Order struct {
// 	Id int `json:"id"`
// 	ExternalId string `json:"externalId"`
// 	TransactionId string `json:"transactionId"`
//     OrderNo string `json:"orderNo"`
// 	ProcessAfterDate time.Time `json:"processAfterDate"`
// 	NeedsReview int `json:"needsReview"`
// 	VendorId int `json:"vendorId"`
// 	VendorName string `json:"vendorName"`
// 	CommerceName string `json:"commerceName"`
// 	Status string `json:"status"`
// 	LastUpdatedDate time.Time `json:"lastUpdatedDate"`
// 	Holds Ref `json:"holds"`
// 	Items Ref `json:"items"`
// 	Trackings Ref `json:"trackings"`
// 	Returns Ref `json:"returns"`
// 	SplitOrders Ref `json:"splitOrders"`
//     Options Ref `json:"options"`
// 	Price Ref `json:"pricing"`
//         "pricing": {
//             "resourceLocation": null,
//             "resource": {
//                 "shipping": 5.25,
//                 "packaging": 0.00,
//                 "insurance": 0.00,
//                 "handling": 2.75,
//                 "total": 8.00
//             }
//         },
//         # Shipping source information
//         "shipFrom": {
//             "resourceLocation": null,
//             "resource": {
//                 "company": "We Sell'em Co."
//             }
//         },
//         # Receipient shipping address information
//         "shipTo": {
//             "resourceLocation": null,
//             "resource": {
//                 # Recipient details
//                 "email": "audrey.horne@greatnothern.com",
//                 "name": "Audrey Horne",
//                 "company": "Audrey's Bikes",
//                 "address1": "6501 Railroad Avenue SE",
//                 "address2": "Room 315",
//                 "address3": "",
//                 "city": "Snoqualmie",
//                 "state": "WA",
//                 "postalCode": "98065",
//                 "country": "US",
//                 "phone": "4258882556",
//                 "isCommercial": 0,
//                 "isPoBox": 0
//             }
//         },
//         # Invoiced amounts
//         "commercialInvoice": {
//             "resourceLocation": null,
//             "resource": {
//                 "shippingValue": 4.85,
//                 "insuranceValue": 6.57,
//                 "additionalValue": 8.29,
//                 "documentLocation": "https://api.shipwire.com/api/v3/orders/12345/commercialInvoice"
//             }
//         },
//         # Messages to include in packages
//         "packingList": {
//             "resourceLocation": null,
//             "resource": {
//                 # First message
//                 "message1": {
//                     "resourceLocation": null,
//                     "resource": {
//                         "document": "packing-list",
//                         "header": "Enjoy this product!",
//                         "location": "lhs",
//                         "body": "This must be where pies go when they die. Enjoy!"
//                     }
//                 },
//                 "message2": {
//                     "resourceLocation": null,
//                     "resource": {
//                         "document": null,
//                         "header": null,
//                         "location": null,
//                         "body": null
//                     }
//                 },
//                 "message3": {
//                     "resourceLocation": null,
//                     "resource": {
//                         "document": null,
//                         "header": null,
//                         "location": null,
//                         "body": null
//                     }
//                 },
//                 "other": {
//                     "resourceLocation": null,
//                     "resource": {
//                         "document": null,
//                         "header": null,
//                         "location": null,
//                         "body": null
//                     }
//                 },
//                 "documentLocation": "https://api.shipwire.com/api/v3/orders/12345/packingList"
//             }
//         },
//         "shippingLabel": {
//             "resourceLocation": null,
//             "resource": {
//                 "orderId": 12345,
//                 "externalId": null,
//                 "warehouseId": 11,
//                 "warehouseExternalId": null,
//                 "documentLocation": "https://api.shipwire.com/api/v3/orders/12345/shippingLabel"
//             }
//         },
//         # Package routing details
//         "routing": {
//             "resourceLocation": null,
//             "resource": {
//                 "warehouseId": 11,
//                 "warehouseExternalId": null,
//                 "warehouseName": "LA",
//                 "originLongitude": 34.0416,
//                 "originLatitude": -117.369,
//                 "destinationLongitude": -122.1738,
//                 "destinationLatitude": 37.4337
//             }
//         },
//         # Order proccessing events
//         "events": {
//             "resourceLocation": null,
//             "resource": {
//                 "createdDate": "2014-06-10T13:48:44-07:00",
//                 "pickedUpDate": null,
//                 "submittedDate": null,
//                 "processedDate": null,
//                 "completedDate": null,
//                 "expectedDate": "2014-06-12T00:00:00-07:00",
//                 "cancelledDate": null,
//                 "returnedDate": null,
//                 "lastManualUpdateDate": null
//             }
//         },
//         "pricingEstimate": {
//             "resourceLocation": null,
//             "resource": {
//                 "total": 0,
//                 "insurance": 0,
//                 "shipping": 0,
//                 "packaging": 0,
//                 "handling": 0
//             }
//         },
//         "shipwireAnywhere": {
//             "resourceLocation": null,
//             "resource": {
//                 "status": null
//             }
//         },
//         "splitOrders": {
//             {
//                 "resourceLocation": "https://api.shipwire.com/api/v3/orders/12345/splitOrders?offset=0&limit=20",
//                 "resource": {
//                     "offset": 0,
//                     "total": 0,
//                     "previous": null,
//                     "next": null,
//                     "items": []
//                 }
//             }
//         },
//         "freightSummary": {
//             "resourceLocation": null,
//             "resource": {
//                 "totalWeight": "2.30",
//                 "weightUnit": "LB",
//                 "measurementType": "total"
//             }
//         }
// }
