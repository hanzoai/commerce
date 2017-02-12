package shipwire

import (
	"net/http"

	"hanzo.io/models/order"
	"hanzo.io/models/user"
	"strconv"
)

type ServiceLevelCode string

const (
	DomesticGround        ServiceLevelCode = "GD"
	Domestic2Day          ServiceLevelCode = "2D"
	Domestic1Day          ServiceLevelCode = "1D"
	InternationalEconomy  ServiceLevelCode = "E-INTL"
	InternationalStandard ServiceLevelCode = "INTL"
	InternationalPlus     ServiceLevelCode = "PL-INTL"
	InternationalPremium  ServiceLevelCode = "PM-INTL"
)

type Item struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

type OrderRequest struct {
	ExternalID   string `json:"externalId"`
	OrderNo      string `json:"orderNo"`
	CommerceName string `json:"commerceName"`

	Options struct {
		ServiceLevelCode ServiceLevelCode `json:"serviceLevelCode"`
	} `json:"options"`

	ShipTo struct {
		Name       string `json:"name"`
		Email      string `json:"email"`
		Address1   string `json:"address1"`
		Address2   string `json:"address2"`
		City       string `json:"city"`
		State      string `json:"state"`
		PostalCode string `json:"postalCode"`
		Country    string `json:"country"`
	} `json:"shipTo"`

	Items []Item `json:"items"`
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

func (c *Client) CreateOrder(ord *order.Order, usr *user.User, serviceLevelCode ServiceLevelCode) (*http.Response, error) {
	req := OrderRequest{}
	req.CommerceName = "Hanzo"
	req.OrderNo = strconv.Itoa(ord.Number)
	req.ExternalID = ord.Id()
	req.Options.ServiceLevelCode = serviceLevelCode
	req.ShipTo.Name = ord.ShippingAddress.Name
	req.ShipTo.Email = usr.Email
	req.ShipTo.Address1 = ord.ShippingAddress.Line1
	req.ShipTo.Address2 = ord.ShippingAddress.Line2
	req.ShipTo.City = ord.ShippingAddress.City
	req.ShipTo.State = ord.ShippingAddress.State
	req.ShipTo.Country = ord.ShippingAddress.Country
	req.ShipTo.PostalCode = ord.ShippingAddress.PostalCode
	req.Items = make([]Item, len(ord.Items))

	for i, item := range ord.Items {
		req.Items[i] = Item{
			SKU:      item.SKU(),
			Quantity: item.Quantity,
		}
	}

	return c.Request("POST", "/orders", req)
}

func (c *Client) GetOrder(ord *order.Order) {
}
