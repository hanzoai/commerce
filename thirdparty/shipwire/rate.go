package shipwire

import (
	"hanzo.io/models/order"
	. "hanzo.io/thirdparty/shipwire/types"
)

func (c *Client) Rate(ord *order.Order) (*Rates, *Response, error) {
	req := RateRequest{}

	req.Options.Currency = ord.Currency.Code()
	req.Options.CanSplit = 1
	req.Options.WarehouseArea = "US"
	// req.Options.ChannelName = "My Channel"
	// req.Options.ExpectedShipDate = Date{time.Now().Add(time.Hour * 24)}
	req.Options.HighAccuracyEstimates = 1
	req.Options.ReturnAllRates = 1

	req.Order.ShipTo.Address1 = ord.ShippingAddress.Line1
	req.Order.ShipTo.Address2 = ord.ShippingAddress.Line2
	req.Order.ShipTo.City = ord.ShippingAddress.City
	req.Order.ShipTo.State = ord.ShippingAddress.State
	req.Order.ShipTo.Country = ord.ShippingAddress.Country
	req.Order.ShipTo.PostalCode = ord.ShippingAddress.PostalCode
	req.Order.Items = make([]Item, len(ord.Items))

	for i, item := range ord.Items {
		req.Order.Items[i] = Item{
			SKU:      item.SKU(),
			Quantity: item.Quantity,
		}
	}

	rates := Rates{}

	// Use /api/v3.1/rate
	res, err := c.Request("POST", ".1/rate", req, &rates)
	if err != nil {
		return &rates, res, err
	}

	return &rates, res, ord.Update()
}
